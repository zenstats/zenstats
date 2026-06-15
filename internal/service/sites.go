package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/shieldrulescountry"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/shieldruleshostname"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/shieldrulesip"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/site"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/sitemembership"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/user"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/utils"
)

var (
	siteServiceInstance *SiteService
	siteOnce            sync.Once
)

// 新增：全站列表缓存项结构体
type allSitesCacheItem struct {
	sites []*ent.Site
}

// SiteService 站点服务，提供站点的 CRUD 操作及屏蔽规则管理，内置多级缓存。
type SiteService struct {
	db               *postgresql.Client
	cache            sync.Map
	allSitesCacheKey string

	domainCache             *expirable.LRU[string, *ent.Site]
	shieldRuleIPCache       *expirable.LRU[string, []*ent.ShieldRulesIp]
	shieldRuleHostnameCache *expirable.LRU[string, []*ent.ShieldRulesHostname]
	shieldRuleCountryCache  *expirable.LRU[string, []*ent.ShieldRulesCountry]
	membershipCache         *expirable.LRU[string, *ent.Site]
}

// GetSiteService 获取 SiteService 单例实例。
func GetSiteService() *SiteService {
	siteOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		l := expirable.NewLRU[string, *ent.Site](1000, nil, 30*time.Minute)
		ipCache := expirable.NewLRU[string, []*ent.ShieldRulesIp](1000, nil, 30*time.Minute)
		hostnameCache := expirable.NewLRU[string, []*ent.ShieldRulesHostname](1000, nil, 30*time.Minute)
		countryCache := expirable.NewLRU[string, []*ent.ShieldRulesCountry](1000, nil, 30*time.Minute)
		membershipCache := expirable.NewLRU[string, *ent.Site](1000, nil, 30*time.Minute)
		siteServiceInstance = &SiteService{
			db:                      db,
			domainCache:             l,
			shieldRuleIPCache:       ipCache,
			shieldRuleHostnameCache: hostnameCache,
			shieldRuleCountryCache:  countryCache,
			membershipCache:         membershipCache,
			cache:                   sync.Map{},
			allSitesCacheKey:        "all_sites",
		}
	})
	return siteServiceInstance
}

// CreateSiteParams 创建站点时的参数。
type CreateSiteParams struct {
	Domain       string
	Timezone     string
	Remark       string
	IngestConfig IngestConfig
}

// IngestConfig 事件采集限流配置。
type IngestConfig struct {
	RateLimitScaleSeconds int
	LimitPerMinute        int
}

// generateVerificationToken 生成随机的 32 字符十六进制验证令牌。
func generateVerificationToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

// CreateSite 创建新站点并为当前用户分配 Owner 角色，使用事务保证原子性。
// 多个用户可以创建相同域名的站点，但只能有一个被验证。
func (s *SiteService) CreateSite(ctx *gin.Context, params CreateSiteParams) (*ent.Site, error) {
	tx, err := s.db.Client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting a transaction: %w", err)
	}

	token, err := generateVerificationToken()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("generating verification token: %w", err)
	}

	site, err := s.db.Client.Site.Create().
		SetDomain(params.Domain).
		SetTimezone(params.Timezone).
		SetRemark(params.Remark).
		SetIngestRateLimitScaleSeconds(params.IngestConfig.RateLimitScaleSeconds).
		SetIngestLimitPerMinute(params.IngestConfig.LimitPerMinute).
		SetVerificationToken(token).
		SetIsVerified(false).
		Save(ctx)

	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("creating site: %w", err)
	}

	// 授权sites 给当前用户
	_, err = s.db.Client.SiteMembership.Create().
		SetSite(site).
		SetUserID(ctx.GetInt64("user_id")).
		SetRole(sitemembership.RoleOwner).
		Save(ctx)

	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("creating site membership: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	// 触发缓存更新，删除对应缓存
	s.cache.Delete(ctx.GetInt64("user_id"))
	// 新增：删除全站列表缓存
	s.cache.Delete(s.allSitesCacheKey)

	return site, nil
}

// GetSiteByDomain 根据域名查询站点，优先从 LRU 缓存中获取。
// 当存在多个同名域名站点时，优先返回已验证的站点。
// 若缓存命中但该站点未验证，会回源确认是否已有其他用户验证了该域名。
func (s *SiteService) GetSiteByDomain(ctx context.Context, domain string) (*ent.Site, error) {
	if cached, ok := s.domainCache.Get(domain); ok {
		// 缓存命中：若已验证则直接返回，若未验证则回源检查是否有已验证站点
		if cached.IsVerified {
			return cached, nil
		}
		verified, err := s.db.Client.Site.Query().
			Where(site.Domain(domain), site.IsVerified(true)).
			First(ctx)
		if err == nil {
			s.domainCache.Add(domain, verified)
			return verified, nil
		}
		// 没有已验证站点，返回缓存的未验证站点
		return cached, nil
	}

	sites, err := s.db.Client.Site.Query().Where(site.Domain(domain)).All(ctx)
	if err != nil {
		return nil, err
	}
	if len(sites) == 0 {
		return nil, fmt.Errorf("site not found")
	}

	// 优先返回已验证的站点
	var result *ent.Site
	for _, s := range sites {
		if s.IsVerified {
			result = s
			break
		}
	}
	if result == nil {
		result = sites[0]
	}

	s.domainCache.Add(domain, result)
	return result, nil
}

// SiteWithRemark 包含角色信息的站点响应结构（service 层）。
type SiteWithRemark struct {
	ID                          int64      `json:"id"`
	Domain                      string     `json:"domain"`
	Timezone                    string     `json:"timezone"`
	Remark                      string     `json:"remark"`
	IngestRateLimitScaleSeconds int        `json:"rate_seconds"`
	IngetLimitPerMinute         int        `json:"limit_minute"`
	AllowedOrigins              string     `json:"allowed_origins"`
	Role                        string     `json:"role"`
	IsVerified                  bool       `json:"is_verified"`
	VerifiedAt                  *time.Time `json:"verified_at,omitempty"`
}

// GetUserSiteList 获取当前用户的站点列表。
func (s *SiteService) GetUserSiteList(ctx *gin.Context) ([]*SiteWithRemark, error) {
	userID := ctx.GetInt64("user_id")

	siteMemberships, err := s.db.Client.SiteMembership.Query().
		Where(sitemembership.UserID(userID)).
		WithSite().
		All(ctx)
	if err != nil {
		return nil, err
	}
	sites := make([]*SiteWithRemark, len(siteMemberships))
	for i, sm := range siteMemberships {
		sites[i] = &SiteWithRemark{
			ID:                          sm.Edges.Site.ID,
			Domain:                      sm.Edges.Site.Domain,
			Timezone:                    sm.Edges.Site.Timezone,
			Remark:                      sm.Edges.Site.Remark,
			IngestRateLimitScaleSeconds: sm.Edges.Site.IngestRateLimitScaleSeconds,
			IngetLimitPerMinute:         sm.Edges.Site.IngestLimitPerMinute,
			AllowedOrigins:              sm.Edges.Site.AllowedOrigins,
			Role:                        sm.Role.String(),
			IsVerified:                  sm.Edges.Site.IsVerified,
			VerifiedAt:                  sm.Edges.Site.VerifiedAt,
		}
	}

	return sites, nil
}

// GetUserSiteByDomain 根据域名模糊查询当前用户的站点列表。
func (s *SiteService) GetUserSiteByDomain(ctx *gin.Context, domain string) ([]*SiteWithRemark, error) {
	siteMemberships, err := s.db.Client.SiteMembership.Query().
		Where(sitemembership.UserID(ctx.GetInt64("user_id"))).
		WithSite(func(sq *ent.SiteQuery) {
			if domain != "" {
				sq.Where(site.DomainContains(domain))
			}
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}
	sites := make([]*SiteWithRemark, 0, len(siteMemberships))
	for _, sm := range siteMemberships {
		// 检查 sm.Edges.Site 是否为 nil
		if sm.Edges.Site == nil {
			continue
		}
		site := &SiteWithRemark{
			ID:                          sm.Edges.Site.ID,
			Domain:                      sm.Edges.Site.Domain,
			Timezone:                    sm.Edges.Site.Timezone,
			Remark:                      sm.Edges.Site.Remark,
			IngestRateLimitScaleSeconds: sm.Edges.Site.IngestRateLimitScaleSeconds,
			IngetLimitPerMinute:         sm.Edges.Site.IngestLimitPerMinute,
			AllowedOrigins:              sm.Edges.Site.AllowedOrigins,
			Role:                        sm.Role.String(),
			IsVerified:                  sm.Edges.Site.IsVerified,
			VerifiedAt:                  sm.Edges.Site.VerifiedAt,
		}
		sites = append(sites, site)
	}

	return sites, nil
}

// CheckSiteMembership 检查用户是否为指定域名站点的成员，并返回该用户所属的具体站点。
func (s *SiteService) CheckSiteMembership(ctx context.Context, userID int64, domain string) (*ent.Site, error) {
	cacheKey := fmt.Sprintf("%d:%s", userID, domain)
	if cached, ok := s.membershipCache.Get(cacheKey); ok {
		return cached, nil
	}

	// 查找该域名下所有站点
	sites, err := s.db.Client.Site.Query().Where(site.Domain(domain)).All(ctx)
	if err != nil || len(sites) == 0 {
		return nil, fmt.Errorf("site not found")
	}

	siteIDs := make([]int64, len(sites))
	for i, st := range sites {
		siteIDs[i] = st.ID
	}

	// 查找用户在这些站点中的 membership
	membership, err := s.db.Client.SiteMembership.Query().
		Where(
			sitemembership.SiteIDIn(siteIDs...),
			sitemembership.UserID(userID),
		).
		WithSite().
		First(ctx)
	if err != nil {
		return nil, fmt.Errorf("site membership not found")
	}

	result := membership.Edges.Site
	s.membershipCache.Add(cacheKey, result)
	return result, nil
}

// GetSiteList 获取所有站点列表，使用缓存加速。
func (s *SiteService) GetSiteList(ctx *gin.Context) ([]*ent.Site, error) {
	// 尝试从缓存中获取全站列表数据
	if cached, ok := s.cache.Load(s.allSitesCacheKey); ok {
		item := cached.(allSitesCacheItem)

		return item.sites, nil
	}

	sites, err := s.db.Client.Site.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	s.cache.Store(s.allSitesCacheKey, allSitesCacheItem{
		sites: sites,
	})

	return sites, nil
}

// AdminSiteItem 管理员站点列表项（含站点所有者信息）。
type AdminSiteItem struct {
	ID         int64
	Domain     string
	Remark     string
	Timezone   string
	OwnerName  string
	IsVerified bool
	VerifiedAt *time.Time
	CreatedAt  time.Time
}

// GetAllSitesWithOwner 分页查询所有站点，附带站点所有者用户名。
func (s *SiteService) GetAllSitesWithOwner(ctx context.Context, offset, pageSize int) ([]*AdminSiteItem, error) {
	siteMemberships, err := s.db.Client.SiteMembership.Query().
		Where(sitemembership.RoleEQ("owner")).
		WithSite().
		WithUser(func(uq *ent.UserQuery) {
			uq.Select(user.FieldID, user.FieldName)
		}).
		Order(ent.Desc(sitemembership.FieldID)).
		Offset(offset).
		Limit(pageSize + 1).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// 每个站点可能有多个 owner，取第一个
	seenSites := make(map[int64]bool)
	result := make([]*AdminSiteItem, 0, pageSize)
	for _, sm := range siteMemberships {
		if sm.Edges.Site == nil || seenSites[sm.Edges.Site.ID] {
			continue
		}
		seenSites[sm.Edges.Site.ID] = true

		ownerName := ""
		if sm.Edges.User != nil {
			ownerName = sm.Edges.User.Name
		}

		result = append(result, &AdminSiteItem{
			ID:         sm.Edges.Site.ID,
			Domain:     sm.Edges.Site.Domain,
			Remark:     sm.Edges.Site.Remark,
			Timezone:   sm.Edges.Site.Timezone,
			OwnerName:  ownerName,
			IsVerified: sm.Edges.Site.IsVerified,
			VerifiedAt: sm.Edges.Site.VerifiedAt,
			CreatedAt:  sm.Edges.Site.CreatedAt,
		})
	}

	return result, nil
}

// GetAllSitesCount 获取所有站点总数。
func (s *SiteService) GetAllSitesCount(ctx context.Context) (int, error) {
	return s.db.Client.Site.Query().Count(ctx)
}

// GetSiteByID 根据ID获取站点信息
func (s *SiteService) GetSiteByID(ctx *gin.Context, id int) (*ent.Site, error) {
	cacheKey := fmt.Sprintf("site:%d", id)
	if cached, ok := s.cache.Load(cacheKey); ok {
		return cached.(*ent.Site), nil
	}

	site, err := s.db.Client.Site.Query().Where(site.ID(int64(id))).Only(ctx)
	if err != nil {
		return nil, err
	}

	s.cache.Store(cacheKey, site)
	return site, nil
}

// AdminVerifySite 管理员手动验证站点（跳过域名所有权验证）。
func (s *SiteService) AdminVerifySite(ctx context.Context, siteID int64) error {
	// 直接查询站点
	siteEntity, err := s.db.Client.Site.Query().Where(site.ID(siteID)).Only(ctx)
	if err != nil {
		return fmt.Errorf("site not found: %w", err)
	}

	if siteEntity.IsVerified {
		return errors.New("site already verified")
	}

	// 启动事务，检查是否有其他同域名站点已验证
	tx, err := s.db.Client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting a transaction: %w", err)
	}

	// 锁定当前站点行
	lockSite, err := tx.Site.Query().Where(site.ID(siteID)).Only(ctx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("locking site: %w", err)
	}
	if lockSite.IsVerified {
		tx.Rollback()
		return errors.New("site already verified")
	}

	// 检查是否有其他同域名站点已验证
	domain := siteEntity.Domain
	otherVerified, err := tx.Site.Query().
		Where(
			site.Domain(domain),
			site.IsVerified(true),
			site.IDNEQ(siteID),
		).
		Exist(ctx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("checking other verified sites: %w", err)
	}
	if otherVerified {
		tx.Rollback()
		return errors.New("domain already verified by another user")
	}

	// 管理员手动验证通过，更新站点状态
	now := time.Now()
	_, err = tx.Site.UpdateOneID(siteID).
		SetIsVerified(true).
		SetVerifiedAt(now).
		Save(ctx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("updating site verification status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	// 清除缓存
	s.InvalidateDomainCache(domain)
	s.cache.Delete(s.allSitesCacheKey)

	return nil
}

// AddShieldRuleIP 添加IP屏蔽规则
func (s *SiteService) AddShieldRuleIP(ctx *gin.Context, siteID int64, ip string, action string, description string) (*ent.ShieldRulesIp, error) {
	ipInet, err := utils.ParseInet(ip)
	if err != nil {
		return nil, fmt.Errorf("invalid IP format: %w", err)
	}

	rule, err := s.db.Client.ShieldRulesIp.Create().
		SetSiteID(siteID).
		SetInet(ipInet).
		SetAction(action).
		SetDescription(description).
		SetAddedBy("system").
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create shield rule ip failed: %w", err)
	}

	s.shieldRuleIPCache.Remove(fmt.Sprintf("%d", siteID))
	return rule, nil
}

// ListShieldRuleIP 获取IP屏蔽规则列表
func (s *SiteService) ListShieldRuleIP(ctx context.Context, siteID int64) ([]*ent.ShieldRulesIp, error) {
	cacheKey := fmt.Sprintf("%d", siteID)
	if rules, ok := s.shieldRuleIPCache.Get(cacheKey); ok {
		return rules, nil
	}

	rules, err := s.db.Client.ShieldRulesIp.Query().
		Where(shieldrulesip.SiteID(siteID)).
		Order(ent.Desc(shieldrulesip.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query shield rule ip failed: %w", err)
	}

	s.shieldRuleIPCache.Add(cacheKey, rules)
	return rules, nil
}

// RemoveShieldRuleIP 删除IP屏蔽规则
func (s *SiteService) RemoveShieldRuleIP(ctx context.Context, siteID int64, ruleID int64) error {
	count, err := s.db.Client.ShieldRulesIp.Query().
		Where(
			shieldrulesip.ID(ruleID),
			shieldrulesip.SiteID(siteID),
		).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("query shield rule ip failed: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("shield rule ip not found or not belongs to site")
	}

	if err := s.db.Client.ShieldRulesIp.DeleteOneID(ruleID).Exec(ctx); err != nil {
		return fmt.Errorf("delete shield rule ip failed: %w", err)
	}

	s.shieldRuleIPCache.Remove(fmt.Sprintf("%d", siteID))
	return nil
}

// AddShieldRuleHostname 添加Hostname屏蔽规则
func (s *SiteService) AddShieldRuleHostname(ctx context.Context, siteID int64, hostname string, hostnamePattern string, action string) (*ent.ShieldRulesHostname, error) {
	rule, err := s.db.Client.ShieldRulesHostname.Create().
		SetSiteID(siteID).
		SetHostname(hostname).
		SetHostnamePattern(hostnamePattern).
		SetAction(action).
		SetAddedBy("system").
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create shield rule hostname failed: %w", err)
	}

	s.shieldRuleHostnameCache.Remove(fmt.Sprintf("%d", siteID))
	return rule, nil
}

// ListShieldRuleHostname 获取Hostname屏蔽规则列表
func (s *SiteService) ListShieldRuleHostname(ctx context.Context, siteID int64) ([]*ent.ShieldRulesHostname, error) {
	cacheKey := fmt.Sprintf("%d", siteID)
	if rules, ok := s.shieldRuleHostnameCache.Get(cacheKey); ok {
		return rules, nil
	}

	rules, err := s.db.Client.ShieldRulesHostname.Query().
		Where(shieldruleshostname.SiteID(siteID)).
		Order(ent.Desc(shieldruleshostname.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query shield rule hostname failed: %w", err)
	}

	s.shieldRuleHostnameCache.Add(cacheKey, rules)
	return rules, nil
}

// RemoveShieldRuleHostname 删除Hostname屏蔽规则
func (s *SiteService) RemoveShieldRuleHostname(ctx *gin.Context, siteID int64, ruleID int64) error {
	count, err := s.db.Client.ShieldRulesHostname.Query().
		Where(
			shieldruleshostname.ID(ruleID),
			shieldruleshostname.SiteID(siteID),
		).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("query shield rule hostname failed: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("shield rule hostname not found or not belongs to site")
	}

	if err := s.db.Client.ShieldRulesHostname.DeleteOneID(ruleID).Exec(ctx); err != nil {
		return fmt.Errorf("delete shield rule hostname failed: %w", err)
	}

	s.shieldRuleHostnameCache.Remove(fmt.Sprintf("%d", siteID))
	return nil
}

// AddShieldRuleCountry 添加Country屏蔽规则
func (s *SiteService) AddShieldRuleCountry(ctx *gin.Context, siteID int64, countryCode string, action string) (*ent.ShieldRulesCountry, error) {
	user, _ := GetUserService().GetUserByID(ctx, ctx.GetInt64("user_id"))
	rule, err := s.db.Client.ShieldRulesCountry.Create().
		SetSiteID(siteID).
		SetCountryCode(countryCode).
		SetAction(action).
		SetAddedBy(user.Name).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create shield rule country failed: %w", err)
	}

	s.shieldRuleCountryCache.Remove(fmt.Sprintf("%d", siteID))
	return rule, nil
}

// ListShieldRuleCountry 获取Country屏蔽规则列表
func (s *SiteService) ListShieldRuleCountry(ctx context.Context, siteID int64) ([]*ent.ShieldRulesCountry, error) {
	cacheKey := fmt.Sprintf("%d", siteID)
	if rules, ok := s.shieldRuleCountryCache.Get(cacheKey); ok {
		return rules, nil
	}

	rules, err := s.db.Client.ShieldRulesCountry.Query().
		Where(shieldrulescountry.SiteID(siteID)).
		Order(ent.Desc(shieldrulescountry.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query shield rule country failed: %w", err)
	}

	s.shieldRuleCountryCache.Add(cacheKey, rules)
	return rules, nil
}

// RemoveShieldRuleCountry 删除Country屏蔽规则
func (s *SiteService) RemoveShieldRuleCountry(ctx *gin.Context, siteID int64, ruleID int64) error {
	count, err := s.db.Client.ShieldRulesCountry.Query().
		Where(
			shieldrulescountry.ID(ruleID),
			shieldrulescountry.SiteID(siteID),
		).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("query shield rule country failed: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("shield rule country not found or not belongs to site")
	}

	if err := s.db.Client.ShieldRulesCountry.DeleteOneID(ruleID).Exec(ctx); err != nil {
		return fmt.Errorf("delete shield rule country failed: %w", err)
	}

	s.shieldRuleCountryCache.Remove(fmt.Sprintf("%d", siteID))
	return nil
}

// UpdateSite 更新站点信息
func (s *SiteService) UpdateSite(ctx *gin.Context, siteID int64, req types.UpdateSiteRequest) (*ent.Site, error) {
	tx, err := s.db.Client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting a transaction: %w", err)
	}
	siteEntity, err := tx.Site.Query().Where(site.ID(siteID)).First(ctx)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("query site failed: %w", err)
	}

	updateQuery := tx.Site.UpdateOne(siteEntity)

	if req.Remark != "" {
		updateQuery = updateQuery.SetRemark(req.Remark)
	}

	if req.Timezone != nil {
		updateQuery = updateQuery.SetTimezone(*req.Timezone)
	}

	if req.Public != nil {
		updateQuery = updateQuery.SetPublic(*req.Public)
	}

	if !req.StatsStartDate.IsZero() {
		updateQuery = updateQuery.SetStatsStartDate(req.StatsStartDate)
	}

	if req.IngestRateLimitScaleSeconds != nil {
		updateQuery = updateQuery.SetIngestRateLimitScaleSeconds(*req.IngestRateLimitScaleSeconds)
	}

	if req.IngestLimitPerMinute != nil {
		updateQuery = updateQuery.SetIngestLimitPerMinute(*req.IngestLimitPerMinute)
	}

	if req.AllowedOrigins != nil {
		updateQuery = updateQuery.SetAllowedOrigins(*req.AllowedOrigins)
	}

	siteEntity, err = updateQuery.Save(ctx)
	if err != nil {
		tx.Rollback()
		if ent.IsConstraintError(err) {
			return nil, errors.New("domain already exists")
		}
		return nil, fmt.Errorf("updating site: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	// 更新缓存
	cacheKey := fmt.Sprintf("site:%d", siteEntity.ID)
	s.cache.Store(cacheKey, siteEntity)
	s.InvalidateDomainCache(siteEntity.Domain)
	// 清除用户站点列表缓存
	s.cache.Delete(s.allSitesCacheKey)

	return siteEntity, nil
}

// DeleteSite 删除站点
func (s *SiteService) DeleteSite(ctx *gin.Context, id int) error {
	tx, err := s.db.Client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting a transaction: %w", err)
	}

	// 先查询站点确认存在
	site, err := tx.Site.Query().Where(site.ID(int64(id))).Only(ctx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("site not found: %w", err)
	}

	// 删除站点成员关系
	if _, err := tx.SiteMembership.Delete().Where(sitemembership.SiteID(int64(id))).Exec(ctx); err != nil {
		tx.Rollback()
		return fmt.Errorf("deleting site memberships: %w", err)
	}

	// 删除站点
	if err := tx.Site.DeleteOne(site).Exec(ctx); err != nil {
		tx.Rollback()
		return fmt.Errorf("deleting site: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	// 清除缓存
	cacheKey := fmt.Sprintf("site:%d", id)
	s.cache.Delete(cacheKey)
	s.InvalidateDomainCache(site.Domain)
	s.cache.Delete(s.allSitesCacheKey)

	return nil
}

// IsDomainInList 检查指定域名是否已注册到系统中。
// IsDomainInList 检查域名是否在系统中且已验证。
func (s *SiteService) IsDomainInList(ctx *gin.Context, domain string) (bool, error) {
	_, err := s.GetVerifiedSiteByDomain(ctx, domain)
	if err != nil {
		return false, err
	}

	return true, nil
}

// GetUserSiteCount 获取用户的站点数量
func (s *SiteService) GetUserSiteCount(ctx context.Context, userID int64) (int, error) {
	return s.db.Client.SiteMembership.Query().
		Where(sitemembership.UserID(userID)).
		Count(ctx)
}

// GetSiteOwnerUserID 获取站点所有者的 user_id
func (s *SiteService) GetSiteOwnerUserID(ctx context.Context, siteID int64) (int64, error) {
	membership, err := s.db.Client.SiteMembership.Query().
		Where(
			sitemembership.SiteID(siteID),
			sitemembership.RoleEQ("owner"),
		).
		Only(ctx)
	if err != nil {
		return 0, err
	}
	return membership.UserID, nil
}

// InvalidateDomainCache 清除指定域名的缓存。
func (s *SiteService) InvalidateDomainCache(domain string) {
	s.domainCache.Remove(domain)
}

// GetVerifiedSiteByDomain 根据域名获取已验证的站点。
// 仅当站点已验证时返回，否则返回错误。
func (s *SiteService) GetVerifiedSiteByDomain(ctx context.Context, domain string) (*ent.Site, error) {
	site, err := s.GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found or not verified")
	}
	if !site.IsVerified {
		return nil, fmt.Errorf("site not found or not verified")
	}
	return site, nil
}

// GetUserSiteByDomainForUser 获取当前用户在指定域名下的站点（通过 membership 关联）。
func (s *SiteService) GetUserSiteByDomainForUser(ctx context.Context, userID int64, domain string) (*ent.Site, error) {
	sites, err := s.db.Client.Site.Query().
		Where(site.Domain(domain)).
		All(ctx)
	if err != nil || len(sites) == 0 {
		return nil, fmt.Errorf("site not found")
	}

	siteIDs := make([]int64, len(sites))
	for i, st := range sites {
		siteIDs[i] = st.ID
	}

	// 查找用户在这些站点中的 membership
	membership, err := s.db.Client.SiteMembership.Query().
		Where(
			sitemembership.SiteIDIn(siteIDs...),
			sitemembership.UserID(userID),
		).
		WithSite().
		First(ctx)
	if err != nil {
		return nil, fmt.Errorf("site membership not found")
	}

	return membership.Edges.Site, nil
}

// GetVerificationStatus 获取站点的验证状态信息。
func (s *SiteService) GetVerificationStatus(ctx context.Context, domain string, userID int64) (*types.SiteVerificationStatus, error) {
	site, err := s.GetUserSiteByDomainForUser(ctx, userID, domain)
	if err != nil {
		return nil, err
	}

	status := &types.SiteVerificationStatus{
		Domain:     site.Domain,
		IsVerified: site.IsVerified,
		VerifiedAt: site.VerifiedAt,
	}

	// 仅在站点未验证且用户是 owner 时返回 token
	if !site.IsVerified {
		// 检查是否为 owner
		isOwner, err := s.db.Client.SiteMembership.Query().
			Where(
				sitemembership.SiteID(site.ID),
				sitemembership.UserID(userID),
				sitemembership.RoleEQ("owner"),
			).
			Exist(ctx)
		if err == nil && isOwner {
			status.VerificationToken = site.VerificationToken
		}
	}

	return status, nil
}

// VerifySite 验证站点域名所有权。
// 通过获取域名下的验证文件并与 token 比对来确认所有权。
func (s *SiteService) VerifySite(ctx context.Context, domain string, userID int64) error {
	siteEntity, err := s.GetUserSiteByDomainForUser(ctx, userID, domain)
	if err != nil {
		return err
	}

	if siteEntity.IsVerified {
		return errors.New("site already verified")
	}

	// 检查是否为 owner
	isOwner, err := s.db.Client.SiteMembership.Query().
		Where(
			sitemembership.SiteID(siteEntity.ID),
			sitemembership.UserID(userID),
			sitemembership.RoleEQ("owner"),
		).
		Exist(ctx)
	if err != nil || !isOwner {
		return errors.New("only site owner can verify")
	}

	// 启动事务，检查是否有其他同域名站点已验证
	tx, err := s.db.Client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting a transaction: %w", err)
	}

	// 锁定当前站点行
	lockSite, err := tx.Site.Query().Where(site.ID(siteEntity.ID)).Only(ctx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("locking site: %w", err)
	}
	if lockSite.IsVerified {
		tx.Rollback()
		return errors.New("site already verified")
	}

	// 检查是否有其他同域名站点已验证
	otherVerified, err := tx.Site.Query().
		Where(
			site.Domain(domain),
			site.IsVerified(true),
			site.IDNEQ(siteEntity.ID),
		).
		Exist(ctx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("checking other verified sites: %w", err)
	}
	if otherVerified {
		tx.Rollback()
		return errors.New("domain already verified by another user")
	}

	// 获取验证文件内容：先尝试 HTTPS，失败后回退 HTTP
	verificationPath := "/.well-known/zenstats-verification.txt"
	client := &http.Client{Timeout: 10 * time.Second}

	httpsURL := fmt.Sprintf("https://%s%s", domain, verificationPath)
	resp, err := client.Get(httpsURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		// HTTPS 失败，回退到 HTTP
		httpURL := fmt.Sprintf("http://%s%s", domain, verificationPath)
		resp, err = client.Get(httpURL)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to fetch verification file (tried HTTPS and HTTP): %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tx.Rollback()
		return fmt.Errorf("verification file not found (HTTP %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to read verification file: %w", err)
	}

	token := strings.TrimSpace(string(body))
	if token != siteEntity.VerificationToken {
		tx.Rollback()
		return errors.New("verification token mismatch")
	}

	// 验证通过，更新站点状态
	now := time.Now()
	_, err = tx.Site.UpdateOneID(siteEntity.ID).
		SetIsVerified(true).
		SetVerifiedAt(now).
		Save(ctx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("updating site verification status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	// 清除缓存
	s.InvalidateDomainCache(domain)
	s.cache.Delete(s.allSitesCacheKey)

	return nil
}

package service

import (
	"context"
	"errors"
	"fmt"
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

type SiteService struct {
	db               *postgresql.Client
	cache            sync.Map
	allSitesCacheKey string

	domainCache             *expirable.LRU[string, *ent.Site]
	shieldRuleIPCache       *expirable.LRU[string, []*ent.ShieldRulesIp]
	shieldRuleHostnameCache *expirable.LRU[string, []*ent.ShieldRulesHostname]
	shieldRuleCountryCache  *expirable.LRU[string, []*ent.ShieldRulesCountry]
}

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
		siteServiceInstance = &SiteService{
			db:                      db,
			domainCache:             l,
			shieldRuleIPCache:       ipCache,
			shieldRuleHostnameCache: hostnameCache,
			shieldRuleCountryCache:  countryCache,
			cache:                   sync.Map{},
			allSitesCacheKey:        "all_sites",
		}
	})
	return siteServiceInstance
}

type CreateSiteParams struct {
	Domain       string
	Timezone     string
	Remark       string
	IngestConfig IngestConfig
}

type IngestConfig struct {
	RateLimitScaleSeconds int
	LimitPerMinute        int
}

func (s *SiteService) CreateSite(ctx *gin.Context, params CreateSiteParams) (*ent.Site, error) {
	tx, err := s.db.Client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting a transaction: %w", err)
	}

	site, err := s.db.Client.Site.Create().
		SetDomain(params.Domain).
		SetTimezone(params.Timezone).
		SetRemark(params.Remark).
		SetIngestRateLimitScaleSeconds(params.IngestConfig.RateLimitScaleSeconds).
		SetIngestLimitPerMinute(params.IngestConfig.LimitPerMinute).
		Save(ctx)

	// 判断err 是否是唯一约束错误
	if ent.IsConstraintError(err) {
		tx.Rollback()
		return nil, errors.New("domain already exists")
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

func (s *SiteService) GetSiteByDomain(ctx context.Context, domain string) (*ent.Site, error) {
	if site, ok := s.domainCache.Get(domain); ok {
		return site, nil
	}

	site, err := s.db.Client.Site.Query().Where(site.Domain(domain)).Only(ctx)
	if err != nil {
		return nil, err
	}
	s.domainCache.Add(domain, site)

	return site, nil
}

type SiteWithRemark struct {
	ID                          int64  `json:"id"`
	Domain                      string `json:"domain"`
	Timezone                    string `json:"timezone"`
	Remark                      string `json:"remark"`
	IngestRateLimitScaleSeconds int    `json:"rate_seconds"`
	IngetLimitPerMinute         int    `json:"limit_minute"`
	Role                        string `json:"role"`
}

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
			Role:                        sm.Role.String(),
		}
	}

	return sites, nil
}

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
			Role:                        sm.Role.String(),
		}
		sites = append(sites, site)
	}

	return sites, nil
}

// 新增：完成 GetSiteList 方法，使用缓存
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

// AddShieldRuleIP 添加IP屏蔽规则
func (s *SiteService) AddShieldRuleIP(ctx *gin.Context, domain string, ip string, action string, description string) (*ent.ShieldRulesIp, error) {
	// 通过domain获取站点信息
	site, err := s.GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	ipInet, err := utils.ParseInet(ip)
	if err != nil {
		return nil, fmt.Errorf("invalid IP format: %w", err)
	}

	// 创建IP屏蔽规则
	rule, err := s.db.Client.ShieldRulesIp.Create().
		SetSiteID(site.ID).
		SetInet(ipInet).
		SetAction(action).
		SetDescription(description).
		SetAddedBy("system").
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create shield rule ip failed: %w", err)
	}

	// 添加成功后清除缓存
	s.shieldRuleIPCache.Remove(domain)
	return rule, nil
}

// ListShieldRuleIP 获取IP屏蔽规则列表
func (s *SiteService) ListShieldRuleIP(ctx context.Context, domain string) ([]*ent.ShieldRulesIp, error) {
	// 尝试从缓存获取
	if rules, ok := s.shieldRuleIPCache.Get(domain); ok {
		return rules, nil
	}

	// 通过domain获取站点信息
	site, err := s.GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	// 查询该站点的所有IP屏蔽规则
	rules, err := s.db.Client.ShieldRulesIp.Query().
		Where(shieldrulesip.SiteID(site.ID)).
		Order(ent.Desc(shieldrulesip.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query shield rule ip failed: %w", err)
	}

	// 存入缓存
	s.shieldRuleIPCache.Add(domain, rules)

	return rules, nil
}

// RemoveShieldRuleIP 删除IP屏蔽规则
func (s *SiteService) RemoveShieldRuleIP(ctx context.Context, domain string, ruleID int64) error {
	// 通过domain获取站点信息
	site, err := s.GetSiteByDomain(ctx, domain)
	if err != nil {
		return fmt.Errorf("site not found: %w", err)
	}

	// 验证规则是否属于该站点
	count, err := s.db.Client.ShieldRulesIp.Query().
		Where(
			shieldrulesip.ID(ruleID),
			shieldrulesip.SiteID(site.ID),
		).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("query shield rule ip failed: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("shield rule ip not found or not belongs to site")
	}

	// 删除规则
	if err := s.db.Client.ShieldRulesIp.DeleteOneID(ruleID).Exec(ctx); err != nil {
		return fmt.Errorf("delete shield rule ip failed: %w", err)
	}

	// 删除成功后清除缓存
	s.shieldRuleHostnameCache.Remove(domain)
	return nil
}

// AddShieldRuleHostname 添加Hostname屏蔽规则
func (s *SiteService) AddShieldRuleHostname(ctx context.Context, domain string, hostname string, hostnamePattern string, action string) (*ent.ShieldRulesHostname, error) {
	// 验证站点是否存在
	site, err := s.db.Client.Site.Query().Where(site.Domain(domain)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	// 创建Hostname屏蔽规则
	rule, err := s.db.Client.ShieldRulesHostname.Create().
		SetSiteID(site.ID).
		SetHostname(hostname).
		SetHostnamePattern(hostnamePattern).
		SetAction(action).
		SetAddedBy("system").
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create shield rule hostname failed: %w", err)
	}

	// 添加成功后清除缓存
	s.shieldRuleHostnameCache.Remove(domain)

	return rule, nil
}

// ListShieldRuleHostname 获取Hostname屏蔽规则列表
func (s *SiteService) ListShieldRuleHostname(ctx context.Context, domain string) ([]*ent.ShieldRulesHostname, error) {
	// 尝试从缓存获取
	if rules, ok := s.shieldRuleHostnameCache.Get(domain); ok {
		return rules, nil
	}

	// 通过domain获取站点信息
	site, err := s.GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	// 查询该站点的所有Hostname屏蔽规则
	rules, err := s.db.Client.ShieldRulesHostname.Query().
		Where(shieldruleshostname.SiteID(site.ID)).
		Order(ent.Desc(shieldruleshostname.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query shield rule hostname failed: %w", err)
	}

	// 存入缓存
	s.shieldRuleHostnameCache.Add(domain, rules)

	return rules, nil
}

// RemoveShieldRuleHostname 删除Hostname屏蔽规则
func (s *SiteService) RemoveShieldRuleHostname(ctx *gin.Context, domain string, ruleID int64) error {
	site, err := s.GetSiteByDomain(ctx, domain)
	if err != nil {
		return fmt.Errorf("site not found: %w", err)
	}

	// 验证规则是否属于该站点
	count, err := s.db.Client.ShieldRulesHostname.Query().
		Where(
			shieldruleshostname.ID(ruleID),
			shieldruleshostname.SiteID(site.ID),
		).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("query shield rule hostname failed: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("shield rule hostname not found or not belongs to site")
	}

	// 删除规则
	if err := s.db.Client.ShieldRulesHostname.DeleteOneID(ruleID).Exec(ctx); err != nil {
		return fmt.Errorf("delete shield rule hostname failed: %w", err)
	}
	// 删除成功后清除缓存
	s.shieldRuleHostnameCache.Remove(domain)

	return nil
}

// AddShieldRuleCountry 添加Country屏蔽规则
func (s *SiteService) AddShieldRuleCountry(ctx *gin.Context, domain string, countryCode string, action string) (*ent.ShieldRulesCountry, error) {
	// 验证站点是否存在
	site, err := s.db.Client.Site.Query().Where(site.Domain(domain)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}
	// 获取当前登陆用户
	user, _ := GetUserService().GetUserByID(ctx, ctx.GetInt64("user_id"))
	// 创建Country屏蔽规则
	rule, err := s.db.Client.ShieldRulesCountry.Create().
		SetSiteID(site.ID).
		SetCountryCode(countryCode).
		SetAction(action).
		SetAddedBy(user.Name).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create shield rule country failed: %w", err)
	}

	// 添加成功后清除缓存
	s.shieldRuleCountryCache.Remove(domain)

	return rule, nil
}

// ListShieldRuleCountry 获取Country屏蔽规则列表
func (s *SiteService) ListShieldRuleCountry(ctx context.Context, domain string) ([]*ent.ShieldRulesCountry, error) {
	// 尝试从缓存获取
	if rules, ok := s.shieldRuleCountryCache.Get(domain); ok {
		return rules, nil
	}

	// 通过domain获取站点信息
	site, err := s.GetSiteByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	// 查询该站点的所有Country屏蔽规则
	rules, err := s.db.Client.ShieldRulesCountry.Query().
		Where(shieldrulescountry.SiteID(site.ID)).
		Order(ent.Desc(shieldrulescountry.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query shield rule country failed: %w", err)
	}

	// 存入缓存
	s.shieldRuleCountryCache.Add(domain, rules)

	return rules, nil
}

// RemoveShieldRuleCountry 删除Country屏蔽规则
func (s *SiteService) RemoveShieldRuleCountry(ctx *gin.Context, domain string, ruleID int64) error {
	site, err := s.GetSiteByDomain(ctx, domain)
	if err != nil {
		return fmt.Errorf("site not found: %w", err)
	}

	// 验证规则是否属于该站点
	count, err := s.db.Client.ShieldRulesCountry.Query().
		Where(
			shieldrulescountry.ID(ruleID),
			shieldrulescountry.SiteID(site.ID),
		).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("query shield rule country failed: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("shield rule country not found or not belongs to site")
	}

	// 删除规则
	if err := s.db.Client.ShieldRulesCountry.DeleteOneID(ruleID).Exec(ctx); err != nil {
		return fmt.Errorf("delete shield rule country failed: %w", err)
	}
	// 删除成功后清除缓存
	s.shieldRuleCountryCache.Remove(domain)

	return nil
}

// UpdateSite 更新站点信息
func (s *SiteService) UpdateSite(ctx *gin.Context, domain string, req types.UpdateSiteRequest) (*ent.Site, error) {
	tx, err := s.db.Client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting a transaction: %w", err)
	}
	siteEntity, err := tx.Site.Query().Where(site.Domain(domain)).First(ctx)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("query site failed: %w", err)
	}

	updateQuery := tx.Site.UpdateOne(siteEntity)

	// 更新 remark 字段，仅当 remark 不为空字符串时更新
	if req.Remark != "" {
		updateQuery = updateQuery.SetRemark(req.Remark)
	}

	// 更新 timezone 字段
	if req.Timezone != nil {
		updateQuery = updateQuery.SetTimezone(*req.Timezone)
	}

	// 更新 public 字段
	if req.Public != nil {
		updateQuery = updateQuery.SetPublic(*req.Public)
	}

	// 更新 stats_start_date 字段
	if !req.StatsStartDate.IsZero() {
		updateQuery = updateQuery.SetStatsStartDate(req.StatsStartDate)
	}

	// 更新 ingest_rate_limit_scale_seconds 字段
	updateQuery = updateQuery.SetIngestRateLimitScaleSeconds(req.IngestRateLimitScaleSeconds)

	// 更新 ingest_limit_per_minute 字段
	updateQuery = updateQuery.SetIngestLimitPerMinute(req.IngestLimitPerMinute)

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
	s.domainCache.Add(domain, siteEntity)
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
	s.domainCache.Remove(site.Domain)
	s.cache.Delete(s.allSitesCacheKey)

	return nil
}

// IsDomainInList
func (s *SiteService) IsDomainInList(ctx *gin.Context, domain string) (bool, error) {
	_, err := s.GetSiteByDomain(ctx, domain)
	if err != nil {
		return false, err
	}

	return true, nil
}

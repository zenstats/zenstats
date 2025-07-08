package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/site"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/sitemembership"
	"github.com/zenstats/zenstats/pkg/globals"
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

	domainCache *expirable.LRU[string, *ent.Site]
}

func GetSiteService() *SiteService {
	siteOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		l := expirable.NewLRU[string, *ent.Site](1000, nil, 30*time.Minute)
		siteServiceInstance = &SiteService{
			db:               db,
			domainCache:      l,
			cache:            sync.Map{},
			allSitesCacheKey: "all_sites",
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
	IngestRateLimitScaleSeconds int    `json:"ingestRateLimitScaleSeconds"`
	IngetLimitPerMinute         int    `json:"ingestLimitPerMinute"`
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

// IsDomainInList
func (s *SiteService) IsDomainInList(ctx *gin.Context, domain string) (bool, error) {
	_, err := s.GetSiteByDomain(ctx, domain)
	if err != nil {
		return false, err
	}

	return true, nil
}

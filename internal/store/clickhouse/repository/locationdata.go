package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/pkg/utils/random"
)

var (
	locationInstance *LocationRepository
)

var keyLocks = struct {
	sync.Mutex
	m map[string]*sync.Mutex
}{
	m: make(map[string]*sync.Mutex),
}

func getLock(key string) *sync.Mutex {
	keyLocks.Lock()
	defer keyLocks.Unlock()

	if _, ok := keyLocks.m[key]; !ok {
		keyLocks.m[key] = &sync.Mutex{}
	}
	return keyLocks.m[key]
}

type LocationRepository struct {
	conn  clickhouse.Conn
	cache *expirable.LRU[string, *models.LocationData]
}

func GetLocationRepository() *LocationRepository {
	locationOnce.Do(func() {
		conn := cl.GetConnection()
		l := expirable.NewLRU[string, *models.LocationData](1000, nil, 30*time.Minute)

		locationInstance = &LocationRepository{
			conn:  conn,
			cache: l,
		}
	})

	return locationInstance
}

func (r *LocationRepository) GetOrCreate(ctx context.Context, dictType, name string) (*models.LocationData, error) {

	cacheKey := fmt.Sprintf("location_data_%s_%s", dictType, name)
	if val, ok := r.cache.Get(cacheKey); ok {
		return val, nil
	}

	lockKey := fmt.Sprintf("%s_%s", dictType, name)
	lock := getLock(lockKey)
	lock.Lock()
	defer lock.Unlock()

	// check again
	if val, ok := r.cache.Get(cacheKey); ok {
		return val, nil
	}

	// 1. 尝试查询现有数据
	data, err := r.getByName(ctx, dictType, name)
	if err == nil && data != nil {
		// Cache the result
		r.cache.Add(cacheKey, data)

		return data, nil
	}

	// 2. 如果不存在则创建新记录
	data, err = r.create(ctx, dictType, name)
	if err == nil && data != nil {
		r.cache.Add(cacheKey, data)
	}

	return data, err
}

func (r *LocationRepository) GetOrCreateById(ctx context.Context, dictType, name, id string) (*models.LocationData, error) {

	cacheKey := fmt.Sprintf("location_data_id_%s_%s_%s", dictType, name, id)
	if val, ok := r.cache.Get(cacheKey); ok {
		return val, nil
	}

	lockKey := fmt.Sprintf("%s_%s", dictType, id)
	lock := getLock(lockKey)
	lock.Lock()
	defer lock.Unlock()

	// check again
	if val, ok := r.cache.Get(cacheKey); ok {
		return val, nil
	}

	// 1. 尝试查询现有数据
	data, err := r.getByName(ctx, dictType, name)
	if err == nil && data != nil {
		r.cache.Add(cacheKey, data)

		return data, nil
	}

	// 2. 如果不存在则创建新记录
	data, err = r.createById(ctx, dictType, name, id)
	if err == nil && data != nil {
		r.cache.Add(cacheKey, data)
	}

	return data, err
}

func (r *LocationRepository) getByName(ctx context.Context, dictType, name string) (*models.LocationData, error) {
	query := `SELECT id, name, type FROM location_data WHERE type = ? AND name = ? LIMIT 1`
	row := r.conn.QueryRow(ctx, query, dictType, name)
	var data models.LocationData
	if err := row.Scan(
		&data.ID,
		&data.Name,
		&data.Type,
	); err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}

	return &data, nil
}

func (r *LocationRepository) create(ctx context.Context, dictType, name string) (*models.LocationData, error) {
	data := &models.LocationData{
		Name: name,
		Type: dictType,
	}

	query := `
		INSERT INTO location_data (
			name,
			id,
			type,
		) VALUES (?, ?, ?)`

	data.ID = random.String(16)
	if err := r.conn.Exec(ctx, query,
		data.Name,
		data.ID,
		data.Type,
	); err != nil {
		return nil, fmt.Errorf("插入失败: %w", err)
	}

	// 获取刚插入的记录的ID（如果表有自增ID）
	row := r.conn.QueryRow(ctx, "SELECT id FROM location_data WHERE type = ? AND name = ? LIMIT 1", dictType, name)
	if err := row.Scan(&data.ID); err != nil {
		return nil, fmt.Errorf("获取ID失败: %w", err)
	}

	return data, nil
}

func (r *LocationRepository) createById(ctx context.Context, dictType, name, id string) (*models.LocationData, error) {
	data := &models.LocationData{
		Name: name,
		Type: dictType,
	}

	query := `
		INSERT INTO location_data (
			name,
			id,
			type,
		) VALUES (?, ?, ?)`

	if err := r.conn.Exec(ctx, query,
		data.Name,
		id,
		data.Type,
	); err != nil {
		return nil, fmt.Errorf("插入失败: %w", err)
	}

	// 获取刚插入的记录的ID（如果表有自增ID）
	row := r.conn.QueryRow(ctx, "SELECT id FROM location_data WHERE type = ? AND name = ? LIMIT 1", dictType, name)
	if err := row.Scan(&data.ID); err != nil {
		return nil, fmt.Errorf("获取ID失败: %w", err)
	}

	return data, nil
}

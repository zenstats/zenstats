package pool

import (
	"github.com/zenstats/zenstats/config"

	"github.com/panjf2000/ants"
)

var pool *Pool

type Pool struct {
	pool *ants.Pool
}

func NewPool() *Pool {
	cp, err := ants.NewPool(config.Conf.PoolSize)
	if err != nil {
		panic(err)
	}
	if pool == nil {
		pool = &Pool{
			pool: cp,
		}
	}

	return pool
}

func (p *Pool) Submit(f func()) {
	p.pool.Submit(f)
}
func (p *Pool) Release() {
	p.pool.Release()
}

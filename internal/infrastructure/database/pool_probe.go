package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolProbe abstracts connectivity and pool usage for health checks without exposing pgx types to HTTP handlers.
type PoolProbe interface {
	Ping(ctx context.Context) error
	AcquiredConnections() int32
}

type poolProbe struct {
	pool *pgxpool.Pool
}

// NewPoolProbe wraps a pgx pool for readiness / health probes.
func NewPoolProbe(pool *pgxpool.Pool) PoolProbe {
	return &poolProbe{pool: pool}
}

func (p *poolProbe) Ping(ctx context.Context) error {
	if p.pool == nil {
		return nil
	}
	return p.pool.Ping(ctx)
}

func (p *poolProbe) AcquiredConnections() int32 {
	if p.pool == nil {
		return 0
	}
	return p.pool.Stat().AcquiredConns()
}

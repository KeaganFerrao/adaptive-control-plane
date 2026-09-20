package models

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ComponentStore interface {
	Create(ctx context.Context, component *Component) error
	Get(ctx context.Context, id uuid.UUID) (*Component, error)
	GetByName(ctx context.Context, name string) (*Component, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ComponentType string

const (
	ComponentTypeCollector  ComponentType = "otel-collector"
	ComponentTypeClickHouse ComponentType = "clickhouse"
	ComponentTypePostgres   ComponentType = "postgres"
	ComponentTypeNginx      ComponentType = "nginx"
)

type Component struct {
	ID        uuid.UUID     `db:"id"`
	Name      string        `db:"name"`
	Type      ComponentType `db:"type"`
	CreatedAt time.Time     `db:"created_at"`
}

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/KeaganFerrao/adaptive-control-plane/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresComponentRepository struct {
	Db *pgxpool.Pool
}

func NewPostgresComponentRepository(db *pgxpool.Pool) *PostgresComponentRepository {
	return &PostgresComponentRepository{
		Db: db,
	}
}

func (c PostgresComponentRepository) Create(ctx context.Context, component *models.Component) error {
	if component == nil {
		return errors.New("component cannot be nil")
	}

	query := `
		INSERT INTO components (
			name,
			type
		)
		VALUES ($1, $2)
		RETURNING created_at
		`

	err := c.Db.QueryRow(
		ctx,
		query,
		component.Name,
		component.Type,
	).Scan(&component.CreatedAt)

	if err != nil {
		return fmt.Errorf("create component: %w", err)
	}

	return nil
}

func (c PostgresComponentRepository) Get(ctx context.Context, id uuid.UUID) (*models.Component, error) {
	query := `
		SELECT
		*
		FROM components
		WHERE id = $1
		`

	var component models.Component

	err := c.Db.QueryRow(
		ctx,
		query,
		id,
	).Scan(
		&component.ID,
		&component.Name,
		&component.Type,
		&component.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("component %s not found: %w", id, err)
		}

		return nil, fmt.Errorf("get component %s: %w", id, err)
	}

	return &component, nil
}

func (c PostgresComponentRepository) GetByName(ctx context.Context, name string) (*models.Component, error) {
	query := `
		SELECT
		id,
		name,
		type,
		created_at
		FROM components
		WHERE name = $1
		`

	var component models.Component

	err := c.Db.QueryRow(
		ctx,
		query,
		name,
	).Scan(
		&component.ID,
		&component.Name,
		&component.Type,
		&component.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("component %q not found: %w", name, err)
		}

		return nil, fmt.Errorf("get component by name %q: %w", name, err)
	}

	return &component, nil
}

func (c PostgresComponentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		DELETE FROM components
		WHERE id = $1
		`

	tag, err := c.Db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete component %s: %w", id, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("component %s not found", id)
	}

	return nil
}

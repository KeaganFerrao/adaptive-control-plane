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

var ErrPolicyNotFound = errors.New("policy not found")

type PostgresPolicyRepository struct {
	Db *pgxpool.Pool
}

func NewPostgresPolicyRepository(db *pgxpool.Pool) *PostgresPolicyRepository {
	return &PostgresPolicyRepository{
		Db: db,
	}
}

func (p *PostgresPolicyRepository) CreateOne(ctx context.Context, policy *models.Policy) error {
	if policy == nil {
		return errors.New("policy must not be nil")
	}

	const query = `
		INSERT INTO policies (
			policy_name,
			status,
			status_updated_at,
			condition_first_seen_at,
			recovery_first_seen_at,
			triggered_at,
			recovered_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT DO NOTHING;
	`

	_, err := p.Db.Exec(
		ctx,
		query,
		policy.PolicyName,
		policy.Status,
		policy.StatusUpdatedAt,
		policy.ConditionFirstSeenAt,
		policy.RecoveryFirstSeenAt,
		policy.TriggeredAt,
		policy.RecoveredAt,
		policy.CreatedAt,
		policy.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create policy %q: %w", policy.PolicyName, err)
	}

	return nil
}

func (p *PostgresPolicyRepository) CreateMany(ctx context.Context, policies []*models.Policy) error {
	if len(policies) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(policies))

	for i, policy := range policies {
		if policy == nil {
			return fmt.Errorf("policy at index %d is nil", i)
		}

		rows = append(rows, []any{
			policy.PolicyName,
			policy.Status,
			policy.StatusUpdatedAt,
			policy.ConditionFirstSeenAt,
			policy.RecoveryFirstSeenAt,
			policy.TriggeredAt,
			policy.RecoveredAt,
			policy.CreatedAt,
			policy.UpdatedAt,
		})
	}

	_, err := p.Db.CopyFrom(
		ctx,
		pgx.Identifier{"policies"},
		[]string{
			"policy_name",
			"status",
			"status_updated_at",
			"condition_first_seen_at",
			"recovery_first_seen_at",
			"triggered_at",
			"recovered_at",
			"created_at",
			"updated_at",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("create %d policies: %w", len(policies), err)
	}

	return nil
}

func (p *PostgresPolicyRepository) Get(ctx context.Context, id uuid.UUID) (*models.Policy, error) {
	query := `
		SELECT * FROM policies
		WHERE id = $1
	`

	var policy models.Policy
	err := p.Db.QueryRow(ctx, query, id).Scan(
		&policy.ID,
		&policy.PolicyName,
		&policy.Status,
		&policy.StatusUpdatedAt,
		&policy.ConditionFirstSeenAt,
		&policy.RecoveryFirstSeenAt,
		&policy.TriggeredAt,
		&policy.RecoveredAt,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}

		return nil, fmt.Errorf("get policy %s: %w", id, err)
	}

	return &policy, nil
}

func (p *PostgresPolicyRepository) GetByName(ctx context.Context, name string) (*models.Policy, error) {
	query := `
		SELECT * FROM policies
		WHERE policy_name = $1
	`

	var policy models.Policy

	err := p.Db.QueryRow(ctx, query, name).Scan(
		&policy.ID,
		&policy.PolicyName,
		&policy.Status,
		&policy.StatusUpdatedAt,
		&policy.ConditionFirstSeenAt,
		&policy.RecoveryFirstSeenAt,
		&policy.TriggeredAt,
		&policy.RecoveredAt,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}

		return nil, fmt.Errorf("get policy by name %q: %w", name, err)
	}

	return &policy, nil
}

func (p *PostgresPolicyRepository) DeleteAll(ctx context.Context) error {
	const query = `
		DELETE FROM policies
	`

	if _, err := p.Db.Exec(ctx, query); err != nil {
		return fmt.Errorf("delete all policies: %w", err)
	}

	return nil
}

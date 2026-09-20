package models

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type PolicyStore interface {
	CreateOne(ctx context.Context, policy *Policy) error
	CreateMany(ctx context.Context, policy []*Policy) error
	Get(ctx context.Context, id uuid.UUID) (*Policy, error)
	GetByName(ctx context.Context, name string) (*Policy, error)
	DeleteAll(ctx context.Context) error
}

type PolicyManager interface {
	// Load loads the policies from the source, validates and gives us a valid Policy which can be saved and used everywhere
	Load(ctx context.Context) ([]*Policy, error)

	// Save saves the Policy into persistant storage
	Save(ctx context.Context, definition *Policy) error
}

type PolicyEvaluator interface {
	Evaluate(ctx context.Context, policy *Policy) (EvaluationResult, error)
}

type EvaluationResult struct {
	TriggerCondition  ConditionResult
	RecoveryCondition ConditionResult
}

type ConditionResult int

const (
	ConditionFalse ConditionResult = iota + 1
	ConditionTrue
	ConditionUnknown
)

type PolicyStatus string

const (
	PolicyStatusNormal          PolicyStatus = "normal"
	PolicyStatusPendingTrigger  PolicyStatus = "pending_trigger"
	PolicyStatusTriggered       PolicyStatus = "triggered"
	PolicyStatusPendingRecovery PolicyStatus = "pending_recovery"
)

type Policy struct {
	ID                       uuid.UUID                 `db:"id"`
	PolicyName               string                    `db:"policy_name"`
	Status                   PolicyStatus              `db:"status"`
	StatusUpdatedAt          *time.Time                `db:"status_updated_at"`
	ConditionFirstSeenAt     *time.Time                `db:"condition_first_seen_at"`
	RecoveryFirstSeenAt      *time.Time                `db:"recovery_first_seen_at"`
	TriggeredAt              *time.Time                `db:"triggered_at"`
	RecoveredAt              *time.Time                `db:"recovered_at"`
	CreatedAt                time.Time                 `db:"created_at"`
	UpdatedAt                *time.Time                `db:"updated_at"`
	PolicyDefinition         *PolicyDefinition         `db:"-"`
	InterestedMetrics        *ComponentMetrics         `db:"-"`
	ComponentMetricSnapshots *ComponentMetricSnapshots `db:"-"`
	mu                       sync.RWMutex              `db:"-"`
}

package policymanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/KeaganFerrao/adaptive-control-plane/internal/models"
	"gopkg.in/yaml.v3"
)

type YAMLPolicyManager struct {
	directoryPath  string
	policyStore    models.PolicyStore
	componentStore models.ComponentStore
	actionRegistry models.ActionRegistry
}

func NewYAMLPolicyManager(directoryPath string, policyStore models.PolicyStore, componentStore models.ComponentStore, actionRegistry models.ActionRegistry) *YAMLPolicyManager {
	return &YAMLPolicyManager{
		directoryPath:  directoryPath,
		policyStore:    policyStore,
		componentStore: componentStore,
		actionRegistry: actionRegistry,
	}
}

func (p *YAMLPolicyManager) Load(ctx context.Context) ([]*models.Policy, error) {
	if strings.TrimSpace(p.directoryPath) == "" {
		return nil, errors.New("policy directory path is empty")
	}

	entries, err := os.ReadDir(p.directoryPath)
	if err != nil {
		return nil, fmt.Errorf("read policy directory %q: %w", p.directoryPath, err)
	}

	policies := make([]*models.Policy, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !p.isYAMLFile(entry.Name()) {
			continue
		}

		filePath := filepath.Join(p.directoryPath, entry.Name())

		definition, err := p.loadDefinition(filePath)
		if err != nil {
			return nil, err
		}

		err = p.setActionHandlers(definition)
		if err != nil {
			return nil, err
		}

		err = p.validateDefinition(ctx, definition)
		if err != nil {
			return nil, err
		}

		interestedMetrics, err := p.getInterestedMetricsFromPolicy(ctx, definition)
		if err != nil {
			return nil, err
		}

		policy := &models.Policy{
			PolicyName:        definition.Name,
			Status:            models.PolicyStatusNormal,
			PolicyDefinition:  definition,
			InterestedMetrics: interestedMetrics,
		}

		policies = append(policies, policy)
	}

	return policies, nil
}

func (p *YAMLPolicyManager) loadDefinition(filePath string) (*models.PolicyDefinition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read policy file %q: %w", filePath, err)
	}

	var definition models.PolicyDefinition

	if err := yaml.Unmarshal(data, &definition); err != nil {
		return nil, fmt.Errorf("unmarshal policy file %q: %w", filePath, err)
	}

	return &definition, nil
}

func (p *YAMLPolicyManager) setActionHandlers(definition *models.PolicyDefinition) error {
	if definition == nil {
		return fmt.Errorf("policy definition cannot be nil")
	}

	setHandlers := func(actions []models.PolicyAction) error {
		for i := range actions {
			action := &actions[i]

			handler, ok := p.actionRegistry[action.Action]
			if !ok {
				return fmt.Errorf("unknown action %v", action.Action)
			}

			action.Handler = handler
		}

		return nil
	}

	// Actions executed when the policy triggers.
	if err := setHandlers(definition.Then); err != nil {
		return fmt.Errorf("set policy action handlers: %w", err)
	}

	// Actions executed when the policy recovers.
	if err := setHandlers(definition.Recover.Then); err != nil {
		return fmt.Errorf("set recovery action handlers: %w", err)
	}

	return nil
}

func (p *YAMLPolicyManager) isYAMLFile(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))

	return extension == ".yaml" || extension == ".yml"
}

func (p *YAMLPolicyManager) validateDefinition(ctx context.Context, definition *models.PolicyDefinition) error {
	return nil
}

func (p *YAMLPolicyManager) Save(ctx context.Context, policy *models.Policy) error {
	err := p.policyStore.CreateOne(ctx, policy)
	if err != nil {
		return err
	}

	return nil
}

func (p *YAMLPolicyManager) getInterestedMetricsFromPolicy(ctx context.Context, policy *models.PolicyDefinition) (*models.ComponentMetrics, error) {
	interestedMetrics := make(models.ComponentMetrics)

	// Process the main condition.
	if err := p.collectInterestedMetrics(
		ctx,
		policy.When,
		interestedMetrics,
	); err != nil {
		return nil, err
	}

	// Process recovery conditions as well.
	if err := p.collectInterestedMetrics(
		ctx,
		policy.Recover.When,
		interestedMetrics,
	); err != nil {
		return nil, err
	}

	return &interestedMetrics, nil
}

func (p *YAMLPolicyManager) collectInterestedMetrics(ctx context.Context, expression models.Expression, interestedMetrics models.ComponentMetrics) error {
	// Leaf condition.
	if expression.Condition != nil {
		condition := expression.Condition

		if condition.Metric == "" {
			return errors.New("condition metric cannot be empty")
		}

		if condition.Scope == nil {
			return errors.New("condition scope cannot be nil")
		}

		if condition.Scope.Component == "" {
			return errors.New("condition scope.component cannot be empty")
		}

		component, err := p.componentStore.GetByName(ctx, condition.Scope.Component)
		if err != nil {
			return fmt.Errorf(
				"get component %q: %w",
				condition.Scope.Component,
				err,
			)
		}

		interestedMetrics[*component] = p.appendUniqueMetric(
			interestedMetrics[*component],
			condition.Metric,
		)
	}

	// AND expressions.
	for _, child := range expression.And {
		if err := p.collectInterestedMetrics(
			ctx,
			child,
			interestedMetrics,
		); err != nil {
			return err
		}
	}

	// OR expressions.
	for _, child := range expression.Or {
		if err := p.collectInterestedMetrics(
			ctx,
			child,
			interestedMetrics,
		); err != nil {
			return err
		}
	}

	return nil
}

func (p *YAMLPolicyManager) appendUniqueMetric(metrics models.MetricSpec, metric string) models.MetricSpec {
	if slices.Contains(metrics, metric) {
		return metrics
	}

	return append(metrics, metric)
}

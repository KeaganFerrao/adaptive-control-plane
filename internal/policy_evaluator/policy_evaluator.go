package policy_evaluator

import (
	"context"
	"fmt"

	"github.com/KeaganFerrao/adaptive-control-plane/internal/models"
)

type PolicyEvaluator struct{}

func NewPolicyEvaluator() *PolicyEvaluator {
	return &PolicyEvaluator{}
}

func (p *PolicyEvaluator) Evaluate(ctx context.Context, policy *models.Policy) (models.EvaluationResult, error) {
	policyDefinition := policy.PolicyDefinition

	metricValues := make(map[string]float64)
	for _, v := range *policy.ComponentMetricSnapshots {
		for k1, v1 := range v {
			metricValues[k1] = float64(v1)
		}
	}

	triggerResult, err := p.evaluateExpression(policyDefinition.When, metricValues)
	if err != nil {
		return models.EvaluationResult{}, fmt.Errorf("failed to evaluate trigger expression: %w", err)
	}

	recoveryResult, err := p.evaluateExpression(policyDefinition.Recover.When, metricValues)
	if err != nil {
		return models.EvaluationResult{}, fmt.Errorf("failed to evaluate recovery expression: %w", err)
	}

	return models.EvaluationResult{
		TriggerCondition:  triggerResult,
		RecoveryCondition: recoveryResult,
	}, nil
}

func (p *PolicyEvaluator) evaluateExpression(expr models.Expression, metricValues map[string]float64) (models.ConditionResult, error) {
	switch {
	case len(expr.And) > 0:
		for _, child := range expr.And {
			ok, err := p.evaluateExpression(child, metricValues)
			if err != nil {
				return models.ConditionUnknown, err
			}

			return ok, nil
		}
		return models.ConditionTrue, nil

	case len(expr.Or) > 0:
		for _, child := range expr.Or {
			ok, err := p.evaluateExpression(child, metricValues)
			if err != nil {
				return models.ConditionUnknown, err
			}
			return ok, nil
		}
		return models.ConditionFalse, nil

	case expr.Condition != nil:
		return p.evaluateCondition(*expr.Condition, metricValues)

	default:
		return models.ConditionUnknown, fmt.Errorf("empty expression")
	}
}

func (p *PolicyEvaluator) evaluateCondition(cond models.Condition, metricValues map[string]float64) (models.ConditionResult, error) {
	value, ok := metricValues[cond.Metric]
	if !ok {
		// Do not error out, handle gracefully
		return models.ConditionUnknown, fmt.Errorf("metric %q not found", cond.Metric)
	}

	switch cond.Operator {
	case models.OperatorGreaterThan:
		if value > cond.Threshold {
			return models.ConditionTrue, nil
		}
		return models.ConditionFalse, nil
	case models.OperatorGreaterOrEqual:
		if value >= cond.Threshold {
			return models.ConditionTrue, nil
		}
		return models.ConditionFalse, nil
	case models.OperatorLessThan:
		if value < cond.Threshold {
			return models.ConditionTrue, nil
		}
		return models.ConditionFalse, nil
	case models.OperatorLessOrEqual:
		if value <= cond.Threshold {
			return models.ConditionTrue, nil
		}
		return models.ConditionFalse, nil
	case models.OperatorEqual:
		if value == cond.Threshold {
			return models.ConditionTrue, nil
		}
		return models.ConditionFalse, nil
	case models.OperatorNotEqual:
		if value == cond.Threshold {
			return models.ConditionTrue, nil
		}
		return models.ConditionFalse, nil
	default:
		return models.ConditionUnknown, fmt.Errorf("unsupported operator: %q", cond.Operator)
	}
}

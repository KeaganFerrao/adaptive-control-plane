package action_runner

import (
	"context"

	"github.com/KeaganFerrao/adaptive-control-plane/internal/models"
)

type ActionRunner struct{}

func NewActionRunner() *ActionRunner {
	return &ActionRunner{}
}

func (a *ActionRunner) Run(ctx context.Context, policyAction []models.PolicyAction) ([]models.ActionResult, error) {
	results := make([]models.ActionResult, len(policyAction))

	for _, action := range policyAction {
		execute, err := action.Handler.Execute(ctx, action.Parameters)
		if err != nil {
			return nil, err
		}

		results = append(results, execute)
	}

	return results, nil
}

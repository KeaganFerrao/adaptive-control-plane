package models

import "context"

type ActionHandler interface {
	Execute(ctx context.Context, params ActionParams) (ActionResult, error)
}

type ActionResult struct {
	ActionName string
	Status     ActionResultStatus
}

type ActionResultStatus string

const (
	ActionResultStatusSuccess ActionResultStatus = "success"
	ActionResultStatusFailure ActionResultStatus = "failure"
)

type ActionParams map[string]any

type ActionRegistry map[ActionName]ActionHandler

type ActionName string

const (
	ActionNameSetCollectorSampling    ActionName = "set_collector_sampling"
	ActionNameSetCollectorMemoryLimit ActionName = "set_collector_mem_limit"
)

type ActionRunner interface {
	Run(ctx context.Context, policyAction []PolicyAction) ([]ActionResult, error)
}

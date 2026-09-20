package actions_registry

import (
	"context"
	"fmt"
	"time"

	"github.com/KeaganFerrao/adaptive-control-plane/internal/models"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/opamp_server/data"
	"github.com/open-telemetry/opamp-go/protobufs"
)

type SetCollectorSamplingAction struct {
	agents  *data.Agents
	timeout time.Duration
}

func NewSetCollectorSamplingAction(agents *data.Agents) *SetCollectorSamplingAction {
	return &SetCollectorSamplingAction{
		agents:  agents,
		timeout: 10 * time.Second,
	}
}

func (s *SetCollectorSamplingAction) Execute(ctx context.Context, params models.ActionParams) (models.ActionResult, error) {
	value, ok := params["value"]
	if !ok {
		return models.ActionResult{}, fmt.Errorf("missing parameter: value")
	}

	setValue, ok := value.(int)
	if !ok {
		return models.ActionResult{}, fmt.Errorf("value must be a int, got %T", value)
	}

	agentMap := s.agents.AgentsById()
	statusUpdateStream := make(chan struct{}, len(agentMap))

	var configStr string
	if setValue == 1 {
		configStr = fmt.Sprintf("service:\n  pipelines:\n    traces:\n      processors: [memory_limiter, tail_sampling, batch]")
	} else {
		configStr = fmt.Sprintf("service:\n  pipelines:\n    traces:\n      processors: [memory_limiter, batch]")
	}

	config := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigObject{
			"": {Body: []byte(configStr)},
		},
	}

	for instanceId := range agentMap {
		s.agents.SetCustomConfigForAgent(instanceId, config, statusUpdateStream)
	}

	for range len(agentMap) {
		select {
		case <-statusUpdateStream:
		case <-ctx.Done():
			return models.ActionResult{
				Status:     models.ActionResultStatusFailure,
				ActionName: "set-collector-sampling-ratio-action",
			}, ctx.Err()
		case <-time.After(s.timeout):
			return models.ActionResult{
				Status:     models.ActionResultStatusFailure,
				ActionName: "set-collector-sampling-ratio-action",
			}, fmt.Errorf("timeout")
		}
	}

	return models.ActionResult{
		Status:     models.ActionResultStatusSuccess,
		ActionName: "set-collector-sampling-ratio-action",
	}, nil
}

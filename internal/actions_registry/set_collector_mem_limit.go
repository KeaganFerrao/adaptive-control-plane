package actions_registry

import (
	"context"
	"fmt"
	"time"

	"github.com/KeaganFerrao/adaptive-control-plane/internal/models"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/opamp_server/data"
	"github.com/open-telemetry/opamp-go/protobufs"
)

type SetCollectorMemoryLimitAction struct {
	agents  *data.Agents
	timeout time.Duration
}

func NewSetCollectorMemoryLimitAction(agents *data.Agents) *SetCollectorMemoryLimitAction {
	return &SetCollectorMemoryLimitAction{
		agents:  agents,
		timeout: 10 * time.Second,
	}
}

func (s SetCollectorMemoryLimitAction) Execute(ctx context.Context, params models.ActionParams) (models.ActionResult, error) {
	limitMib, ok := params["limit_mib"]
	if !ok {
		return models.ActionResult{}, fmt.Errorf("missing parameter: limit_mib")
	}

	limitMibValue, ok := limitMib.(int)
	if !ok {
		return models.ActionResult{}, fmt.Errorf("limit_mib must be a int, got %T", limitMib)
	}

	agentMap := s.agents.AgentsById()
	statusUpdateStream := make(chan struct{}, len(agentMap))

	configStr := fmt.Sprintf("processors:\n    memory_limiter:\n        limit_mib: %d", limitMibValue)
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
				ActionName: "set-collector-memory-limit",
			}, ctx.Err()
		case <-time.After(s.timeout):
			return models.ActionResult{
				Status:     models.ActionResultStatusFailure,
				ActionName: "set-collector-memory-limit",
			}, fmt.Errorf("timeout")
		}
	}

	return models.ActionResult{
		Status:     models.ActionResultStatusSuccess,
		ActionName: "set-collector-memory-limit",
	}, nil
}

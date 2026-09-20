package actions_registry

import (
	"github.com/KeaganFerrao/adaptive-control-plane/internal/models"
	"github.com/KeaganFerrao/adaptive-control-plane/internal/opamp_server/data"
)

func GetRegistry() (models.ActionRegistry, error) {
	setCollectorSamplingAction := NewSetCollectorSamplingAction(data.AllAgents)
	setCollectorMemoryLimitAction := NewSetCollectorMemoryLimitAction(data.AllAgents)

	actionRegistry := models.ActionRegistry{
		models.ActionNameSetCollectorSampling:    setCollectorSamplingAction,
		models.ActionNameSetCollectorMemoryLimit: setCollectorMemoryLimitAction,
	}

	return actionRegistry, nil
}

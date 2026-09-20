package models

import (
	"context"

	"github.com/prometheus/common/model"
)

type ComponentMetrics map[Component]MetricSpec

type MetricSpec []string

type ComponentMetricSnapshots map[Component]MetricSnapshot

type MetricSnapshot map[string]model.SampleValue

type MetricCollector interface {
	Collect(ctx context.Context, metrics ComponentMetrics) (*ComponentMetricSnapshots, error)
}

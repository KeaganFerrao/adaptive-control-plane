package metric_collector

import (
	"context"
	"fmt"
	"time"

	"github.com/KeaganFerrao/adaptive-control-plane/internal/models"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PrometheusMetricCollector struct {
	prometheusClient *api.Client
}

func NewPrometheusMetricCollector(prometheusClient *api.Client) *PrometheusMetricCollector {
	return &PrometheusMetricCollector{
		prometheusClient: prometheusClient,
	}
}

// Collect TODO: Optimize the prometheus calls. Batch and group to avoid multiple http calls to prometheus
func (p *PrometheusMetricCollector) Collect(ctx context.Context, metrics models.ComponentMetrics) (*models.ComponentMetricSnapshots, error) {
	promAPI := v1.NewAPI(*p.prometheusClient)

	snapshots := make(models.ComponentMetricSnapshots, len(metrics))

	for component, metricSpecs := range metrics {
		snapshot := make(models.MetricSnapshot, len(metricSpecs))

		for _, metricName := range metricSpecs {
			query := fmt.Sprintf(
				`%s{collector_name=%q}`,
				metricName,
				component.Name,
			)

			result, warnings, err := promAPI.Query(
				ctx,
				query,
				time.Now(),
			)
			if err != nil {
				return nil, fmt.Errorf(
					"querying metric %q for component %q: %w",
					metricName,
					component.Name,
					err,
				)
			}

			// Prometheus warnings are not query errors.
			// Log them if you have a logger available.
			if len(warnings) > 0 {
				// p.logger.Warn(...)
			}

			vector, ok := result.(model.Vector)
			if !ok {
				return nil, fmt.Errorf(
					"expected vector result for metric %q, got %T",
					metricName,
					result,
				)
			}

			// No matching series for this component.
			if len(vector) == 0 {
				continue
			}

			// We expect one series because we're filtering by name.
			if len(vector) > 1 {
				return nil, fmt.Errorf(
					"metric %q returned %d series for component %q",
					metricName,
					len(vector),
					component.Name,
				)
			}

			snapshot[metricName] = vector[0].Value
		}

		snapshots[component] = snapshot
	}

	return &snapshots, nil
}

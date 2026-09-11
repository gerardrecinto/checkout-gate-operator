package controller

import (
	"context"
	"fmt"
	"time"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promodel "github.com/prometheus/common/model"

	"github.com/gerardrecinto/checkout-gate-operator/internal/gate"
)

// PrometheusMetricsProvider is the real MetricsProvider used in production,
// querying error rate, p99 latency, and CPU saturation for a Deployment's
// pods over PromQL. The queries assume the target exposes the same
// metric names this repo's own PromQL-adjacent tooling assumes elsewhere
// (rollout-sentinel), request counters split by status class, and a
// latency histogram.
type PrometheusMetricsProvider struct {
	api promv1.API
}

func NewPrometheusMetricsProvider(address string) (*PrometheusMetricsProvider, error) {
	client, err := promapi.NewClient(promapi.Config{Address: address})
	if err != nil {
		return nil, fmt.Errorf("creating Prometheus client: %w", err)
	}
	return &PrometheusMetricsProvider{api: promv1.NewAPI(client)}, nil
}

func (p *PrometheusMetricsProvider) FetchMetrics(ctx context.Context, namespace, deploymentName string) (gate.Metrics, error) {
	errorRate, err := p.scalarQuery(ctx, fmt.Sprintf(
		`100 * sum(rate(http_requests_total{namespace=%q,deployment=%q,status=~"5.."}[5m])) / sum(rate(http_requests_total{namespace=%q,deployment=%q}[5m]))`,
		namespace, deploymentName, namespace, deploymentName,
	))
	if err != nil {
		return gate.Metrics{}, fmt.Errorf("querying error rate: %w", err)
	}

	p99, err := p.scalarQuery(ctx, fmt.Sprintf(
		`1000 * histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{namespace=%q,deployment=%q}[5m])) by (le))`,
		namespace, deploymentName,
	))
	if err != nil {
		return gate.Metrics{}, fmt.Errorf("querying p99 latency: %w", err)
	}

	cpu, err := p.scalarQuery(ctx, fmt.Sprintf(
		`100 * avg(rate(container_cpu_usage_seconds_total{namespace=%q,pod=~%q}[5m]))`,
		namespace, deploymentName+"-.*",
	))
	if err != nil {
		return gate.Metrics{}, fmt.Errorf("querying cpu saturation: %w", err)
	}

	return gate.Metrics{
		ErrorRateMilliPercent: int32(errorRate * 1000),
		P99LatencyMs:          int32(p99),
		CpuSaturationPercent:  int32(cpu),
	}, nil
}

func (p *PrometheusMetricsProvider) scalarQuery(ctx context.Context, query string) (float64, error) {
	result, warnings, err := p.api.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}
	for _, w := range warnings {
		_ = w // surfaced via logging in a real deployment, kept quiet here to avoid a logger dependency in this file
	}

	vector, ok := result.(promodel.Vector)
	if !ok || len(vector) == 0 {
		return 0, nil
	}
	return float64(vector[0].Value), nil
}

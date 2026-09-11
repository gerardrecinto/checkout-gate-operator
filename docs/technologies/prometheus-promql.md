# Prometheus and PromQL: the metrics behind a gate's verdict

**TL;DR: Prometheus scrapes numeric time series over HTTP, stores them,
and answers questions about them in PromQL, its own query language. This
repo's `PrometheusMetricsProvider` turns three PromQL queries (error
rate, p99 latency, CPU saturation) into the plain `gate.Metrics` struct
the evaluator actually reasons about.**

## Pull, not push, and why that shapes everything

Prometheus scrapes: it polls a `/metrics` HTTP endpoint on a schedule
and pulls whatever numbers are there, rather than services pushing
metrics to it. That single design choice is why every Prometheus
deployment looks the same shape: your application (or a sidecar) just
exposes counters and histograms on an endpoint, Prometheus does the
collecting, storing, and querying entirely on its own side.

```text
retail-checkout-api pods
   expose /metrics: http_requests_total{status="500"} 42, ...
        |
        | Prometheus scrapes this endpoint every N seconds
        v
   Prometheus's time-series database
        |
        | PromQL query, evaluated at query time
        v
   PrometheusMetricsProvider.FetchMetrics() in this repo
        |
        v
   gate.Metrics{ErrorRateMilliPercent, P99LatencyMs, CpuSaturationPercent}
        |
        v
   gate.Evaluate() never sees PromQL at all, just plain numbers
```

That last line matters as a design decision, not just a fact: the
evaluator in `internal/gate/evaluator.go` has zero PromQL, zero HTTP
client, zero knowledge that Prometheus exists. All of that complexity
is contained in `internal/controller/prometheus_metrics.go`, behind the
`MetricsProvider` interface, which is exactly why the evaluator is
100% unit-test-covered with no mocking required, there's nothing to
mock, it's a pure function over plain structs.

## Reading this repo's actual PromQL, piece by piece

```promql
100 * sum(rate(http_requests_total{namespace="default",deployment="retail-checkout-api",status=~"5.."}[5m]))
    / sum(rate(http_requests_total{namespace="default",deployment="retail-checkout-api"}[5m]))
```

**Mnemonic: RSAO.** Rate first (never raw counters), Sum across pods,
Apply the time window, Over the total to get a ratio.

- `http_requests_total{...}` selects the raw counter, filtered by
  label matchers, `status=~"5.."` is a regex matcher meaning "any
  status code starting with 5", the 5xx class.
- `rate(...[5m])` is the step almost everyone gets wrong the first
  time: a counter only ever goes up, so you can't chart it directly,
  `rate()` computes the per-second average rate of increase over the
  trailing 5-minute window, which is what actually behaves like "how
  fast is this happening right now."
- `sum(...)` collapses the per-pod time series into one number, error
  rate needs to be a fleet-wide ratio, not five separate numbers per
  pod.
- Dividing the 5xx rate by the total request rate turns two absolute
  rates into a percentage, and multiplying by 100 turns the fraction
  into a percent (the controller then multiplies by another 1000 to
  get millipercent, matching the CRD's integer schema, see
  [crds-and-codegen.md](crds-and-codegen.md)).

```promql
1000 * histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{...}[5m])) by (le))
```

`histogram_quantile` is the PromQL-specific idea worth understanding on
its own: Prometheus histograms don't store individual request
durations, they store counts of requests falling into pre-defined
buckets (`le`, "less than or equal to", is the bucket's upper bound
label). `histogram_quantile(0.99, ...)` interpolates across those
bucket boundaries to estimate the 99th percentile, an approximation,
bounded by how the buckets were defined, not an exact per-request
value, which is a real, worth-knowing limitation of histogram-based
percentiles versus a system that tracks every raw sample.

## Why the interface exists at all

```go
type MetricsProvider interface {
    FetchMetrics(ctx context.Context, namespace, deploymentName string) (gate.Metrics, error)
}
```

`checkoutgate_controller_test.go` never touches a real Prometheus, it
injects a `fakeMetricsProvider` returning fixed values, because the
thing under test there is the reconciler's logic (what does it do with
a Breach verdict, how does it handle a missing target), not whether
PromQL syntax is correct. Testing those two concerns separately, one
against a fake at the interface boundary, one (implicitly, by code
review and the queries matching Prometheus's real query language) at
the query level, is what keeps each test focused on one thing.

## Go deeper

- [prometheus.io/docs/prometheus/latest/querying/basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
  for PromQL fundamentals beyond the three query shapes used here.
- [prometheus.io/docs/practices/histograms](https://prometheus.io/docs/practices/histograms/)
  covers the histogram-versus-summary trade-off and bucket boundary
  selection in more depth than this doc does.

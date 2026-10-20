package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// promServer answers every instant query with one vector sample chosen by a
// substring of the PromQL, so each of the three queries gets its own value.
func promServer(t *testing.T, values map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.Form.Get("query")
		for needle, v := range values {
			if strings.Contains(q, needle) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1700000000,%q]}]}}`, v)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[]}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPrometheusMetricsProvider_FetchMetrics(t *testing.T) {
	srv := promServer(t, map[string]string{
		`status=~"5.."`:       "2.5",
		"histogram_quantile":  "480.9",
		"container_cpu_usage": "37.2",
	})
	p, err := NewPrometheusMetricsProvider(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	m, err := p.FetchMetrics(context.Background(), "shop", "checkout-api")
	if err != nil {
		t.Fatal(err)
	}
	if m.ErrorRateMilliPercent != 2500 || m.P99LatencyMs != 480 || m.CpuSaturationPercent != 37 {
		t.Fatalf("metrics = %+v", m)
	}
}

func TestPrometheusMetricsProvider_NoSamplesReadAsZero(t *testing.T) {
	srv := promServer(t, nil)
	p, _ := NewPrometheusMetricsProvider(srv.URL)
	m, err := p.FetchMetrics(context.Background(), "shop", "checkout-api")
	if err != nil {
		t.Fatal(err)
	}
	if m.ErrorRateMilliPercent != 0 || m.P99LatencyMs != 0 || m.CpuSaturationPercent != 0 {
		t.Fatalf("metrics = %+v, want zeros", m)
	}
}

// Prometheus returns NaN for 0/0 (a service with no traffic) and for a
// histogram_quantile over empty buckets. int32(NaN) is implementation
// defined in Go and is a large negative number on amd64, which would read as
// an impossibly healthy service.
func TestPrometheusMetricsProvider_NaNAndInfReadAsZero(t *testing.T) {
	for _, bad := range []string{"NaN", "+Inf", "-Inf"} {
		t.Run(bad, func(t *testing.T) {
			srv := promServer(t, map[string]string{
				`status=~"5.."`:       bad,
				"histogram_quantile":  bad,
				"container_cpu_usage": bad,
			})
			p, _ := NewPrometheusMetricsProvider(srv.URL)
			m, err := p.FetchMetrics(context.Background(), "shop", "checkout-api")
			if err != nil {
				t.Fatal(err)
			}
			if m.ErrorRateMilliPercent != 0 || m.P99LatencyMs != 0 || m.CpuSaturationPercent != 0 {
				t.Fatalf("metrics = %+v, want zeros", m)
			}
		})
	}
}

func TestPrometheusMetricsProvider_QueryErrorIsWrapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	p, _ := NewPrometheusMetricsProvider(srv.URL)
	_, err := p.FetchMetrics(context.Background(), "shop", "checkout-api")
	if err == nil || !strings.Contains(err.Error(), "querying error rate") {
		t.Fatalf("err = %v, want a wrapped error-rate query failure", err)
	}
}

func TestNewPrometheusMetricsProvider_BadAddress(t *testing.T) {
	if _, err := NewPrometheusMetricsProvider("://nope"); err == nil {
		t.Fatal("expected an error for an unparseable address")
	}
}

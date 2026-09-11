package gate

import (
	"testing"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
)

func defaultSpec() checkoutv1alpha1.CheckoutGateSpec {
	return checkoutv1alpha1.CheckoutGateSpec{
		TargetDeployment:         "retail-checkout-api",
		MaxErrorRateMilliPercent: 500, // 0.5%
		MaxP99LatencyMs:          450,
		MaxCpuSaturationPercent:  80,
	}
}

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name    string
		metrics Metrics
		want    checkoutv1alpha1.GateVerdict
	}{
		{
			name: "healthy",
			metrics: Metrics{
				ErrorRateMilliPercent: 120,
				P99LatencyMs:          200,
				CpuSaturationPercent:  40,
				ReadyReplicas:         3,
				DesiredReplicas:       3,
			},
			want: checkoutv1alpha1.VerdictPass,
		},
		{
			name: "error rate breach",
			metrics: Metrics{
				ErrorRateMilliPercent: 2400, // 2.4%
				P99LatencyMs:          200,
				CpuSaturationPercent:  40,
				ReadyReplicas:         3,
				DesiredReplicas:       3,
			},
			want: checkoutv1alpha1.VerdictBreach,
		},
		{
			name: "latency breach",
			metrics: Metrics{
				ErrorRateMilliPercent: 100,
				P99LatencyMs:          18094,
				CpuSaturationPercent:  40,
				ReadyReplicas:         3,
				DesiredReplicas:       3,
			},
			want: checkoutv1alpha1.VerdictBreach,
		},
		{
			name: "cpu saturation warns, does not breach",
			metrics: Metrics{
				ErrorRateMilliPercent: 100,
				P99LatencyMs:          200,
				CpuSaturationPercent:  95,
				ReadyReplicas:         3,
				DesiredReplicas:       3,
			},
			want: checkoutv1alpha1.VerdictWarn,
		},
		{
			name: "zero ready replicas breaches regardless of metrics",
			metrics: Metrics{
				ErrorRateMilliPercent: 0,
				P99LatencyMs:          0,
				CpuSaturationPercent:  0,
				ReadyReplicas:         0,
				DesiredReplicas:       3,
			},
			want: checkoutv1alpha1.VerdictBreach,
		},
		{
			name: "zero desired replicas is not itself a breach",
			metrics: Metrics{
				ErrorRateMilliPercent: 0,
				P99LatencyMs:          0,
				CpuSaturationPercent:  0,
				ReadyReplicas:         0,
				DesiredReplicas:       0,
			},
			want: checkoutv1alpha1.VerdictPass,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Evaluate(defaultSpec(), tc.metrics)
			if result.Verdict != tc.want {
				t.Fatalf("Evaluate() verdict = %s, want %s (message: %s)", result.Verdict, tc.want, result.Message)
			}
			if result.Message == "" {
				t.Fatalf("Evaluate() returned an empty message for verdict %s", result.Verdict)
			}
		})
	}
}

func TestEvaluate_PriorityOrder(t *testing.T) {
	// Error rate breach should win even when latency and CPU are also over threshold,
	// zero-ready-replicas should win over everything, ordering matters and is worth locking down explicitly.
	spec := defaultSpec()
	m := Metrics{
		ErrorRateMilliPercent: 5000,
		P99LatencyMs:          99999,
		CpuSaturationPercent:  99,
		ReadyReplicas:         0,
		DesiredReplicas:       3,
	}
	result := Evaluate(spec, m)
	if result.Verdict != checkoutv1alpha1.VerdictBreach {
		t.Fatalf("expected Breach, got %s", result.Verdict)
	}
	if got := result.Message; got == "" {
		t.Fatal("expected a non-empty message explaining the breach")
	}
}

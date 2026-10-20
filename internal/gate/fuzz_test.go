package gate

import (
	"testing"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
)

// FuzzEvaluate checks properties that must hold for any input: no panic,
// the verdict is one of Pass/Warn/Breach, the result is deterministic, and
// a hard-threshold violation can never come back as Pass or Warn.
func FuzzEvaluate(f *testing.F) {
	f.Add(int32(500), int32(450), int32(80), int32(100), int32(200), int32(40), int32(3), int32(3))
	f.Add(int32(0), int32(0), int32(0), int32(0), int32(0), int32(0), int32(0), int32(0))
	f.Add(int32(-1), int32(-1), int32(-1), int32(1<<30), int32(-1<<30), int32(101), int32(5), int32(0))

	f.Fuzz(func(t *testing.T, maxErr, maxP99, maxCPU, errRate, p99, cpu, desired, ready int32) {
		spec := checkoutv1alpha1.CheckoutGateSpec{
			MaxErrorRateMilliPercent: maxErr,
			MaxP99LatencyMs:          maxP99,
			MaxCpuSaturationPercent:  maxCPU,
		}
		m := Metrics{
			ErrorRateMilliPercent: errRate,
			P99LatencyMs:          p99,
			CpuSaturationPercent:  cpu,
			DesiredReplicas:       desired,
			ReadyReplicas:         ready,
		}

		got := Evaluate(spec, m)
		if again := Evaluate(spec, m); again != got {
			t.Fatalf("not deterministic: %+v vs %+v", got, again)
		}
		switch got.Verdict {
		case checkoutv1alpha1.VerdictPass, checkoutv1alpha1.VerdictWarn, checkoutv1alpha1.VerdictBreach:
		default:
			t.Fatalf("unexpected verdict %q", got.Verdict)
		}
		if got.Message == "" {
			t.Fatal("empty message")
		}

		hard := (desired > 0 && ready == 0) || errRate > maxErr || p99 > maxP99
		if hard && got.Verdict != checkoutv1alpha1.VerdictBreach {
			t.Fatalf("hard violation returned %s for spec=%+v m=%+v", got.Verdict, spec, m)
		}
		if !hard && cpu > maxCPU && got.Verdict != checkoutv1alpha1.VerdictWarn {
			t.Fatalf("cpu-only violation returned %s", got.Verdict)
		}
		if !hard && cpu <= maxCPU && got.Verdict != checkoutv1alpha1.VerdictPass {
			t.Fatalf("clean input returned %s", got.Verdict)
		}
	})
}

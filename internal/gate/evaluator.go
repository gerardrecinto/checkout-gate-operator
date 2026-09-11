// Package gate holds the pure evaluation logic the controller calls on
// every reconcile. Kept free of any Kubernetes client or API server
// dependency on purpose: a verdict decision should be testable with plain
// structs in and a plain struct out, the I/O (fetching metrics, fetching
// Deployment status) lives in the controller, not here.
package gate

import (
	"fmt"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
)

// Metrics is a point-in-time snapshot of what the controller observed for
// the target Deployment's pods. Deliberately a plain struct, not a
// Prometheus client response, so the evaluator has no knowledge of where
// the numbers came from.
type Metrics struct {
	ErrorRateMilliPercent int32
	P99LatencyMs          int32
	CpuSaturationPercent  int32
	ReadyReplicas         int32
	DesiredReplicas       int32
}

// Result is what the controller writes into CheckoutGate.Status.
type Result struct {
	Verdict checkoutv1alpha1.GateVerdict
	Message string
}

// Evaluate applies the gate's thresholds to a metrics snapshot. Pure
// function: same inputs always produce the same Result, no side effects,
// no network calls, which is what makes it trivial to unit test.
func Evaluate(spec checkoutv1alpha1.CheckoutGateSpec, m Metrics) Result {
	if m.DesiredReplicas > 0 && m.ReadyReplicas == 0 {
		return Result{
			Verdict: checkoutv1alpha1.VerdictBreach,
			Message: fmt.Sprintf("target has %d desired replicas but 0 ready", m.DesiredReplicas),
		}
	}

	if m.ErrorRateMilliPercent > spec.MaxErrorRateMilliPercent {
		return Result{
			Verdict: checkoutv1alpha1.VerdictBreach,
			Message: fmt.Sprintf("error rate %.3f%% exceeds max %.3f%%",
				milliPercentToPercent(m.ErrorRateMilliPercent), milliPercentToPercent(spec.MaxErrorRateMilliPercent)),
		}
	}

	if m.P99LatencyMs > spec.MaxP99LatencyMs {
		return Result{
			Verdict: checkoutv1alpha1.VerdictBreach,
			Message: fmt.Sprintf("p99 latency %dms exceeds max %dms", m.P99LatencyMs, spec.MaxP99LatencyMs),
		}
	}

	if m.CpuSaturationPercent > spec.MaxCpuSaturationPercent {
		return Result{
			Verdict: checkoutv1alpha1.VerdictWarn,
			Message: fmt.Sprintf("cpu saturation %d%% exceeds max %d%%, holding rather than breaching on a resource signal alone",
				m.CpuSaturationPercent, spec.MaxCpuSaturationPercent),
		}
	}

	return Result{
		Verdict: checkoutv1alpha1.VerdictPass,
		Message: "all thresholds satisfied",
	}
}

func milliPercentToPercent(mp int32) float64 {
	return float64(mp) / 1000.0
}

package controller

import (
	"context"
	"errors"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
	"github.com/gerardrecinto/checkout-gate-operator/internal/gate"
)

var errBoom = errors.New("boom")

func TestReconcileTable(t *testing.T) {
	spec := checkoutv1alpha1.CheckoutGateSpec{
		TargetDeployment:         "api",
		MaxErrorRateMilliPercent: 500,
		MaxP99LatencyMs:          450,
		MaxCpuSaturationPercent:  80,
	}
	healthy := gate.Metrics{ErrorRateMilliPercent: 100, P99LatencyMs: 200, CpuSaturationPercent: 40}

	tests := []struct {
		name        string
		dep         *appsv1.Deployment
		metrics     gate.Metrics
		metricsErr  error
		statusErr   error
		wantVerdict checkoutv1alpha1.GateVerdict
		wantErr     string // substring; empty means no error
		wantUpdated bool   // whether status should have been written
	}{
		{
			name:        "healthy",
			dep:         dep("api", 3, 3),
			metrics:     healthy,
			wantVerdict: checkoutv1alpha1.VerdictPass,
			wantUpdated: true,
		},
		{
			name:        "cpu over limit warns",
			dep:         dep("api", 3, 3),
			metrics:     gate.Metrics{ErrorRateMilliPercent: 100, P99LatencyMs: 200, CpuSaturationPercent: 95},
			wantVerdict: checkoutv1alpha1.VerdictWarn,
			wantUpdated: true,
		},
		{
			name:        "latency over limit breaches",
			dep:         dep("api", 3, 3),
			metrics:     gate.Metrics{P99LatencyMs: 900},
			wantVerdict: checkoutv1alpha1.VerdictBreach,
			wantUpdated: true,
		},
		{
			name:        "no ready replicas breaches",
			dep:         dep("api", 3, 0),
			metrics:     healthy,
			wantVerdict: checkoutv1alpha1.VerdictBreach,
			wantUpdated: true,
		},
		{
			name:        "missing target is unknown",
			dep:         nil,
			wantVerdict: checkoutv1alpha1.VerdictUnknown,
			wantUpdated: true,
		},
		{
			name:       "metrics failure is returned and status is left alone",
			dep:        dep("api", 3, 3),
			metricsErr: errBoom,
			wantErr:    "fetching metrics",
		},
		{
			name:      "status update failure is wrapped",
			dep:       dep("api", 3, 3),
			metrics:   healthy,
			statusErr: errBoom,
			wantErr:   "updating CheckoutGate status",
		},
		{
			name:      "status update failure on missing target is wrapped",
			dep:       nil,
			statusErr: errBoom,
			wantErr:   "updating CheckoutGate status",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cg := &checkoutv1alpha1.CheckoutGate{
				ObjectMeta: metav1.ObjectMeta{Name: "gate", Namespace: "default", Generation: 4},
				Spec:       spec,
			}
			objs := []client.Object{cg}
			if tc.dep != nil {
				objs = append(objs, tc.dep)
			}
			c := fake.NewClientBuilder().
				WithScheme(newScheme(t)).
				WithObjects(objs...).
				WithStatusSubresource(cg).
				WithInterceptorFuncs(interceptor.Funcs{
					SubResourceUpdate: func(ctx context.Context, cl client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
						if tc.statusErr != nil {
							return tc.statusErr
						}
						return cl.SubResource(sub).Update(ctx, obj, opts...)
					},
				}).
				Build()

			r := &CheckoutGateReconciler{Client: c, Metrics: &fakeMetricsProvider{metrics: tc.metrics, err: tc.metricsErr}}
			_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "gate", Namespace: "default"}})

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				if !errors.Is(err, errBoom) {
					t.Fatalf("error %v does not wrap the cause", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var got checkoutv1alpha1.CheckoutGate
			if err := c.Get(context.Background(), types.NamespacedName{Name: "gate", Namespace: "default"}, &got); err != nil {
				t.Fatal(err)
			}
			if !tc.wantUpdated {
				if got.Status.Verdict != "" {
					t.Fatalf("status written despite failure: %+v", got.Status)
				}
				return
			}
			if got.Status.Verdict != tc.wantVerdict {
				t.Fatalf("verdict = %s, want %s (%s)", got.Status.Verdict, tc.wantVerdict, got.Status.Message)
			}
			if got.Status.ObservedGeneration != 4 {
				t.Fatalf("observedGeneration = %d, want 4", got.Status.ObservedGeneration)
			}
		})
	}
}

func dep(name string, desired, ready int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: replicaCount(desired)},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: ready},
	}
}

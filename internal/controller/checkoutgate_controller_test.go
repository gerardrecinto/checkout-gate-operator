package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
	"github.com/gerardrecinto/checkout-gate-operator/internal/gate"
)

// fakeMetricsProvider lets each test control exactly what the reconciler
// sees, without a real Prometheus anywhere.
type fakeMetricsProvider struct {
	metrics gate.Metrics
	err     error
}

func (f *fakeMetricsProvider) FetchMetrics(_ context.Context, _, _ string) (gate.Metrics, error) {
	return f.metrics, f.err
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := checkoutv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("adding checkoutv1alpha1 to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(s); err != nil {
		t.Fatalf("adding appsv1 to scheme: %v", err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("adding corev1 to scheme: %v", err)
	}
	return s
}

func replicaCount(n int32) *int32 { return &n }

func TestReconcile_HealthyTargetProducesPass(t *testing.T) {
	s := newScheme(t)

	cg := &checkoutv1alpha1.CheckoutGate{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout-gate", Namespace: "default"},
		Spec: checkoutv1alpha1.CheckoutGateSpec{
			TargetDeployment:         "retail-checkout-api",
			MaxErrorRateMilliPercent: 500,
			MaxP99LatencyMs:          450,
			MaxCpuSaturationPercent:  80,
		},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "retail-checkout-api", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: replicaCount(3)},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 3},
	}

	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(cg, dep).
		WithStatusSubresource(cg).
		Build()

	r := &CheckoutGateReconciler{
		Client: c,
		Metrics: &fakeMetricsProvider{metrics: gate.Metrics{
			ErrorRateMilliPercent: 100,
			P99LatencyMs:          200,
			CpuSaturationPercent:  40,
		}},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "checkout-gate", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var got checkoutv1alpha1.CheckoutGate
	if err := c.Get(context.Background(), types.NamespacedName{Name: "checkout-gate", Namespace: "default"}, &got); err != nil {
		t.Fatalf("fetching CheckoutGate after reconcile: %v", err)
	}
	if got.Status.Verdict != checkoutv1alpha1.VerdictPass {
		t.Fatalf("Status.Verdict = %s, want Pass (message: %s)", got.Status.Verdict, got.Status.Message)
	}
	if got.Status.ReadyReplicas != 3 {
		t.Fatalf("Status.ReadyReplicas = %d, want 3", got.Status.ReadyReplicas)
	}
}

func TestReconcile_BreachingTargetProducesBreach(t *testing.T) {
	s := newScheme(t)

	cg := &checkoutv1alpha1.CheckoutGate{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout-gate", Namespace: "default"},
		Spec: checkoutv1alpha1.CheckoutGateSpec{
			TargetDeployment:         "retail-checkout-api",
			MaxErrorRateMilliPercent: 500,
			MaxP99LatencyMs:          450,
			MaxCpuSaturationPercent:  80,
		},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "retail-checkout-api", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: replicaCount(3)},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 3},
	}

	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(cg, dep).
		WithStatusSubresource(cg).
		Build()

	r := &CheckoutGateReconciler{
		Client: c,
		Metrics: &fakeMetricsProvider{metrics: gate.Metrics{
			ErrorRateMilliPercent: 100,
			P99LatencyMs:          18094,
			CpuSaturationPercent:  40,
		}},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "checkout-gate", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var got checkoutv1alpha1.CheckoutGate
	if err := c.Get(context.Background(), types.NamespacedName{Name: "checkout-gate", Namespace: "default"}, &got); err != nil {
		t.Fatalf("fetching CheckoutGate after reconcile: %v", err)
	}
	if got.Status.Verdict != checkoutv1alpha1.VerdictBreach {
		t.Fatalf("Status.Verdict = %s, want Breach", got.Status.Verdict)
	}
}

func TestReconcile_MissingTargetDeploymentProducesUnknown(t *testing.T) {
	s := newScheme(t)

	cg := &checkoutv1alpha1.CheckoutGate{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout-gate", Namespace: "default"},
		Spec: checkoutv1alpha1.CheckoutGateSpec{
			TargetDeployment: "does-not-exist",
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(cg).
		WithStatusSubresource(cg).
		Build()

	r := &CheckoutGateReconciler{
		Client:  c,
		Metrics: &fakeMetricsProvider{},
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "checkout-gate", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var got checkoutv1alpha1.CheckoutGate
	if err := c.Get(context.Background(), types.NamespacedName{Name: "checkout-gate", Namespace: "default"}, &got); err != nil {
		t.Fatalf("fetching CheckoutGate after reconcile: %v", err)
	}
	if got.Status.Verdict != checkoutv1alpha1.VerdictUnknown {
		t.Fatalf("Status.Verdict = %s, want Unknown", got.Status.Verdict)
	}
}

func TestReconcile_MissingCheckoutGateIsANoOp(t *testing.T) {
	s := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).Build()

	r := &CheckoutGateReconciler{Client: c, Metrics: &fakeMetricsProvider{}}

	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "does-not-exist", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() on a deleted CheckoutGate should not error, got: %v", err)
	}
	if res.Requeue {
		t.Fatalf("Reconcile() on a deleted CheckoutGate should not request a requeue")
	}
}

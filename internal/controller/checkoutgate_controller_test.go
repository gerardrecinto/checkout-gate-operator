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
	"github.com/gerardrecinto/checkout-gate-operator/internal/notify"
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

// fakeNotifier records every call it receives, so tests can assert on
// exactly how many times, and with what payload, the reconciler notified.
type fakeNotifier struct {
	calls []notify.VerdictTransition
}

func (f *fakeNotifier) NotifyVerdictTransition(_ context.Context, t notify.VerdictTransition) error {
	f.calls = append(f.calls, t)
	return nil
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

func TestReconcile_NotifiesOnceOnVerdictTransition(t *testing.T) {
	s := newScheme(t)

	cg := &checkoutv1alpha1.CheckoutGate{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout-gate", Namespace: "default"},
		Spec: checkoutv1alpha1.CheckoutGateSpec{
			TargetDeployment:         "retail-checkout-api",
			MaxErrorRateMilliPercent: 500,
			MaxP99LatencyMs:          450,
			MaxCpuSaturationPercent:  80,
		},
		// A gate that was already Pass from a previous reconcile, so this
		// run's Breach is a real transition, not an initial verdict.
		Status: checkoutv1alpha1.CheckoutGateStatus{Verdict: checkoutv1alpha1.VerdictPass},
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

	notifier := &fakeNotifier{}
	r := &CheckoutGateReconciler{
		Client: c,
		Metrics: &fakeMetricsProvider{metrics: gate.Metrics{
			ErrorRateMilliPercent: 100,
			P99LatencyMs:          18094,
			CpuSaturationPercent:  40,
		}},
		Notifier: notifier,
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "checkout-gate", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("notifier called %d times, want exactly 1 (calls: %+v)", len(notifier.calls), notifier.calls)
	}

	got := notifier.calls[0]
	if got.OldVerdict != string(checkoutv1alpha1.VerdictPass) {
		t.Fatalf("OldVerdict = %q, want %q", got.OldVerdict, checkoutv1alpha1.VerdictPass)
	}
	if got.NewVerdict != string(checkoutv1alpha1.VerdictBreach) {
		t.Fatalf("NewVerdict = %q, want %q", got.NewVerdict, checkoutv1alpha1.VerdictBreach)
	}
	if got.Namespace != "default" || got.Name != "checkout-gate" {
		t.Fatalf("transition identity = %s/%s, want default/checkout-gate", got.Namespace, got.Name)
	}
	if got.TargetDeployment != "retail-checkout-api" {
		t.Fatalf("TargetDeployment = %q, want %q", got.TargetDeployment, "retail-checkout-api")
	}
	wantSubject := "checkoutgate.default.checkout-gate.verdict"
	if got.Subject() != wantSubject {
		t.Fatalf("Subject() = %q, want %q", got.Subject(), wantSubject)
	}
}

func TestReconcile_DoesNotNotifyWhenVerdictUnchangedAcrossTwoReconciles(t *testing.T) {
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

	notifier := &fakeNotifier{}
	r := &CheckoutGateReconciler{
		Client: c,
		Metrics: &fakeMetricsProvider{metrics: gate.Metrics{
			ErrorRateMilliPercent: 100,
			P99LatencyMs:          200,
			CpuSaturationPercent:  40,
		}},
		Notifier: notifier,
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "checkout-gate", Namespace: "default"}}

	// First reconcile: Status.Verdict starts empty (never evaluated), so
	// landing on Pass is an initial verdict, not a transition.
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	// Second reconcile: same healthy metrics, so Pass -> Pass, no change.
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("notifier called %d times across two unchanged reconciles, want 0 (calls: %+v)", len(notifier.calls), notifier.calls)
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

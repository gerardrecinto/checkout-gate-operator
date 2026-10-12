package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
	"github.com/gerardrecinto/checkout-gate-operator/internal/gate"
)

// Runs against a real kube-apiserver and etcd, so it covers what the fake
// client cannot: CRD schema validation and the real status subresource.
// Skipped unless KUBEBUILDER_ASSETS points at the envtest binaries
// (setup-envtest use -p path).
func startEnvtest(t *testing.T) client.Client {
	t.Helper()
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("KUBEBUILDER_ASSETS not set, skipping envtest")
	}
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("starting envtest: %v", err)
	}
	t.Cleanup(func() { _ = env.Stop() })
	c, err := client.New(cfg, client.Options{Scheme: newScheme(t)})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEnvtestRejectsOutOfRangeSpec(t *testing.T) {
	c := startEnvtest(t)
	bad := &checkoutv1alpha1.CheckoutGate{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "default"},
		Spec:       checkoutv1alpha1.CheckoutGateSpec{TargetDeployment: "api", MaxCpuSaturationPercent: 101},
	}
	err := c.Create(context.Background(), bad)
	if !apierrors.IsInvalid(err) {
		t.Fatalf("Create() error = %v, want an Invalid error from CRD validation", err)
	}
}

func TestEnvtestReconcileWritesStatus(t *testing.T) {
	c := startEnvtest(t)
	ctx := context.Background()

	replicas := int32(1)
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
		},
	}
	d.Spec.Template.ObjectMeta.Labels = map[string]string{"app": "api"}
	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, containerNamed("api"))
	if err := c.Create(ctx, d); err != nil {
		t.Fatal(err)
	}
	cg := &checkoutv1alpha1.CheckoutGate{
		ObjectMeta: metav1.ObjectMeta{Name: "gate", Namespace: "default"},
		Spec: checkoutv1alpha1.CheckoutGateSpec{
			TargetDeployment: "api", MaxErrorRateMilliPercent: 500, MaxP99LatencyMs: 450, MaxCpuSaturationPercent: 80,
		},
	}
	if err := c.Create(ctx, cg); err != nil {
		t.Fatal(err)
	}

	r := &CheckoutGateReconciler{Client: c, Metrics: &fakeMetricsProvider{metrics: gate.Metrics{}}}
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "gate", Namespace: "default"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var got checkoutv1alpha1.CheckoutGate
	if err := c.Get(ctx, types.NamespacedName{Name: "gate", Namespace: "default"}, &got); err != nil {
		t.Fatal(err)
	}
	// No pods run in envtest, so the Deployment has 0 ready replicas of 1.
	if got.Status.Verdict != checkoutv1alpha1.VerdictBreach {
		t.Fatalf("verdict = %q, want Breach (%s)", got.Status.Verdict, got.Status.Message)
	}
}

func containerNamed(name string) corev1.Container {
	return corev1.Container{Name: name, Image: "registry.invalid/" + name + ":1"}
}

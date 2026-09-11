package webhook

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := checkoutv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("adding checkoutv1alpha1 to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(s); err != nil {
		t.Fatalf("adding appsv1 to scheme: %v", err)
	}
	return s
}

func TestValidateCreate_RejectsZeroLatencyThreshold(t *testing.T) {
	s := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).Build()
	v := &CheckoutGateValidator{Client: c}

	cg := &checkoutv1alpha1.CheckoutGate{
		ObjectMeta: metav1.ObjectMeta{Name: "gate", Namespace: "default"},
		Spec: checkoutv1alpha1.CheckoutGateSpec{
			TargetDeployment: "retail-checkout-api",
			MaxP99LatencyMs:  0,
		},
	}

	_, err := v.ValidateCreate(context.Background(), cg)
	if err == nil {
		t.Fatal("expected ValidateCreate to reject a zero MaxP99LatencyMs, got nil error")
	}
}

func TestValidateCreate_WarnsWhenTargetMissingButDoesNotReject(t *testing.T) {
	s := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).Build() // no Deployment seeded
	v := &CheckoutGateValidator{Client: c}

	cg := &checkoutv1alpha1.CheckoutGate{
		ObjectMeta: metav1.ObjectMeta{Name: "gate", Namespace: "default"},
		Spec: checkoutv1alpha1.CheckoutGateSpec{
			TargetDeployment: "not-created-yet",
			MaxP99LatencyMs:  450,
		},
	}

	warnings, err := v.ValidateCreate(context.Background(), cg)
	if err != nil {
		t.Fatalf("expected no error for a not-yet-existing target, got: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected exactly one warning about the missing target, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateCreate_PassesForAHealthySpec(t *testing.T) {
	s := newScheme(t)
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "retail-checkout-api", Namespace: "default"}}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(dep).Build()
	v := &CheckoutGateValidator{Client: c}

	cg := &checkoutv1alpha1.CheckoutGate{
		ObjectMeta: metav1.ObjectMeta{Name: "gate", Namespace: "default"},
		Spec: checkoutv1alpha1.CheckoutGateSpec{
			TargetDeployment: "retail-checkout-api",
			MaxP99LatencyMs:  450,
		},
	}

	warnings, err := v.ValidateCreate(context.Background(), cg)
	if err != nil {
		t.Fatalf("expected no error for a valid spec with an existing target, got: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got: %v", warnings)
	}
}

func TestValidateCreate_RejectsWrongType(t *testing.T) {
	s := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).Build()
	v := &CheckoutGateValidator{Client: c}

	_, err := v.ValidateCreate(context.Background(), &appsv1.Deployment{})
	if err == nil {
		t.Fatal("expected an error when handed the wrong runtime.Object type")
	}
}

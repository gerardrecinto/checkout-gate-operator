package apiserver

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	checkoutv1alpha1 "github.com/gerardrecinto/checkout-gate-operator/api/v1alpha1"
)

func TestHandleListGates(t *testing.T) {
	s := runtime.NewScheme()
	if err := checkoutv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("adding scheme: %v", err)
	}

	cg := &checkoutv1alpha1.CheckoutGate{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout-gate", Namespace: "default"},
		Spec:       checkoutv1alpha1.CheckoutGateSpec{TargetDeployment: "retail-checkout-api"},
		Status: checkoutv1alpha1.CheckoutGateStatus{
			Verdict:       checkoutv1alpha1.VerdictPass,
			Message:       "all thresholds satisfied",
			ReadyReplicas: 3,
		},
	}

	c := fake.NewClientBuilder().WithScheme(s).WithObjects(cg).Build()
	srv := &Server{Reader: c}

	req := httptest.NewRequest("GET", "/api/gates", nil)
	rec := httptest.NewRecorder()

	srv.handleListGates(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got []GateSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response body: %v (body: %s)", err, rec.Body.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 gate summary, got %d", len(got))
	}
	if got[0].Verdict != "Pass" {
		t.Fatalf("expected verdict Pass, got %s", got[0].Verdict)
	}
	if got[0].TargetDeployment != "retail-checkout-api" {
		t.Fatalf("expected targetDeployment retail-checkout-api, got %s", got[0].TargetDeployment)
	}
}

func TestHandleListGates_EmptyListReturnsEmptyArrayNotNull(t *testing.T) {
	s := runtime.NewScheme()
	if err := checkoutv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("adding scheme: %v", err)
	}
	c := fake.NewClientBuilder().WithScheme(s).Build()
	srv := &Server{Reader: c}

	req := httptest.NewRequest("GET", "/api/gates", nil)
	rec := httptest.NewRecorder()
	srv.handleListGates(rec, req)

	if got := rec.Body.String(); got != "[]\n" {
		t.Fatalf("expected an empty JSON array for no gates (not null), got: %q", got)
	}
}

package apiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func TestHandleSummary(t *testing.T) {
	s := runtime.NewScheme()
	if err := checkoutv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("adding scheme: %v", err)
	}

	gates := []client.Object{
		&checkoutv1alpha1.CheckoutGate{
			ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "default"},
			Status:     checkoutv1alpha1.CheckoutGateStatus{Verdict: checkoutv1alpha1.VerdictPass},
		},
		&checkoutv1alpha1.CheckoutGate{
			ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "default"},
			Status:     checkoutv1alpha1.CheckoutGateStatus{Verdict: checkoutv1alpha1.VerdictWarn},
		},
		&checkoutv1alpha1.CheckoutGate{
			ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "default"},
			Status:     checkoutv1alpha1.CheckoutGateStatus{Verdict: checkoutv1alpha1.VerdictBreach},
		},
		&checkoutv1alpha1.CheckoutGate{
			ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "default"},
			Status:     checkoutv1alpha1.CheckoutGateStatus{Verdict: checkoutv1alpha1.VerdictBreach},
		},
		&checkoutv1alpha1.CheckoutGate{
			ObjectMeta: metav1.ObjectMeta{Name: "e", Namespace: "default"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(s).WithObjects(gates...).Build()
	srv := &Server{Reader: c}

	req := httptest.NewRequest("GET", "/api/gates/summary", nil)
	rec := httptest.NewRecorder()

	srv.handleSummary(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got VerdictCounts
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response body: %v (body: %s)", err, rec.Body.String())
	}

	want := VerdictCounts{Total: 5, Pass: 1, Warn: 1, Breach: 2, Unknown: 1}
	if got != want {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestHandleSummary_EmptyListReturnsZeroCounts(t *testing.T) {
	s := runtime.NewScheme()
	if err := checkoutv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("adding scheme: %v", err)
	}
	c := fake.NewClientBuilder().WithScheme(s).Build()
	srv := &Server{Reader: c}

	req := httptest.NewRequest("GET", "/api/gates/summary", nil)
	rec := httptest.NewRecorder()
	srv.handleSummary(rec, req)

	var got VerdictCounts
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response body: %v (body: %s)", err, rec.Body.String())
	}
	if got != (VerdictCounts{}) {
		t.Fatalf("expected zero counts, got %+v", got)
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

func TestWithCORS(t *testing.T) {
	called := false
	h := withCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/gates", nil))
	if !called || get.Code != http.StatusTeapot {
		t.Fatalf("GET must reach the wrapped handler, code=%d called=%v", get.Code, called)
	}
	if get.Header().Get("Access-Control-Allow-Origin") != "*" || get.Header().Get("Access-Control-Allow-Methods") != "GET, OPTIONS" {
		t.Fatalf("missing CORS headers: %v", get.Header())
	}

	called = false
	pre := httptest.NewRecorder()
	h.ServeHTTP(pre, httptest.NewRequest(http.MethodOptions, "/api/gates", nil))
	if called || pre.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS must short-circuit with 204, code=%d called=%v", pre.Code, called)
	}
}

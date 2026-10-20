package notify

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestVerdictTransition_Subject(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		gateName  string
		want      string
	}{
		{
			name:      "default namespace",
			namespace: "default",
			gateName:  "checkout-gate",
			want:      "checkoutgate.default.checkout-gate.verdict",
		},
		{
			name:      "different namespace and name",
			namespace: "retail",
			gateName:  "black-friday-checkout",
			want:      "checkoutgate.retail.black-friday-checkout.verdict",
		},
		{
			name:      "empty namespace and name still produces a well-formed subject",
			namespace: "",
			gateName:  "",
			want:      "checkoutgate...verdict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transition := VerdictTransition{Namespace: tt.namespace, Name: tt.gateName}
			if got := transition.Subject(); got != tt.want {
				t.Fatalf("Subject() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVerdictTransition_JSONPayloadShape(t *testing.T) {
	ts := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	transition := VerdictTransition{
		Namespace:        "default",
		Name:             "checkout-gate",
		TargetDeployment: "retail-checkout-api",
		OldVerdict:       "Pass",
		NewVerdict:       "Breach",
		Message:          "p99 latency 18094ms exceeds max 450ms",
		Timestamp:        ts,
	}

	payload, err := json.Marshal(transition)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	wantFields := map[string]string{
		"namespace":        "default",
		"name":             "checkout-gate",
		"targetDeployment": "retail-checkout-api",
		"oldVerdict":       "Pass",
		"newVerdict":       "Breach",
		"message":          "p99 latency 18094ms exceeds max 450ms",
	}
	for field, want := range wantFields {
		got, ok := decoded[field]
		if !ok {
			t.Fatalf("payload missing field %q, got: %s", field, payload)
		}
		if got != want {
			t.Fatalf("payload[%q] = %v, want %v", field, got, want)
		}
	}

	rawTimestamp, ok := decoded["timestamp"]
	if !ok {
		t.Fatalf("payload missing field %q, got: %s", "timestamp", payload)
	}
	gotTime, ok := rawTimestamp.(string)
	if !ok {
		t.Fatalf("payload[\"timestamp\"] = %v (%T), want a string", rawTimestamp, rawTimestamp)
	}
	parsed, err := time.Parse(time.RFC3339, gotTime)
	if err != nil {
		t.Fatalf("timestamp %q did not parse as RFC3339: %v", gotTime, err)
	}
	if !parsed.Equal(ts) {
		t.Fatalf("timestamp = %v, want %v", parsed, ts)
	}
}

// recordingNotifier is the fake used to prove the Notifier interface is
// what a caller actually depends on, not a concrete type. Anything
// satisfying this interface, this fake, NoopNotifier, or the real
// NATSNotifier, is interchangeable from the caller's side.
type recordingNotifier struct {
	calls []VerdictTransition
	err   error
}

func (r *recordingNotifier) NotifyVerdictTransition(_ context.Context, t VerdictTransition) error {
	r.calls = append(r.calls, t)
	return r.err
}

func TestNoopNotifier_NeverErrorsAndRecordsNothing(t *testing.T) {
	var n Notifier = NoopNotifier{}

	transitions := []VerdictTransition{
		{Namespace: "default", Name: "a", OldVerdict: "Pass", NewVerdict: "Warn"},
		{Namespace: "default", Name: "b", OldVerdict: "Warn", NewVerdict: "Breach"},
	}

	for _, transition := range transitions {
		if err := n.NotifyVerdictTransition(context.Background(), transition); err != nil {
			t.Fatalf("NoopNotifier.NotifyVerdictTransition() error = %v, want nil", err)
		}
	}
}

func TestRecordingNotifier_CapturesEachCallInOrder(t *testing.T) {
	var n Notifier = &recordingNotifier{}
	rec := n.(*recordingNotifier)

	first := VerdictTransition{Namespace: "default", Name: "checkout-gate", OldVerdict: "Pass", NewVerdict: "Warn"}
	second := VerdictTransition{Namespace: "default", Name: "checkout-gate", OldVerdict: "Warn", NewVerdict: "Breach"}

	if err := n.NotifyVerdictTransition(context.Background(), first); err != nil {
		t.Fatalf("NotifyVerdictTransition() error = %v", err)
	}
	if err := n.NotifyVerdictTransition(context.Background(), second); err != nil {
		t.Fatalf("NotifyVerdictTransition() error = %v", err)
	}

	if len(rec.calls) != 2 {
		t.Fatalf("len(calls) = %d, want 2", len(rec.calls))
	}
	if rec.calls[0] != first {
		t.Fatalf("calls[0] = %+v, want %+v", rec.calls[0], first)
	}
	if rec.calls[1] != second {
		t.Fatalf("calls[1] = %+v, want %+v", rec.calls[1], second)
	}
}

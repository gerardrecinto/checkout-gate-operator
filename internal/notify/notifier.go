// Package notify is the seam between a CheckoutGate verdict transition and
// whatever downstream system reacts to it: NATS in production, a no-op
// when nothing is configured, a fake in tests. Same interface-at-the-
// boundary pattern as controller.MetricsProvider: the reconciler doesn't
// know or care what's behind this, and nothing in this package knows
// about Kubernetes client types beyond the plain VerdictTransition struct
// below.
package notify

import (
	"context"
	"fmt"
	"time"
)

// VerdictTransition is published exactly once per real change in a
// CheckoutGate's verdict, never on every reconcile. A reconcile that
// re-confirms the same verdict is the same kind of noise gate.Evaluate()
// already declines to react to (see its CPU saturation comment, holding
// rather than escalating on a signal alone); a Notifier shouldn't
// manufacture that noise on the way out either.
type VerdictTransition struct {
	Namespace        string    `json:"namespace"`
	Name             string    `json:"name"`
	TargetDeployment string    `json:"targetDeployment"`
	OldVerdict       string    `json:"oldVerdict"`
	NewVerdict       string    `json:"newVerdict"`
	Message          string    `json:"message"`
	Timestamp        time.Time `json:"timestamp"`
}

// Subject is the NATS subject a transition publishes to. Kept as a
// method on the struct itself, in this package, rather than duplicated
// in each Notifier implementation, so the no-op notifier, the real NATS
// notifier, and their tests all agree on the naming scheme by
// construction instead of by convention.
func (t VerdictTransition) Subject() string {
	return fmt.Sprintf("checkoutgate.%s.%s.verdict", t.Namespace, t.Name)
}

// Notifier is the seam a CheckoutGateReconciler calls through when a
// CheckoutGate's verdict actually changes. Implementations should not
// block the reconcile loop for long; the NATS implementation publishes
// over an already-open connection for that reason.
type Notifier interface {
	NotifyVerdictTransition(ctx context.Context, t VerdictTransition) error
}

// NoopNotifier is the default when no notifier is configured: every call
// is a no-op that returns nil, so the operator behaves exactly as it did
// before this package existed.
type NoopNotifier struct{}

// NotifyVerdictTransition on NoopNotifier does nothing and never errors.
func (NoopNotifier) NotifyVerdictTransition(context.Context, VerdictTransition) error {
	return nil
}

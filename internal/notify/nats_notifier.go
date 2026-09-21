package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

// NATSNotifier publishes verdict transitions to NATS as JSON, one message
// per subject "checkoutgate.<namespace>.<name>.verdict". This is the real
// implementation behind the Notifier interface, wired in only when
// -nats-url is set on the manager; the rest of this repo depends on the
// interface, not on this type, the same seam pattern as
// controller.PrometheusMetricsProvider behind controller.MetricsProvider.
type NATSNotifier struct {
	conn *nats.Conn
}

// NewNATSNotifier connects to the given NATS URL and returns a Notifier
// backed by that connection. The connection is kept open for the life of
// the process; NotifyVerdictTransition reuses it rather than dialing per
// publish.
func NewNATSNotifier(url string) (*NATSNotifier, error) {
	conn, err := nats.Connect(url, nats.Name("checkout-gate-operator"))
	if err != nil {
		return nil, fmt.Errorf("connecting to NATS at %q: %w", url, err)
	}
	return &NATSNotifier{conn: conn}, nil
}

// Close drains and closes the underlying NATS connection. Meant to run
// once, on process shutdown.
func (n *NATSNotifier) Close() {
	if n.conn != nil {
		_ = n.conn.Drain()
	}
}

// NotifyVerdictTransition marshals t to JSON and publishes it to
// t.Subject(). nats.go's Publish call itself is non-blocking, it hands
// the message to the connection's internal buffer and flushes
// asynchronously, so the ctx check here only avoids doing the work at
// all once the reconcile's own context is already done, it doesn't cancel
// an in-flight publish.
func (n *NATSNotifier) NotifyVerdictTransition(ctx context.Context, t VerdictTransition) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	payload, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshaling verdict transition: %w", err)
	}

	if err := n.conn.Publish(t.Subject(), payload); err != nil {
		return fmt.Errorf("publishing to %q: %w", t.Subject(), err)
	}
	return nil
}

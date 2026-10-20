# NATS notifications: reacting to a verdict change without polling

**TL;DR: `internal/notify` is the same interface-at-the-boundary pattern
as `MetricsProvider`, a `Notifier` the reconciler calls through. The
default is a no-op. Set `-nats-url` and the reconciler instead publishes
a JSON message to NATS, but only when a `CheckoutGate`'s verdict actually
changes, never on every reconcile.**

## Why this exists

The Svelte dashboard already reads verdicts by polling
`internal/apiserver` every few seconds. Polling is fine for a human
looking at a screen; it's the wrong shape for another system that wants
to react the moment a gate goes from `Pass` to `Breach`, a deploy
pipeline gating a rollout, a paging system, a Slack bot. Those want to
be told, not to ask repeatedly. `internal/notify` gives them a subject
to subscribe to instead of a polling loop of their own.

## The interface, same shape as MetricsProvider

```go
type Notifier interface {
    NotifyVerdictTransition(ctx context.Context, t VerdictTransition) error
}
```

`CheckoutGateReconciler.Notifier` is this interface, not a concrete NATS
type, for the same reason `Metrics` is a `MetricsProvider` and not a
`*PrometheusMetricsProvider`: the reconciler and its tests shouldn't
need a real NATS server any more than they need a real Prometheus.
`NoopNotifier{}` is the zero-cost default, wired in by
`cmd/manager/main.go` whenever `-nats-url` is empty, and it's what makes
this feature opt-in: an operator that never sets the flag behaves
exactly as it did before this package existed.

## Detecting a transition, not a reconcile

The reconciler runs on every informer event, a spec edit, a target
Deployment's replica count changing, a periodic resync, most of which
land on the same verdict the gate already had. Publishing on every one
of those would turn one real signal (a verdict crossing a threshold)
into constant noise, which defeats the entire point of a subscriber not
having to poll.

`Reconcile` captures `cg.Status.Verdict` before writing anything, then
compares it against the freshly computed verdict after the status
write:

```go
previousVerdict := cg.Status.Verdict
// ... fetch Deployment, fetch metrics, gate.Evaluate(), write Status ...
r.notifyVerdictChange(ctx, &cg, previousVerdict)
```

`notifyVerdictChange` only calls the `Notifier` when `previousVerdict`
is non-empty and differs from the new verdict. An empty previous verdict
means this `CheckoutGate` has never been evaluated before, so its first
verdict is an initial state, not a change, the same reasoning
`gate.Evaluate()` applies when it holds at `Warn` instead of escalating
to `Breach` on a CPU signal alone: react to a real change, not to every
signal that crosses your desk.

## The payload and the subject

```go
type VerdictTransition struct {
    Namespace        string
    Name             string
    TargetDeployment string
    OldVerdict       string
    NewVerdict       string
    Message          string
    Timestamp        time.Time
}
```

marshaled to JSON and published to `checkoutgate.<namespace>.<name>.verdict`,
computed by `VerdictTransition.Subject()` so the publisher and any test
asserting on subject naming can't drift apart. A subscriber can wildcard
`checkoutgate.*.*.verdict` for every gate, or `checkoutgate.default.>`
for a namespace, standard NATS subject hierarchy, nothing custom.

## Testing without a live NATS server

`internal/notify/notifier_test.go` never dials NATS. It tests
`VerdictTransition.Subject()` and the JSON shape directly (both are
plain functions over plain structs), plus `NoopNotifier` and a small
`recordingNotifier` fake, both satisfying `Notifier`, to prove the
interface is what callers actually depend on.
`internal/controller/checkoutgate_controller_test.go` extends that same
`fakeNotifier` pattern already used for `fakeMetricsProvider`: one test
reconciles a gate that was already `Pass` into a `Breach` and asserts
the fake was called exactly once with the right `OldVerdict`/`NewVerdict`,
another reconciles the same healthy gate twice and asserts the fake was
never called, the unchanged-verdict case. `go test ./...` never touches
a network.

## Go deeper

- [docs.nats.io/nats-concepts/subjects](https://docs.nats.io/nats-concepts/subjects)
  for subject hierarchy and wildcard matching.
- [github.com/nats-io/nats.go](https://github.com/nats-io/nats.go), the
  client this repo's `NATSNotifier` wraps.

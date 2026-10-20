# Architecture

**TL;DR: One `controller-runtime` manager process runs three things
side by side: a reconciler watching `CheckoutGate` custom resources, a
validating admission webhook, and a small HTTP API the frontend reads
from. All three share one Kubernetes client and one informer cache.**

## The whole system, end to end

```text
                          kubectl apply -f my-gate.yaml
                                    |
                                    v
                    +-------------------------------+
                    |   API server (admission chain) |
                    |  schema check -> webhook call  |
                    +---------------+----------------+
                                    |
                       CheckoutGateValidator.ValidateCreate
                       (does the target Deployment exist?
                        real TLS, cert-manager-issued cert)
                                    |
                          (approved) v
                    +-------------------------------+
                    |     object persisted to etcd    |
                    +---------------+----------------+
                                    |
                    informer notices the create/update,
                    updates the shared cache, enqueues a Request
                                    |
                                    v
                    +-------------------------------+
                    |  CheckoutGateReconciler.Reconcile |
                    |  1. fetch CheckoutGate (cache)    |
                    |  2. fetch target Deployment       |
                    |  3. fetch live metrics (Prometheus)|
                    |  4. gate.Evaluate() -- pure logic |
                    |  5. write Status.Verdict           |
                    |  6. notify on verdict change only  |
                    |     (NATS, optional, see below)    |
                    +---------------+----------------+
                                    |
                    Status write also lands in the shared cache
                                    |
                                    v
                    +-------------------------------+
                    |   internal/apiserver.Server     |
                    |   GET /api/gates reads the cache |
                    |   (no fresh API server round-trip)|
                    +---------------+----------------+
                                    |
                                    v
                    +-------------------------------+
                    |   Svelte dashboard, polling     |
                    |   every 5s, color-coded verdicts |
                    +-------------------------------+
```

## Why one process, not three

`cmd/manager/main.go` starts a single `ctrl.Manager` and registers the
reconciler, the webhook, and the API server onto it (the API server via
`mgr.Add(...)`, since it implements `manager.Runnable`). This isn't
just convenience, it's what makes the API server cheap: it reads
through the same client the reconciler already uses, which is backed
by the manager's shared informer cache. `GET /api/gates` is a map
lookup against already-synced local state, not a new connection to the
Kubernetes API server on every request.

Leader election (`--leader-elect`) applies to all three components at
once for the same reason: only one replica should be actively
reconciling and serving writes at a time, but the API server is
read-only against a cache that's kept in sync on every replica
regardless of leadership, which is why
`Server.NeedLeaderElection()` returns `true` here specifically, to keep
its lifecycle tied to the same leader as the reconciler rather than
running independently on every replica.

## Package layout and what each one is responsible for

```text
api/v1alpha1/       The CheckoutGate type. No logic, just the schema.

internal/gate/       Pure evaluation: (Spec, Metrics) -> Verdict.
                      No Kubernetes types, no I/O. 100% test coverage
                      because there's nothing here that needs mocking.

internal/controller/ The reconciler (orchestration: fetch, evaluate,
                      write) and the real Prometheus-backed
                      MetricsProvider implementation.

internal/notify/     The Notifier seam: a no-op by default, a real
                      NATS publisher when -nats-url is set. Called by
                      the reconciler only when a verdict actually
                      changes, see technologies/nats-notifications.md.

internal/webhook/    The validating admission webhook. Depends on
                      client.Reader only, not the full reconciler.

internal/apiserver/  The status API. A thin JSON view over the same
                      client.Reader everything else uses.

cmd/manager/         Wiring only. No business logic lives here.
```

The pattern repeating across every `internal/` package is the same one:
the thing that does I/O (fetch a Deployment, query Prometheus, call the
Kubernetes API) sits at the edge, behind a small interface, and the
thing that makes a decision (`gate.Evaluate`) never touches I/O at all.
That's what makes each piece testable in isolation, see each package's
own `_test.go` file for exactly how.

## Related docs

- [GETTING_STARTED.md](GETTING_STARTED.md) for running this locally.
- [technologies/](technologies/) for a deep dive into each specific
  technology used here (`controller-runtime`, CRDs, webhooks,
  cert-manager, RBAC, Cosign, Trivy/govulncheck, distroless containers,
  Prometheus/PromQL, Svelte, Kustomize, NATS notifications).

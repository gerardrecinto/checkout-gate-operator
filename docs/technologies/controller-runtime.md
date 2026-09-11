# controller-runtime: how a Kubernetes controller actually works

**TL;DR: A controller is a loop. Something changes in the cluster, the
controller gets told, it reads the current state, decides what should be
true, and makes one change toward that. It repeats forever. `controller-runtime`
is the library that handles the "gets told" and "reads current state"
plumbing so you only have to write the "decides and makes one change" part.**

## The core idea: reconciliation, not event handling

The first mental model to drop if it's there: a controller does not
process events like a message queue consumer. It reconciles state. The
difference matters. An event handler says "X happened, do Y." A
reconciler says "here's the name of a thing that might have changed,
figure out from scratch what should be true about it, and make it so."

```text
Something changes (CheckoutGate edited, a Deployment it watches changes)
              |
              v
   A Request lands in the work queue: just a namespace + name,
   NOT a diff, NOT "what changed", just "go look at this again"
              |
              v
   Reconcile(ctx, req) runs:
     1. Fetch the current object fresh from the cache
     2. Fetch whatever else you need (the target Deployment)
     3. Decide what Status should say right now
     4. Write it
              |
              v
   Done. If nothing needs re-checking, return. If something's still
   pending, return a Result asking to be requeued later.
```

This repo's version:
[`internal/controller/checkoutgate_controller.go`](../../internal/controller/checkoutgate_controller.go),
`Reconcile` never looks at "what changed", it just re-derives the whole
verdict from scratch every time it's called: fetch the `CheckoutGate`,
fetch its target `Deployment`, fetch fresh metrics, evaluate, write
`Status`. That's deliberate, and it's the actual discipline
`controller-runtime` expects: **reconciliation must be idempotent**,
calling it twice in a row with nothing having changed should produce
the same result both times, with no side effect from the first call
that the second call depends on.

**Mnemonic: FIRE.** Fetch fresh (never trust a cached copy from
earlier in the function), Infer the desired state from scratch, Reconcile
by writing exactly one thing (usually Status), Exit, don't chain
multiple unrelated mutations in one Reconcile call.

## Why "just re-derive everything" instead of tracking a diff yourself

This is the part that feels wasteful the first time you see it, why
fetch the Deployment again if nothing about it changed? The answer is
where `controller-runtime`'s real value is: the client you call
(`r.Get`, `r.List`) isn't hitting the Kubernetes API server over the
network every time. It's reading a local, in-memory cache that's kept
in sync by an **informer**, a long-lived watch connection that streams
every change to every object your controller cares about and updates
the cache automatically. So "fetch it again" is a map lookup, not an
HTTP call. That's what makes "re-derive from scratch every time" cheap
enough to be the default instead of an optimization you have to earn.

## What actually triggers a Reconcile call

`SetupWithManager` in this repo's controller
(`checkoutgate_controller.go`) does two things:

```go
ctrl.NewControllerManagedBy(mgr).
    For(&checkoutv1alpha1.CheckoutGate{}).
    Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(r.mapDeploymentToGates)).
    Complete(r)
```

`For(&CheckoutGate{})` is the primary resource: any create, update, or
delete of a `CheckoutGate` enqueues a `Request` for that exact object.
`Watches(&Deployment{}, ...)` is the interesting one: this controller
also needs to react when a Deployment it doesn't own changes (someone
scales it, someone rolls out a new version), and there's no
OwnerReference tying that Deployment back to a CheckoutGate to make the
default `Owns()` machinery work. The `EnqueueRequestsFromMapFunc`
handler is the escape hatch: it's a function that takes the changed
Deployment and returns a list of `Request`s (CheckoutGate names) that
should be re-reconciled because of it. `mapDeploymentToGates` in this
repo does exactly that: list every CheckoutGate in the Deployment's
namespace, find the ones referencing it by name, enqueue those.

## Why the fake client tests are real tests, not stubs

`checkoutgate_controller_test.go` uses
`sigs.k8s.io/controller-runtime/pkg/client/fake`, which is worth
understanding precisely: it's not a hand-rolled mock with a handful of
methods stubbed out. It's a real, in-memory implementation of the same
`client.Client` interface the production code depends on, backed by an
actual object tracker that understands schemas, resource versions, and
status subresources (`WithStatusSubresource(cg)` in the tests is what
makes `.Status().Update()` behave the same way it does against a real
API server, where spec and status update through separate endpoints).
That's why these tests catch real bugs: they exercise the actual
`Get`/`List`/`Update` code paths, just against an in-memory store
instead of etcd.

## Manager, not just controller

One more piece worth naming: `cmd/manager/main.go` doesn't start the
controller directly, it starts a `Manager`, and the controller, the
webhook server, and the status API (`internal/apiserver`) all register
themselves onto that one manager. The manager owns the shared cache,
the shared client, leader election, and health checks, one process, one
set of Kubernetes API connections, multiple components riding on top.

## Go deeper

- The official book: [book.kubebuilder.io](https://book.kubebuilder.io/)
  walks the exact same CRD + controller + webhook shape this repo uses,
  step by step.
- [pkg.go.dev/sigs.k8s.io/controller-runtime](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
  for the actual API reference when a specific type's behavior needs
  checking.

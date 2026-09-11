# CRDs and code generation: teaching Kubernetes a new resource type

**TL;DR: A CustomResourceDefinition (CRD) registers a new resource kind
with the Kubernetes API server, so `kubectl get checkoutgates` works the
same way `kubectl get pods` does. `controller-gen` reads Go struct tags
and comment markers on your types and generates both the CRD's OpenAPI
schema and the boilerplate Go code Kubernetes needs, so you write the
struct once and never hand-maintain either.**

## What a CRD actually is

Kubernetes' API server is generic. It doesn't know what a "Pod" is any
more specially than it knows what a "CheckoutGate" is, both are just
JSON objects stored in etcd, validated against a schema, exposed over a
REST-ish API, watchable. A CRD is the registration step: "here is a new
kind called CheckoutGate, in group checkout.gerardrecinto.dev, here's
its schema, store and serve it like any other resource."

```text
config/crd/checkout.gerardrecinto.dev_checkoutgates.yaml
              |
              v (kubectl apply -f)
      API server now understands "CheckoutGate" as a real kind
              |
              v
   kubectl get checkoutgates    <- works, same as any built-in type
   kubectl apply -f my-gate.yaml  <- validated against the CRD's schema
```

This repo's CRD lives at
[`api/v1alpha1/checkoutgate_types.go`](../../api/v1alpha1/checkoutgate_types.go)
as a plain Go struct. Everything else, the YAML schema, the code
Kubernetes needs to deep-copy the struct safely, is generated from it,
never hand-written.

## The two things `controller-gen` generates here, and why both matter

**1. The OpenAPI schema (`config/crd/*.yaml`)**, from `+kubebuilder:validation`
markers on the struct fields:

```go
// +kubebuilder:validation:Minimum=0
// +kubebuilder:validation:Maximum=100000
MaxErrorRateMilliPercent int32 `json:"maxErrorRateMilliPercent"`
```

This isn't decoration, the API server itself enforces it, before your
controller or webhook ever sees the object. Submit a
`CheckoutGate` with `maxErrorRateMilliPercent: 999999` and it's
rejected at `kubectl apply` time with a clear schema error, not
something your Go code has to defend against later.

**Real bug this caught in this repo, worth knowing about because it'll
happen again:** the first version of this CRD used `float64` for the
threshold fields. `controller-gen crd` refused to generate a clean
schema and printed `found float, the usage of which is highly
discouraged`. Floating-point numbers don't round-trip identically
across every JSON parser in every language a Kubernetes client might be
written in, so the API convention is to avoid them in CRD schemas
entirely. The fix, used throughout this repo, is the same one
Kubernetes itself uses for CPU (millicores): represent fractional
values as whole-number thousandths. `500` means `0.500%`, exact,
unambiguous, no float anywhere in the wire format.

**2. `DeepCopyObject` and friends (`api/v1alpha1/zz_generated.deepcopy.go`)**,
required because `controller-runtime`'s caching and client machinery
needs to hand out copies of objects, not shared references, so one
reconciler can't accidentally mutate another's view of the same object
mid-flight. Writing this by hand for every struct, including every
nested struct, is exactly the kind of repetitive, error-prone code
generation exists to remove.

## Running it yourself

```bash
go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5 \
  object:headerFile="" paths="./api/..."

go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5 \
  crd paths="./api/..." output:crd:artifacts:config=config/crd
```

The first command regenerates `zz_generated.deepcopy.go`. The second
regenerates the CRD YAML. `.github/workflows/ci.yml`'s `manifests` job
runs both and diffs the result against what's committed, so a struct
change that someone forgets to regenerate for fails CI loudly instead
of silently drifting.

## Validating a sample against the real generated schema, without a cluster

You don't need a running Kubernetes cluster to check whether a sample
CR is valid against the CRD schema, the schema is just JSON Schema
under `spec.versions[0].schema.openAPIV3Schema`, extractable and
checkable with any JSON Schema validator:

```python
import yaml, jsonschema
crd = yaml.safe_load(open("config/crd/checkout.gerardrecinto.dev_checkoutgates.yaml"))
schema = crd["spec"]["versions"][0]["schema"]["openAPIV3Schema"]["properties"]["spec"]
sample = yaml.safe_load(open("config/samples/checkout_v1alpha1_checkoutgate.yaml"))
jsonschema.validate(instance=sample["spec"], schema=schema)
```

This is exactly how this repo's manifests were checked before ever
touching a real cluster, including deliberately feeding it invalid
input (an out-of-range percent, a missing required field) to confirm
the schema actually rejects what it's supposed to, not just accepts
everything silently.

## Go deeper

- [kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
  for the concept from the API server's side.
- [book.kubebuilder.io/reference/markers](https://book.kubebuilder.io/reference/markers.html)
  is the actual reference for every `+kubebuilder:` marker, not just
  the validation ones used here.

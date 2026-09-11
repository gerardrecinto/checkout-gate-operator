# Kustomize: composing YAML without templating it

**TL;DR: Kustomize combines a set of plain, valid YAML manifests into
one applyable set, `kubectl apply -k config/`. No templating language,
no `{{ }}` placeholders anywhere, every file in `config/` is real,
directly-applyable YAML on its own, Kustomize's job is only to gather
and, when needed, patch.**

## Why not just a shell script cat-ing YAML files together

```text
config/
  kustomization.yaml   <- lists which files belong together
  manager/deployment.yaml
  rbac/role.yaml
  rbac/binding_and_sa.yaml
  crd/checkout.gerardrecinto.dev_checkoutgates.yaml
  webhook/certificate.yaml
```

```yaml
# config/kustomization.yaml
resources:
  - manager/deployment.yaml
  - rbac/role.yaml
  - rbac/binding_and_sa.yaml
  - crd/checkout.gerardrecinto.dev_checkoutgates.yaml
  - webhook/certificate.yaml
```

`kubectl kustomize config/` reads that list and concatenates the real
documents from each file into one multi-document YAML stream, in an
order Kustomize determines is safe (namespaces and CRDs before the
things that depend on them, for instance). `kubectl apply -k config/`
does the same thing and applies the result directly. The advantage
over a shell script doing `cat *.yaml` is everything Kustomize adds on
top when a deployment actually needs to differ across environments,
none of which this repo currently uses, but which is the entire reason
the tool exists instead of a simpler concatenation script: overlays
that patch a base (a different replica count for staging versus prod),
without duplicating the whole manifest per environment or reaching for
a templating language that turns valid YAML into a text file with
`{{ }}` scattered through it that can't be validated as YAML until
after substitution.

## Why "no templating" is the actual design principle, not an accident

Every file this repo's `config/` directory contains is valid,
`kubectl apply -f`-able YAML on its own, verified directly in this
repo's own build process:

```bash
kubectl apply --dry-run=client --validate=true -f config/rbac/role.yaml
kubectl apply --dry-run=client --validate=true -f config/manager/deployment.yaml
```

Both pass, individually, with no Kustomize involved at all. That's the
actual design principle Kustomize is built around, worth contrasting
directly with Helm's approach: a Helm chart's raw template
files are not valid YAML until Helm's templating engine renders them,
which means you can't point a generic YAML linter or `kubectl
--dry-run` at the source files directly, only at rendered output. A
Kustomize base has no such gap, the source and the applyable output are
the same syntax the whole way through.

## What Kustomize is actually for, beyond "combine some files"

The feature this repo doesn't need yet, but that's the actual reason
to reach for Kustomize over a plain script, is overlays: a `base/`
directory with the common manifests, and a `overlays/staging/`,
`overlays/prod/` directory each containing a small `kustomization.yaml`
that references the base and applies patches (a different replica
count, a different image tag, an extra env var) on top, without
copying the base manifests at all. The base stays the single source of
truth; each overlay is only the diff.

## Go deeper

- [kubectl.docs.kubernetes.io/references/kustomize](https://kubectl.docs.kubernetes.io/references/kustomize/)
  is the official reference.
- [kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/)
  covers the overlay pattern this repo doesn't use yet, with real
  examples.

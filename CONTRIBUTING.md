# Contributing

This is a personal portfolio project, but it's built to real
engineering standards and treats contributions the same way a
production repo would.

## Before opening a PR

```bash
go build ./...
go vet ./...
go test ./... -race
gofmt -l .          # should print nothing

cd frontend && npm run check && npm run build

cd scripts && pytest -v
```

If you touch anything under `api/v1alpha1/`, regenerate the CRD and
deepcopy code before committing, CI checks this and fails on drift:

```bash
go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5 object:headerFile="" paths="./api/..."
go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5 crd paths="./api/..." output:crd:artifacts:config=config/crd
```

If you add a new `+kubebuilder:rbac` marker, regenerate the RBAC role
too:

```bash
go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5 rbac:roleName=checkout-gate-operator-manager paths="./..." output:rbac:artifacts:config=config/rbac
```

## What CI actually checks

Six jobs, all required before the image job runs: Go build/vet/test,
frontend build/type-check, Python script tests, generated-manifest
freshness, `govulncheck` + Trivy + Gitleaks, then the image itself gets
scanned again before it's pushed and signed. See
[`docs/technologies/`](docs/technologies/) for what each of those
actually checks and why.

## Style

- Interfaces at I/O boundaries (`MetricsProvider`, `client.Reader`),
  pure logic behind them. See `internal/gate/evaluator.go` for the
  pattern this repo follows everywhere else.
- No comment explains *what* code does if the code already says it
  clearly. A comment earns its place by explaining *why*, a
  non-obvious constraint, a decision that looks wrong until you know
  the reason.

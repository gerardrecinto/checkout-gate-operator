# Getting started

## Prerequisites

- Go 1.25+ (the module declares `go 1.25.0`; `go run`'s `GOTOOLCHAIN=auto`
  will fetch a newer one automatically if a tool needs it, as
  `govulncheck` does)
- Node 20+ and npm, for the frontend
- Docker, for building the image
- `kubectl` (1.27+) for manifest validation and `kustomize` (bundled
  into `kubectl` since 1.14)
- A real Kubernetes cluster plus cert-manager installed, only if you
  want to run the manager against a live cluster, everything else below
  works without one

## Run the Go side

```bash
go build ./...
go vet ./...
go test ./... -race -coverprofile=coverage.out
go tool cover -func=coverage.out
```

## Run the frontend

```bash
cd frontend
npm install
npm run dev      # starts Vite's dev server, proxies /api to :8081
npm run check    # svelte-check, full type verification
npm run build    # real production build into dist/
```

## Build the image

```bash
docker build -t checkout-gate-operator:local .

# confirm the hardening claims directly rather than trusting the Dockerfile
docker inspect checkout-gate-operator:local --format '{{.Config.User}}'   # 65532:65532
docker run --rm --entrypoint=/bin/sh checkout-gate-operator:local -c "echo hi"  # fails, no shell
```

## Regenerate the CRD and deepcopy code after changing `api/v1alpha1`

```bash
go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5 \
  object:headerFile="" paths="./api/..."

go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5 \
  crd paths="./api/..." output:crd:artifacts:config=config/crd

go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5 \
  rbac:roleName=checkout-gate-operator-manager paths="./..." \
  output:rbac:artifacts:config=config/rbac
```

`.github/workflows/ci.yml`'s `manifests` job runs the first two and
fails if the committed output doesn't match, so this isn't optional
after touching a CRD field or an RBAC marker, CI will catch it either
way.

## Validate the Kubernetes manifests without a cluster

```bash
kubectl apply --dry-run=client --validate=true -f config/rbac/role.yaml
kubectl apply --dry-run=client --validate=true -f config/manager/deployment.yaml
kubectl kustomize config/ > /dev/null && echo "kustomize build OK"
```

The `cert-manager.io` resources in `config/webhook/certificate.yaml`
won't validate this way unless cert-manager's own CRDs are installed
locally, that's expected, not a defect, see
[docs/technologies/cert-manager-and-tls.md](technologies/cert-manager-and-tls.md).

## Security scans, the same ones CI runs

```bash
GOTOOLCHAIN=auto go run golang.org/x/vuln/cmd/govulncheck@latest ./...
trivy fs --scanners vuln --severity CRITICAL,HIGH --exit-code 1 --ignore-unfixed .
```

## Run the Python verification script's tests

```bash
cd scripts
pip install -r requirements.txt
pytest -v
```

## Deploy to a real cluster

```bash
# cert-manager first, this repo's webhook cert depends on it
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
kubectl wait --for=condition=Available deployment --all -n cert-manager --timeout=120s

kubectl apply -k config/

# once the manager is up and a CheckoutGate + its target Deployment exist:
python3 scripts/verify_gate.py --gate retail-checkout-gate --namespace default --expect-verdict Pass
```

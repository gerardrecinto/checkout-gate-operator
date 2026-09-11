# Security

## Reporting a vulnerability

Open a [GitHub issue](https://github.com/gerardrecinto/checkout-gate-operator/issues)
or reach out directly at gerardrecinto@gmail.com. This is a personal
portfolio project, not a production system anyone depends on, so
there's no formal disclosure embargo process, a plain issue is fine for
anything found here.

## What's actually scanned, and how often

Every push and pull request against `main` runs, per
[`.github/workflows/ci.yml`](.github/workflows/ci.yml):

- **`govulncheck`**, Go's own reachability-based vulnerability scanner,
  flags only vulnerabilities the actual code path calls into.
- **Trivy**, both a filesystem scan (`go.mod`, `frontend/package.json`)
  and, after the image is built, a full image scan covering the base
  OS layer too.
- **Gitleaks**, secret scanning across the diff.

See [`docs/technologies/trivy-and-govulncheck.md`](docs/technologies/trivy-and-govulncheck.md)
for why both scanners run rather than just one, they've each caught
real CVEs the other missed while building this repo.

## Supply chain

Every image pushed to `ghcr.io/gerardrecinto/checkout-gate-operator` is
signed with keyless Cosign, tied to this exact repo's `ci.yml` workflow
identity via Sigstore's OIDC flow, no long-lived signing key exists to
leak. Verify any tag yourself:

```bash
cosign verify \
  --certificate-identity-regexp "^https://github.com/gerardrecinto/checkout-gate-operator/.github/workflows/ci.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  ghcr.io/gerardrecinto/checkout-gate-operator:<tag>
```

See [`docs/technologies/cosign-and-sigstore.md`](docs/technologies/cosign-and-sigstore.md)
for how the keyless flow actually works.

## Runtime posture

- Distroless, nonroot base image, no shell, verified directly (see
  [`docs/technologies/distroless-containers.md`](docs/technologies/distroless-containers.md)),
  not just declared in a Dockerfile comment.
- RBAC generated from `+kubebuilder:rbac` markers on the exact code
  that needs each permission, not a hand-written role that
  accumulates unused grants over time.
- The admission webhook runs over real, cert-manager-issued and
  auto-rotated TLS, no static or manually-generated certificate
  anywhere in this repo.

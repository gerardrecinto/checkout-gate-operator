# Changelog

## 2026-09-11

- Initial build: `CheckoutGate` CRD, `controller-runtime` reconciler
  against live error rate/p99 latency/CPU saturation, a validating
  admission webhook over cert-manager TLS, a Svelte dashboard on a Go
  status API, a distroless nonroot image, Trivy + govulncheck +
  keyless Cosign wired into CI.
- Fixed a wrong pinned `trivy-action` version tag in CI (a guessed tag
  GitHub couldn't resolve, replaced with the same pinned SHA already
  proven working in this author's other repos).
- Fixed `govulncheck`'s own toolchain requirement in CI (the scanner
  needs a newer Go than this module targets to run at all).
- Bumped `golang.org/x/net` and `golang.org/x/text` for 3 real,
  reachable CVEs `govulncheck` flagged in the actual `Reconcile` call
  path.
- Bumped `golang.org/x/net` again (a second, different CVE) and
  `golang.org/x/oauth2`, both flagged by Trivy's broader
  (non-reachability-based) filesystem scan after the `govulncheck` fix
  had already shipped, the module's minimum Go version moved to 1.25
  as a result. Verified clean locally with both scanners before the
  next push, not just after CI failed a second time.
- Added `docs/`: an architecture doc, a getting-started guide, and a
  dedicated technology deep-dive for each real piece of this stack
  (`controller-runtime`, CRDs/codegen, admission webhooks,
  cert-manager/TLS, RBAC, Cosign/Sigstore, Trivy/govulncheck,
  distroless containers, Prometheus/PromQL, Svelte, Kustomize).
- Added `CONTRIBUTING.md`, `SECURITY.md`.

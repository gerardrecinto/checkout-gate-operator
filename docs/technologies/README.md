# Technology deep dives

Each doc here explains a real technology used in this repo: how it
actually works, and exactly how and where this repo uses it, with file
references, not a generic tutorial copy-pasted from the tool's own
docs.

| Doc | What it covers |
| :--- | :--- |
| [controller-runtime.md](controller-runtime.md) | Reconciliation loops, the informer cache, why `Reconcile` re-derives everything from scratch every time |
| [crds-and-codegen.md](crds-and-codegen.md) | CustomResourceDefinitions, `controller-gen`, the real `float64` mistake it caught in this repo |
| [admission-webhooks.md](admission-webhooks.md) | What a schema can't check that a webhook can, `failurePolicy`, this repo's live-existence check |
| [cert-manager-and-tls.md](cert-manager-and-tls.md) | Self-issued CAs, `Issuer` vs `ClusterIssuer`, automatic cert rotation for the webhook |
| [rbac.md](rbac.md) | ClusterRole/Binding/ServiceAccount, and why this repo's role is generated from code markers, not hand-written |
| [cosign-and-sigstore.md](cosign-and-sigstore.md) | Keyless signing, Fulcio, Rekor, why signing and scanning answer different questions |
| [trivy-and-govulncheck.md](trivy-and-govulncheck.md) | Reachability-based vs. presence-based vulnerability scanning, proven by two real CVE rounds this repo hit |
| [distroless-containers.md](distroless-containers.md) | What "distroless" strips out, the nonroot UID, and proving no-shell directly instead of trusting a comment |
| [prometheus-promql.md](prometheus-promql.md) | Pull-based scraping, reading this repo's actual PromQL line by line, `histogram_quantile` |
| [svelte.md](svelte.md) | Compiler vs. runtime framework, why `let gates = []` is reactive with no extra API |
| [kustomize.md](kustomize.md) | Composing plain YAML without templating it, contrasted with Helm's render step |
| [nats-notifications.md](nats-notifications.md) | The optional `Notifier` seam, detecting a real verdict transition instead of reacting to every reconcile, subject naming |

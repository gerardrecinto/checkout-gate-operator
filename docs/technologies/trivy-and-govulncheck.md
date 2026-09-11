# Trivy and govulncheck: two scanners that catch different things

**TL;DR: `govulncheck` only flags a vulnerability if your code's actual
call graph reaches the vulnerable function, fewer false positives, Go
specific. Trivy flags any known-vulnerable version of any dependency
you have installed, regardless of whether you call it, broader net,
language-agnostic, also scans the container image itself. This repo
runs both, on purpose, because they genuinely catch different things.**

## The real difference, proven by this repo's own CI failures

This isn't a hypothetical comparison, it's exactly what happened
building this repo. `govulncheck` flagged 3 vulnerabilities reachable
from `Reconcile`'s actual call path (`golang.org/x/net`'s `idna`
package, `golang.org/x/text`'s `norm` package, an HTTP/2 transport
path), all genuinely exercised by code this controller calls. Fixing
those satisfied `govulncheck`. Pushing that fix, Trivy's filesystem
scan then flagged two *more* CVEs in the same `go.mod`,
`golang.org/x/net` (a different CVE than the one govulncheck found) and
`golang.org/x/oauth2`, neither of which govulncheck had flagged, because
nothing in this repo's actual code path calls the vulnerable functions.
Trivy doesn't check reachability, it checks "is a known-bad version of
this dependency present," full stop.

```text
govulncheck: "your code's call graph reaches a vulnerable function"
             -> fewer results, all provably exploitable through this code
             -> Go-only, source-level

Trivy (fs):  "a vulnerable version of a dependency is present"
             -> more results, some may be dead code paths you never call
             -> language-agnostic, works on go.mod, package.json, etc.

Trivy (image): everything the fs scan does, PLUS the base OS packages
               baked into the actual container layers
```

Neither one is strictly better, they answer different questions. A
dependency with a vulnerable function you never call is still worth
patching eventually (you might call it tomorrow, or a transitive
dependency might start calling it), which is exactly why running both
scanners, not picking one, is the actual security posture this repo
takes.

## Where each runs in this repo

```yaml
# .github/workflows/ci.yml, security job
- name: govulncheck
  uses: golang/govulncheck-action@v1
  with:
    go-version-input: "1.26"   # the scanner's own toolchain requirement,
                                 # separate from what this module targets
- name: Trivy filesystem scan
  uses: aquasecurity/trivy-action@...  # scans go.mod/go.sum, frontend's package.json

# .github/workflows/ci.yml, image job, after the image is built but before it's pushed
- name: Trivy image scan (block on critical/high)
  uses: aquasecurity/trivy-action@...
  with:
    image-ref: local/checkout-gate-operator:scan
```

The image scan matters separately from the filesystem scan: it's
checking the actual base OS layer (`gcr.io/distroless/static-debian12`)
and anything baked into the final image, not just this repo's own
dependency manifests. A clean `go.mod` says nothing about whether the
base image itself has an unpatched OS package.

## Running both locally before pushing

```bash
# govulncheck needs a newer Go toolchain than this module's go.mod declares,
# GOTOOLCHAIN=auto lets Go download it automatically
GOTOOLCHAIN=auto go run golang.org/x/vuln/cmd/govulncheck@latest ./...

trivy fs --scanners vuln --severity CRITICAL,HIGH --exit-code 1 --ignore-unfixed .
```

Both were run locally against this exact repo before every dependency
bump in its git history got pushed, catching the CVEs before CI did,
the second time, after the first round taught the lesson that a green
`govulncheck` run doesn't mean a green `trivy` run.

## Go deeper

- [go.dev/security/vuln](https://go.dev/security/vuln/) is Go's own
  vulnerability database and the reasoning behind reachability-based
  scanning specifically.
- [trivy.dev/latest/docs](https://trivy.dev/latest/docs/) covers every
  scan target Trivy supports beyond what this repo uses (IaC
  misconfiguration scanning, license scanning, SBOM generation).

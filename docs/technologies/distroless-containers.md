# Distroless containers: removing everything an attacker could use

**TL;DR: A typical base image (`ubuntu`, `debian`) ships a shell, a
package manager, and hundreds of utilities your application never uses,
every one of them is something code execution inside the container
could use to explore, persist, or move laterally. Distroless strips all
of it down to the language runtime and nothing else.**

## What "distroless" actually means

```text
ubuntu:22.04                    gcr.io/distroless/static-debian12:nonroot
  bash, sh                        (no shell at all)
  apt, dpkg                       (no package manager)
  curl, wget, ls, cat, grep...    (no coreutils)
  a full init system              (just enough to run one binary)
  ~78MB minimum                   this repo's full image: 41MB, including
                                   a statically-linked Go binary AND the
                                   built frontend's static assets
```

"Distroless" doesn't mean "no operating system," it means no Linux
*distribution* userland, no shell, no package manager, no general-purpose
utilities, just the handful of shared libraries (or, for a fully static
binary, none at all) a specific runtime needs. `static-debian12`
specifically expects a statically linked binary with zero dynamic
library dependencies, which is exactly what this repo's build produces.

## This repo's three-stage build, and why each stage exists

```dockerfile
FROM node:20-slim AS frontend-builder   # builds the Svelte app
FROM golang:1.25 AS go-builder          # builds the Go binary
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=go-builder /out/manager /manager
COPY --from=frontend-builder /frontend/dist /frontend-dist
USER 65532:65532
ENTRYPOINT ["/manager"]
```

Multi-stage builds exist precisely so the final image never sees the
tools used to build it. Node, npm, the entire frontend `node_modules`
tree, the Go compiler, none of that ships in the runtime image, they're
thrown away with the intermediate stages. Only the two things actually
needed at runtime, the compiled binary and the built static assets, get
copied into the final stage.

`CGO_ENABLED=0` in the Go build step is what makes the binary
statically linked, without it, Go would dynamically link against the
build environment's C library, and that library wouldn't exist in the
distroless runtime image, the binary would fail to start with a missing
shared object error.

## The nonroot user, and why 65532 specifically

`distroless/static-debian12:nonroot` ships a pre-created user at UID/GID
`65532` (the tag itself, `:nonroot`, is what selects this variant over
the default root-running one). `USER 65532:65532` in the Dockerfile
switches to it explicitly. This matters beyond "best practice": a
process running as root inside a container that escapes the container
boundary (a kernel exploit, a misconfigured volume mount) is root on
whatever it escaped to. A process running as UID 65532, a completely
unprivileged, meaningless ID on the host, has far less to work with
even in that worst case.

## Verifying this for real, not just trusting the Dockerfile

Comments in a Dockerfile are not proof. This repo's hardening claims
were checked directly:

```bash
$ docker inspect checkout-gate-operator:test --format '{{.Config.User}}'
65532:65532

$ docker run --rm --entrypoint=/bin/sh checkout-gate-operator:test -c "echo hi"
docker: Error response from daemon: ... exec: "/bin/sh":
stat /bin/sh: no such file or directory
```

The second command is the meaningful one: explicitly trying to override
the entrypoint to a shell, and getting a "no such file" error rather
than an actual shell, is direct proof there is no shell binary anywhere
in the image, not an inference from what's absent in a Dockerfile.

## Go deeper

- [github.com/GoogleContainerTools/distroless](https://github.com/GoogleContainerTools/distroless)
  is the actual base image project, with the full list of variants
  (`base`, `static`, `cc`, language-specific ones for Java/Python/Node).
- [docs.docker.com/build/building/multi-stage](https://docs.docker.com/build/building/multi-stage/)
  for the multi-stage build mechanics generally, not distroless-specific.

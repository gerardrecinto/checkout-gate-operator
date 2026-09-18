# Stage 1: build the frontend. Kept in its own stage so the final image
# never sees node, npm, or the frontend's dependency tree, only the
# static files vite already compiled.
FROM node:26-slim AS frontend-builder
WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# Stage 2: build the Go manager binary. CGO disabled and a static build so
# the runtime stage needs nothing beyond the binary itself, no libc
# dependency to drag into a distroless image.
FROM golang:1.25 AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api/ api/
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/manager ./cmd/manager

# Stage 3: runtime. Distroless static base, no shell, no package manager,
# nothing for an attacker to pivot with even with code execution inside
# the container. Runs as the distroless nonroot user (UID 65532), not
# root, and the filesystem it needs to write to (webhook certs) is the
# only writable mount, everything else is read-only at the Pod spec level
# (see config/manager, readOnlyRootFilesystem: true).
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /
COPY --from=go-builder /out/manager /manager
COPY --from=frontend-builder /frontend/dist /frontend-dist
USER 65532:65532

ENTRYPOINT ["/manager"]

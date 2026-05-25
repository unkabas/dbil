# syntax=docker/dockerfile:1.7

# Stage 1: build the React SPA. Single stage for all target arches because
# the output is platform-agnostic static assets.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# Stage 2: build the Go binary. Embeds the SPA produced by the web stage.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# .dockerignore excludes dist/ from the source copy so we always rely on the
# fresh bundle produced by the web stage just above.
COPY . .
COPY --from=web /web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
        -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
        -o /out/dbil ./cmd/dbil

# Stage 3: minimal runtime image. Static binary + the SPA both live inside
# /dbil — no external assets at runtime.
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="dbil"
LABEL org.opencontainers.image.description="Security-first PostgreSQL tool"
LABEL org.opencontainers.image.source="https://github.com/unkabas/dbil"
LABEL org.opencontainers.image.licenses="Apache-2.0"

COPY --from=build /out/dbil /dbil

# The container starts as root only long enough for `dbil init|serve` to fix
# /data ownership on fresh Docker named volumes. The binary then drops to
# UID 65532 / GID 0 before opening the state DB or serving HTTP.
USER 0:0
EXPOSE 4242
ENTRYPOINT ["/dbil"]
CMD ["version"]

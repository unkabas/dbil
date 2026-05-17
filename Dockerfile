# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
        -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
        -o /out/dbil ./cmd/dbil

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="dbil"
LABEL org.opencontainers.image.description="Security-first PostgreSQL tool"
LABEL org.opencontainers.image.source="https://github.com/unkabas/dbil"
LABEL org.opencontainers.image.licenses="Apache-2.0"

COPY --from=build /out/dbil /dbil

USER nonroot:nonroot
EXPOSE 4242
ENTRYPOINT ["/dbil"]
CMD ["version"]

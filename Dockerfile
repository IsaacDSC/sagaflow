# syntax=docker/dockerfile:1

# Match go.mod; override with: docker build --build-arg GO_VERSION=1.26 .
ARG GO_VERSION=1.25.8

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

# git: some module proxies / replace directives may need it during go mod download
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -buildvcs=false -o /out/sagaflow ./cmd && \
    go build -trimpath -ldflags="-s -w" -buildvcs=false -o /out/example ./example && \
    go build -trimpath -ldflags="-s -w" -buildvcs=false -o /out/migrate ./migrations

FROM gcr.io/distroless/static-debian12:nonroot AS example

COPY --from=builder /out/example /example

USER nonroot:nonroot

EXPOSE 3002 3003

ENTRYPOINT ["/example"]

FROM gcr.io/distroless/static-debian12:nonroot AS migrate

COPY --from=builder /out/migrate /migrate

USER nonroot:nonroot

ENTRYPOINT ["/migrate"]
# github.com/IsaacDSC/migrations Start() requires argv[1] (up|down|new|version|help)
CMD ["up"]

FROM gcr.io/distroless/static-debian12:nonroot AS sagaflow

COPY --from=builder /out/sagaflow /sagaflow

USER nonroot:nonroot

EXPOSE 3001

ENTRYPOINT ["/sagaflow"]

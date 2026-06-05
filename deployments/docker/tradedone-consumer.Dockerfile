# Multi-stage build for cmd/tradedone-consumer.
#
# Build:
#   docker build -f deployments/docker/tradedone-consumer.Dockerfile \
#     -t venturoid/tuai-be-tradedone-consumer:1.0.0 .

FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/tradedone-consumer ./cmd/tradedone-consumer

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/tradedone-consumer /tradedone-consumer

ENV TZ=UTC
ENV ENV=production

USER 65532:65532

ENTRYPOINT ["/tradedone-consumer"]

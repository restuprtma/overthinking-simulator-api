# Multi-stage build for cmd/resend-trade-consumer.
#
# Build:
#   docker build -f deployments/docker/resend-trade-consumer.Dockerfile \
#     -t venturoid/tuai-be-resend-trade-consumer:1.0.0 .

FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/resend-trade-consumer ./cmd/resend-trade-consumer

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/resend-trade-consumer /resend-trade-consumer

ENV TZ=UTC
ENV ENV=production

USER 65532:65532

ENTRYPOINT ["/resend-trade-consumer"]

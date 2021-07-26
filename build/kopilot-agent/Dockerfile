FROM golang:1.16.6-alpine3.14 AS builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/
RUN --mount=type=cache,target=/root/.cache/go-build go build cmd/kopilot-agent/main.go


FROM alpine:3.14

COPY --from=builder /workspace/main /kopilot-agent
ENTRYPOINT ["/kopilot-agent"]

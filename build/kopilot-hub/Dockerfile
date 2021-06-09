FROM golang:1.16-alpine3.13 AS builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/
RUN --mount=type=cache,target=/root/.cache/go-build go build cmd/kopilot-hub/main.go


FROM alpine:3.13

ARG AGENT_IMAGE
ENV AGENT_IMAGE=$AGENT_IMAGE

COPY --from=builder /workspace/main /kopilot-hub
ENTRYPOINT ["/kopilot-hub"]

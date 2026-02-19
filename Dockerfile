# syntax=docker/dockerfile:1

# Build stage: compile a static Go binary.
FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/my_shell ./cmd/my_shell

# Runtime stage: keep a small image, but still include basic shell tools from Alpine.
FROM alpine:3.21 AS runtime
RUN apk --no-cache add ca-certificates && adduser -D -g '' appuser
WORKDIR /app

COPY --from=builder /out/my_shell /usr/local/bin/my_shell

USER appuser
ENTRYPOINT ["my_shell"]

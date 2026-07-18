# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/auth-scope ./cmd/auth-scope

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
	&& addgroup -S authscope \
	&& adduser -S -G authscope authscope

WORKDIR /app
COPY --from=build /out/auth-scope /app/auth-scope

ENV AUTH_SCOPE_ADDR=:8080
EXPOSE 8080

USER authscope:authscope

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
	CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/auth-scope"]

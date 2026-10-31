FROM golang:1.26-alpine AS build

WORKDIR /src
ARG VERSION=local
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
# Static binary: the runtime image has no libc to link against.
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.buildVersion=${VERSION}" -o /out/apex ./cmd/apex

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 apex \
    && install -d -o apex -g apex -m 700 /data
COPY --from=build /out/apex /usr/local/bin/apex

# Riders' routes and lists, and the key that signs their sessions; without the mount a redeploy
# signs everyone out and forgets everything.
VOLUME /data

ENV APEX_LOG_JSON=1 \
    APEX_DATA_DIR=/data \
    APEX_PORT=8000

EXPOSE 8000
USER apex

HEALTHCHECK --interval=60s --timeout=10s --start-period=15s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8000/healthz >/dev/null || exit 1

ENTRYPOINT ["apex"]

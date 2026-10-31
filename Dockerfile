FROM golang:1.26-alpine AS build

WORKDIR /src
ARG VERSION=local
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
# Static binary: the runtime image has no libc to link against.
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.buildVersion=${VERSION}" -o /out/apex ./cmd/apex

# iOS ignores SVG icons on the home screen, so the PNG is rendered from the same icon.svg, its
# background squared because iOS rounds the corners itself.
FROM alpine:3.21 AS icon
RUN apk add --no-cache rsvg-convert
COPY internal/server/web/icon.svg /icon.svg
RUN echo 'svg > rect:first-of-type { rx: 0; ry: 0 }' > /square.css \
    && rsvg-convert --width 180 --height 180 --stylesheet /square.css /icon.svg -o /apple-touch-icon.png

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 apex \
    && install -d -o apex -g apex -m 700 /data
COPY --from=build /out/apex /usr/local/bin/apex
COPY --from=icon /apple-touch-icon.png /usr/local/share/apex/apple-touch-icon.png

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

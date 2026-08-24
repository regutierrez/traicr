# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=development
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w \
    -X github.com/regutierrez/traicr/internal/version.buildVersion=${VERSION} \
    -X github.com/regutierrez/traicr/internal/version.buildCommit=${COMMIT} \
    -X github.com/regutierrez/traicr/internal/version.buildDate=${BUILD_DATE}" \
    -o /out/traicr-server ./cmd/traicr-server

FROM debian:bookworm-slim

RUN groupadd --system --gid 10001 traicr \
    && useradd --system --uid 10001 --gid traicr --home-dir /nonexistent --shell /usr/sbin/nologin traicr \
    && install -d -o traicr -g traicr -m 0700 /data \
    && install -d -o root -g root -m 0755 /usr/share/traicr/migrations /usr/share/traicr/web/static /usr/share/traicr/web/templates

COPY --from=build /out/traicr-server /usr/local/bin/traicr-server
COPY migrations/ /usr/share/traicr/migrations/
COPY web/static/ /usr/share/traicr/web/static/
COPY web/templates/ /usr/share/traicr/web/templates/

USER 10001:10001
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/traicr-server"]

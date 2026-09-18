# syntax=docker/dockerfile:1

FROM node:22-bookworm AS ui

WORKDIR /src
COPY web/app/package.json web/app/package-lock.json web/app/
COPY web/static/traicr-theme.css web/static/traicr-theme.css
WORKDIR /src/web/app
RUN npm ci
COPY web/app/ ./
RUN npm run build

FROM golang:1.27-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
COPY web ./web
COPY --from=ui /src/web/static/ui ./web/static/ui

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
    && install -d -o traicr -g traicr -m 0700 /data

COPY --from=build /out/traicr-server /usr/local/bin/traicr-server

USER 10001:10001
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/traicr-server"]

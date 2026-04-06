ARG TARGETOS=linux
ARG TARGETARCH=

FROM golang:1.22-alpine AS build

WORKDIR /app

ARG TARGETOS
ARG TARGETARCH

COPY go.mod ./
RUN go mod download

COPY . .

ARG APP_NAME=api

RUN set -eu; \
    goos="${TARGETOS:-linux}"; \
    goarch="${TARGETARCH:-}"; \
    if [ -z "$goarch" ]; then \
        case "$(uname -m)" in \
            aarch64|arm64) goarch=arm64 ;; \
            x86_64|amd64) goarch=amd64 ;; \
            *) echo "unsupported build arch: $(uname -m)" >&2; exit 1 ;; \
        esac; \
    fi; \
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -o /out/app ./cmd/${APP_NAME}

FROM alpine:3.20

WORKDIR /srv

RUN apk add --no-cache ca-certificates ffmpeg su-exec && \
    update-ca-certificates

RUN adduser -D -u 10001 appuser

COPY --from=build /out/app /usr/local/bin/app
COPY --from=build /app/db /srv/db
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod +x /usr/local/bin/docker-entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

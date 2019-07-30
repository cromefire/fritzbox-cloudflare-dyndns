FROM golang:alpine as server_build

# Add build deps
RUN apk add --update gcc g++ git

COPY go.mod /appbuild/

RUN set -ex \
    && cd /appbuild \
    && go mod download

COPY ./ /appbuild

RUN set -ex \
    && go version \
    && cd /appbuild \
    && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o server

# Build deployable server
FROM alpine:latest

ENV FRITZBOX_ENDPOINT_URL ${FRITZBOX_ENDPOINT_URL:-http://fritz.box:49000}
ENV FRITZBOX_ENDPOINT_TIMEOUT ${FRITZBOX_ENDPOINT_TIMEOUT:-30s}

ENV DYNDNS_SERVER_BIND ${DYNDNS_SERVER_BIND:-:8080}
ENV DYNDNS_SERVER_USERNAME ${DYNDNS_SERVER_USERNAME}
ENV DYNDNS_SERVER_PASSWORD ${DYNDNS_SERVER_PASSWORD}

ENV CLOUDFLARE_API_EMAIL ""
ENV CLOUDFLARE_API_KEY ""

ENV CLOUDFLARE_ZONES_IPV4 ""
ENV CLOUDFLARE_ZONES_IPV6 ""

WORKDIR /app

RUN set -ex \
    && apk add --update --no-cache ca-certificates tzdata \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

COPY --from=server_build /appbuild/server /app/server

EXPOSE 8080

CMD ["./server"]

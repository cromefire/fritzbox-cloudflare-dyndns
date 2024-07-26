FROM golang:1.22-alpine AS server_build

WORKDIR /appbuild

ARG GOARCH

COPY go.mod go.sum /appbuild/

COPY ./ /appbuild

RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/go/pkg/mod CGO_ENABLED=0 GOOS=linux go build -o fritzbox-cloudflare-dyndns

# Build deployable server
FROM gcr.io/distroless/static:debug

ENV FRITZBOX_ENDPOINT_URL="http://fritz.box:49000" \
    FRITZBOX_ENDPOINT_TIMEOUT="30s" \
    DYNDNS_SERVER_BIND=":8080" \
    DYNDNS_SERVER_USERNAME="" \
    DYNDNS_SERVER_PASSWORD="" \
    CLOUDFLARE_API_EMAIL="" \
    CLOUDFLARE_API_KEY="" \
    CLOUDFLARE_ZONES_IPV4="" \
    CLOUDFLARE_ZONES_IPV6="" \
    DEVICE_LOCAL_ADDRESS_IPV6=""

WORKDIR /app

COPY --from=server_build /appbuild/fritzbox-cloudflare-dyndns /app/fritzbox-cloudflare-dyndns

EXPOSE 8080

ENTRYPOINT ["./fritzbox-cloudflare-dyndns"]

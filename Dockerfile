FROM --platform=$BUILDPLATFORM golang:latest AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-extldflags=-static" -o gogopher ./src

FROM debian:bookworm-slim
COPY --from=build /app/gogopher gogopher
COPY .env .env
COPY firewall-config.json firewall-config.json
COPY file-access-config.json file-access-config.json
COPY gopher-root /gopher-root
ENV TITLE="Wes C's Gopher Hole" \
    HOSTNAME=localhost \
    HOST_BIND_IP=0.0.0.0 \
    PORT=70 \
    GOPHER_ROOT=/gopher-root \
    FIREWALL_CONFIG_FILE=firewall-config.json \
    FILE_ACCESS_CONFIG_FILE=file-access-config.json \
    REQUEST_TIMEOUT_SECONDS=30 \
    REQUEST_MAXIMUM_BYTES=1024
VOLUME /gopher-root
EXPOSE 70
ENTRYPOINT ["/gogopher"]

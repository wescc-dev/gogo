FROM --platform=$BUILDPLATFORM golang:latest AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -ldflags="-extldflags=-static" -o gogopher ./src

FROM debian:bookworm-slim
COPY --from=build /app/gogopher gogopher
COPY firewall-config.json firewall-config.json
COPY gopher-root /gopher-root
ENV TITLE="Wes C's Gopher Server" \
    HOSTNAME=localhost \
    HOST_BIND_IP=0.0.0.0 \
    PORT=70 \
    GOPHER_ROOT=/gopher-root \
    FIREWALL_CONFIG_FILE=firewall-config.json \
    IDLE_TIMEOUT_SECONDS=10 \
    READWRITE_TIMEOUT_SECONDS=30
VOLUME /gopher-root
EXPOSE 70
ENTRYPOINT ["/gogopher"]

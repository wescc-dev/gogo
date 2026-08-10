FROM --platform=$BUILDPLATFORM golang:latest AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -ldflags="-extldflags=-static" -o gogopher

FROM debian:bookworm-slim
COPY --from=build /src/gogopher gogopher
COPY gopher-root /gopher-root
ENV TITLE="Wes C's Gopher Server" \
    HOST=localhost \
    HOST_BIND_IP=0.0.0.0 \
    PORT=70 \
    GOPHER_ROOT=/gopher-root
VOLUME /gopher-root
EXPOSE 70
ENTRYPOINT ["/gogopher"]

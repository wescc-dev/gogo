FROM --platform=$BUILDPLATFORM golang:latest AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -ldflags="-extldflags=-static" -o gogopher

FROM ubuntu:latest
COPY --from=build /src/gogopher gogopher
COPY .env .
COPY /gopher-root gopherroot
EXPOSE 70
ENTRYPOINT ["/gogopher"]

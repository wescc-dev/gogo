docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --no-cache \
  -t dbppgpmdtacr/gogopher:latest \
  --load \
  .

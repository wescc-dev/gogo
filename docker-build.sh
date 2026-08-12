docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t dbppgpmdtacr/gogopher:latest \
  --push \
  .

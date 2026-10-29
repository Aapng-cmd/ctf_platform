FROM golang:1.27.3-bookworm AS build
WORKDIR /src
COPY . .
# vendor is the module source. -mod=vendor keeps the build off the proxy.
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /out/ctfd ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata docker.io docker-compose && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/ctfd /app/ctfd
COPY web /app/web
RUN mkdir -p /app/data/uploads
ENV PORT=8000
EXPOSE 8000
CMD ["/app/ctfd"]

FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /backend ./cmd

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /backend /usr/local/bin/backend
WORKDIR /app
# Миграции применяются сами при старте (postgre.Migrate), админ сидится из ADMIN_*.
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/backend"]

FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /agent ./cmd

FROM ubuntu:22.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /agent /usr/local/bin/agent
VOLUME ["/data"]
ENV SQLITE_PATH=/data/agent.db
ENV WORKSPACE_DIR=/data/workspace
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/agent"]

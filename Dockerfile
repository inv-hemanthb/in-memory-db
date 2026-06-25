FROM golang:1.23 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 go build -o /out/in-memory-db ./cmd/in-memory-db
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 go build -o /out/seed ./cmd/seed

FROM debian:bookworm-slim AS kv

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/in-memory-db /in-memory-db

EXPOSE 55555
CMD ["/in-memory-db"]

FROM debian:bookworm-slim AS api

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/api /out/migrate /out/seed ./
COPY go.mod go.sum ./
COPY web ./web
COPY migrations ./migrations

EXPOSE 8080
CMD ["./api"]

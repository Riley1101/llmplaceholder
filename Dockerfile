FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -tags musl -ldflags="-s -w" -o server ./cmd/server

# ─────────────────────────────────────────────────────────────────────────────

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata libgcc

WORKDIR /app

COPY --from=builder /build/server .
COPY frontend/ ./frontend/
COPY data/tenants/ ./seeds/

RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./server"]

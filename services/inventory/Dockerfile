FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY gen/ gen/
COPY services/inventory/ services/inventory/

RUN CGO_ENABLED=0 GOOS=linux go build -o /inventory-service ./services/inventory/cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /inventory-service /app/inventory-service

EXPOSE 50053

ENTRYPOINT ["/app/inventory-service"]

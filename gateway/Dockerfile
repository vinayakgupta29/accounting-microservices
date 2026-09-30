FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY gen/ gen/
COPY gateway/ gateway/

RUN CGO_ENABLED=0 GOOS=linux go build -o /api-gateway ./gateway/cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /api-gateway /app/api-gateway

EXPOSE 8080

ENTRYPOINT ["/app/api-gateway"]

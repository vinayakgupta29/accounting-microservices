FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY gen/ gen/
COPY services/statement/ services/statement/

RUN CGO_ENABLED=0 GOOS=linux go build -o /statement-service ./services/statement/cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /statement-service /app/statement-service

EXPOSE 50055

ENTRYPOINT ["/app/statement-service"]

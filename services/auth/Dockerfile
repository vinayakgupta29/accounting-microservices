FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY gen/ gen/
COPY services/auth/ services/auth/

RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-service ./services/auth/cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /auth-service /app/auth-service

EXPOSE 50051

ENTRYPOINT ["/app/auth-service"]

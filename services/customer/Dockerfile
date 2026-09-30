FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY gen/ gen/
COPY services/customer/ services/customer/

RUN CGO_ENABLED=0 GOOS=linux go build -o /customer-service ./services/customer/cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /customer-service /app/customer-service

EXPOSE 50052

ENTRYPOINT ["/app/customer-service"]

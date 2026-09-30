FROM rust:1.80-alpine AS builder

WORKDIR /usr/src/app

RUN apk add --no-cache musl-dev protobuf-dev

COPY proto/ ../../proto/
COPY services/invoicing/ ./

RUN cargo build --release

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /usr/src/app/target/release/invoicing-service /app/invoicing-service

EXPOSE 50054

ENTRYPOINT ["/app/invoicing-service"]

FROM rust:1.87.0-alpine3.21 AS builder

WORKDIR /build

RUN apk add --no-cache musl-dev

COPY . .
RUN cargo build --release

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /build/target/release/flare .
RUN wget https://www.eff.org/files/2016/07/18/eff_large_wordlist.txt

EXPOSE 8080

ENTRYPOINT ["./flare"]

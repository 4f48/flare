FROM golang:1.24.2-alpine3.21 AS builder

WORKDIR /build

COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o flare

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /build/flare .
RUN wget https://www.eff.org/files/2016/07/18/eff_large_wordlist.txt

EXPOSE 8080

CMD ["./flare"]

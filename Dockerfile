FROM golang AS builder

WORKDIR /src
COPY . .
ENV CGO_ENABLED=0
RUN go build -v ./cmd/nmea-collect

FROM alpine

EXPOSE 2000/tcp 2000/udp 9140/tcp

COPY --from=builder /src/nmea-collect /bin/nmea-collect

ENTRYPOINT ["/bin/nmea-collect", "serve", "--input-udp-listen=2000"]


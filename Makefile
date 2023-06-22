all:	test bin nmea-collect nmea-collect-linux-arm64 summarize-gpx udp-proxy-linux-mips

.PHONY: test
test:
	@go test ./...

bin:
	@mkdir bin

.PHONY: nmea-collect
nmea-collect:
	@go build -v -o bin/nmea-collect ./cmd/nmea-collect

.PHONY: nmea-collect-linux-arm64
nmea-collect-linux-arm64:
	@GOOS=linux GOARCH=arm64 go build -v -o bin/nmea-collect-linux-arm64 ./cmd/nmea-collect

.PHONY: summarize-gpx
summarize-gpx:
	@go build -v -o bin/summarize-gpx ./cmd/summarize-gpx

.PHONY: udp-proxy-linux-mips
udp-proxy-linux-mips:
	@GOOS=linux GOARCH=mips GOMIPS=softfloat go build -v -ldflags '-w -s' -o bin/udp-proxy-linux-mips ./cmd/udp-proxy

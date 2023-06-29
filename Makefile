all:	test bin nmea-collect-linux-arm64

.PHONY: test
test:
	@go test ./...

.PHONY: bin
bin:
	@mkdir -p bin
	@go build -v -o bin ./cmd/...

.PHONY: nmea-collect-linux-arm64
nmea-collect-linux-arm64:
	@GOOS=linux GOARCH=arm64 go build -v -o bin/nmea-collect-linux-arm64 ./cmd/nmea-collect

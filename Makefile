all:	test bin nmea-collect nmea-collect-linux-arm summarize-gpx

.PHONY: test
test:
	@go test ./...

bin:
	@mkdir bin

.PHONY: nmea-collect
nmea-collect:
	@go build -v -o bin/nmea-collect ./cmd/nmea-collect

.PHONY: nmea-collect-linux-arm
nmea-collect-linux-arm:
	@GOOS=linux GOARCH=arm go build -v -o bin/nmea-collect-linux-arm ./cmd/nmea-collect

.PHONY: summarize-gpx
summarize-gpx:
	@go build -v -o bin/summarize-gpx ./cmd/summarize-gpx

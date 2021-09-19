all:	bin bin/nmea-collect bin/nmea-collect-linux-arm

bin:
	@mkdir bin

bin/nmea-collect:
	@go build -v -o bin/nmea-collect ./cmd/nmea-collect

bin/nmea-collect-linux-arm:
	@GOOS=linux GOARCH=arm go build -v -o bin/nmea-collect-linux-arm ./cmd/nmea-collect

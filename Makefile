all:	bin nmea-collect nmea-collect-linux-arm summarize-gpx

bin:
	@mkdir bin

nmea-collect:
	@go build -v -o bin/nmea-collect ./cmd/nmea-collect

nmea-collect-linux-arm:
	@GOOS=linux GOARCH=arm go build -v -o bin/nmea-collect-linux-arm ./cmd/nmea-collect

summarize-gpx:
	@go build -v -o bin/summarize-gpx ./cmd/summarize-gpx

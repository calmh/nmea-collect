# nmea-collect

A small tool to collect NMEA raw data and track logs.

```
Usage: nmea-collect serve

Process incoming NMEA data

Flags:
  -h, --help    Show context-sensitive help.

Input
  --input-tcp-connect=ADDR,...    TCP connect input addresses (e.g.,
                                  172.16.1.2:2000)
  --input-udp-listen=PORT,...     UDP broadcast input listen ports (e.g., 2000)
  --input-http-listen=PORT,...    HTTP input listen ports (e.g., 8080)
  --input-serial=DEV,...          Serial port inputs (e.g., /dev/ttyS0)
  --input-stdin                   Read NMEA from standard input

UDP output
  --forward-udp-all=ADDR,...    UDP output destination address (all NMEA)
  --forward-udp-all-max-packet-size=1472
                                Maximum UDP payload size (all NMEA)
  --forward-udp-all-max-delay=1s
                                Maximum UDP buffer delay (all NMEA)
  --forward-ais-udp=ADDR,...    UDP output destination address (AIS only)
  --forward-ais-udp-max-packet-size=1472
                                Maximum UDP payload size (AIS only)
  --forward-ais-udp-max-delay=10s
                                Maximum UDP buffer delay (AIS only)

TCP output
  --forward-all-tcp-listen=ADDR    TCP listen address (all NMEA)
  --forward-ais-tcp-listen=ADDR    TCP listen address (AIS only)

GPX File Output
  --output-gpx-pattern="track-20060102-150405.gpx"
      File naming pattern, see https://golang.org/pkg/time/#Time.Format
  --output-gpx-sample-interval=10s
      Time between track points
  --output-gpx-moving-distance=25
      Minimum travel in time window to consider us moving (meters)
  --output-gpx-start-time-window=1m
      Movement time window for starting track
  --output-gpx-stop-time-window=5m
      Movement time window before ending track

Raw NMEA File Output
  --output-raw-pattern="nmea-raw.20060102-150405.gz"
                                  File naming pattern, see
                                  https://golang.org/pkg/time/#Time.Format
  --output-raw-buffer-size=131072
                                  Write buffer for output file
  --output-raw-uncompressed       Write uncompressed NMEA (default is gzipped)
  --output-raw-time-window=24h    How often to create a new raw file
  --output-raw-flush-interval=5m
                                  How often to flush raw data to disk

Metrics
  --prometheus-metrics-listen=ADDR
      HTTP listen address for Prometheus metrics endpoint
```

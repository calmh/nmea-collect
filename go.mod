module calmh.dev/nmea-collect

go 1.20

require (
	github.com/BertoldVdb/go-ais v0.1.0
	github.com/adrianmo/go-nmea v1.8.0
	github.com/alecthomas/kong v0.7.1
	github.com/lmittmann/tint v0.3.4
	github.com/mattn/go-isatty v0.0.19
	github.com/prometheus/client_golang v1.16.0
	github.com/thejerf/suture/v4 v4.0.2
	golang.org/x/exp v0.0.0-20230626212559-97b1e661b5df
)

replace github.com/adrianmo/go-nmea => github.com/calmh/go-nmea v1.8.1-0.20230624051950-2e4c023fe89a

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	golang.org/x/sys v0.8.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
)

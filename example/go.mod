module github.com/polarsignals/rprof/example

go 1.22.1

require github.com/polarsignals/rprof v0.0.0-20240701160231-adc1026976aa

require (
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)

replace github.com/polarsignals/rprof => ../

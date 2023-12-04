package instance

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Monitoring interface {
	EventV3() EventV3
	Register(prometheus.Registerer)
}

type EventV3 struct {
	TotalConnections               prometheus.Histogram
	TotalConnectionDurationSeconds prometheus.Histogram
	CurrentConnections             prometheus.Gauge
	CurrentEventStreams            prometheus.Gauge
	CurrentWebSockets              prometheus.Gauge
	Heartbeats                     prometheus.Histogram
	Dispatches                     prometheus.Histogram
}

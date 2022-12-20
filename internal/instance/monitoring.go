package instance

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Monitoring interface {
	EventV1() EventV1
	EventV3() EventV3
	Register(prometheus.Registerer)
}

type EventV1 struct {
	ChannelEmotes EventV1ChannelEmotes
}

type EventV3 struct {
	TotalConnections               prometheus.Histogram
	TotalConnectionDurationSeconds prometheus.Histogram
	CurrentConnections             prometheus.Gauge
}

type EventV1ChannelEmotes struct {
	TotalConnections               prometheus.Histogram
	TotalConnectionDurationSeconds prometheus.Histogram
	CurrentConnections             prometheus.Gauge
}

package instance

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Monitoring interface {
	EventV1() EventV1
	Register(prometheus.Registerer)
}

type EventV1 struct {
	ChannelEmotes EventV1ChannelEmotes
}

type EventV1ChannelEmotes struct {
	TotalConnections               prometheus.Histogram
	TotalConnectionDurationSeconds prometheus.Histogram
	CurrentConnections             prometheus.Gauge
}

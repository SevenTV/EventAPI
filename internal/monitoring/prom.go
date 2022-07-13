package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/seventv/eventapi/internal/configure"
	"github.com/seventv/eventapi/internal/global"
	"github.com/seventv/eventapi/internal/instance"
)

type mon struct {
	eventv1 instance.EventV1
}

func (m *mon) EventV1() instance.EventV1 {
	return m.eventv1
}

func (m *mon) Register(r prometheus.Registerer) {
	r.MustRegister(
		// v1 channel-emotes
		m.eventv1.ChannelEmotes.TotalConnections,
		m.eventv1.ChannelEmotes.TotalConnectionDurationSeconds,
		m.eventv1.ChannelEmotes.CurrentConnections,
	)
}

func labelsFromKeyValue(kv []configure.KeyValue) prometheus.Labels {
	mp := prometheus.Labels{}

	for _, v := range kv {
		mp[v.Key] = v.Value
	}

	return mp
}

func NewPrometheus(gCtx global.Context) instance.Monitoring {
	return &mon{
		eventv1: instance.EventV1{
			ChannelEmotes: instance.EventV1ChannelEmotes{
				TotalConnections: prometheus.NewHistogram(prometheus.HistogramOpts{
					Name:        "events_total_connections",
					ConstLabels: labelsFromKeyValue(gCtx.Config().Monitoring.Labels),
					Help:        "The total number of connections",
				}),
				TotalConnectionDurationSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
					Name:        "events_total_connection_duration_seconds",
					ConstLabels: labelsFromKeyValue(gCtx.Config().Monitoring.Labels),
					Help:        "The total number of seconds used on connections",
				}),
				CurrentConnections: prometheus.NewGauge(prometheus.GaugeOpts{
					Name:        "events_current_connections",
					ConstLabels: labelsFromKeyValue(gCtx.Config().Monitoring.Labels),
					Help:        "The current number of connections",
				}),
			},
		},
	}
}

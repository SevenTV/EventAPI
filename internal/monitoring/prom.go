package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/seventv/eventapi/internal/configure"
	"github.com/seventv/eventapi/internal/global"
	"github.com/seventv/eventapi/internal/instance"
)

type mon struct {
	eventv3 instance.EventV3
}

func (m *mon) EventV3() instance.EventV3 {
	return m.eventv3
}

func (m *mon) Register(r prometheus.Registerer) {
	r.MustRegister(
		// v3
		m.eventv3.TotalConnections,
		m.eventv3.TotalConnectionDurationSeconds,
		m.eventv3.CurrentConnections,
		m.eventv3.CurrentEventStreams,
		m.eventv3.CurrentWebSockets,
		m.eventv3.Heartbeats,
		m.eventv3.Dispatches,
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
		eventv3: instance.EventV3{
			TotalConnections: prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:        "events_v3_total_connections",
				ConstLabels: labelsFromKeyValue(gCtx.Config().Monitoring.Labels),
				Help:        "The total number of connections",
			}),
			TotalConnectionDurationSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:        "events_v3_total_connection_duration_seconds",
				ConstLabels: labelsFromKeyValue(gCtx.Config().Monitoring.Labels),
				Help:        "The total number of seconds used on connections",
			}),
			CurrentConnections: prometheus.NewGauge(prometheus.GaugeOpts{
				Name:        "events_v3_current_connections",
				ConstLabels: labelsFromKeyValue(gCtx.Config().Monitoring.Labels),
				Help:        "The current number of connections",
			}),
			CurrentEventStreams: prometheus.NewGauge(prometheus.GaugeOpts{
				Name:        "events_v3_current_event_streams",
				ConstLabels: labelsFromKeyValue(gCtx.Config().Monitoring.Labels),
				Help:        "The current number of connections via EventStream transport",
			}),
			CurrentWebSockets: prometheus.NewGauge(prometheus.GaugeOpts{
				Name:        "events_v3_current_event_websockets",
				ConstLabels: labelsFromKeyValue(gCtx.Config().Monitoring.Labels),
				Help:        "The current number of connections via WebSocket transport",
			}),
			Heartbeats: prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:        "events_v3_heartbeats",
				ConstLabels: labelsFromKeyValue(gCtx.Config().Monitoring.Labels),
				Help:        "The number of heartbeats sent out to clients",
			}),
			Dispatches: prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:        "events_v3_dispatches",
				ConstLabels: labelsFromKeyValue(gCtx.Config().Monitoring.Labels),
				Help:        "The number of dispatches sent out to clients",
			}),
		},
	}
}

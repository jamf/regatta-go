// Copyright JAMF Software, LLC

package rprom

import (
	"context"

	client "github.com/jamf/regatta-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc/stats"
)

// NewMetrics returns a new Metrics that adds prometheus metrics to the
// registry under the given namespace.
func NewMetrics(namespace string, opts ...Opt) *Metrics {
	return &Metrics{cfg: newCfg(namespace, opts...)}
}

type Metrics struct {
	cfg

	connConnectsTotal    prometheus.Counter
	connDisconnectsTotal prometheus.Counter
	connConnectActive    prometheus.Gauge

	// Write
	writeBytesTotal  prometheus.Counter
	writeErrorsTotal prometheus.Counter
	writeTimeSeconds prometheus.Histogram

	// Read
	readBytesTotal  prometheus.Counter
	readErrorsTotal prometheus.Counter
	readTimeSeconds prometheus.Histogram
}

func (m *Metrics) OnNewClient(_ *client.Client) {
	var (
		factory     = promauto.With(m.cfg.reg)
		namespace   = m.cfg.namespace
		subsystem   = m.cfg.subsystem
		constLabels prometheus.Labels
	)

	getHistogramBuckets := func(h Histogram) []float64 {
		if buckets, ok := m.cfg.histograms[h]; ok && len(buckets) != 0 {
			return buckets
		}
		return m.cfg.defBuckets
	}

	m.connConnectsTotal = factory.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "connects_total",
		Help:        "Total number of connections opened",
	})

	m.connDisconnectsTotal = factory.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "disconnects_total",
		Help:        "Total number of connections closed",
	})

	m.connConnectActive = factory.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "connect_active",
		Help:        "Total number of connections active",
	})

	// Write

	m.writeBytesTotal = factory.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "write_bytes_total",
		Help:        "Total number of bytes written",
	})

	m.writeErrorsTotal = factory.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "write_errors_total",
		Help:        "Total number of write errors",
	})

	m.writeTimeSeconds = factory.NewHistogram(prometheus.HistogramOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "write_time_seconds",
		Help:        "Time spent writing to Regatta",
		Buckets:     getHistogramBuckets(WriteTime),
	})

	// Read

	m.readBytesTotal = factory.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "read_bytes_total",
		Help:        "Total number of bytes read",
	})

	m.readErrorsTotal = factory.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "read_errors_total",
		Help:        "Total number of read errors",
	})

	m.readTimeSeconds = factory.NewHistogram(prometheus.HistogramOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "read_time_seconds",
		Help:        "Time spent reading from Regatta",
		Buckets:     getHistogramBuckets(ReadTime),
	})
}

func (m *Metrics) OnClientClose(_ *client.Client) {
	_ = m.cfg.reg.Unregister(m.connConnectsTotal)
	_ = m.cfg.reg.Unregister(m.connDisconnectsTotal)
	_ = m.cfg.reg.Unregister(m.connConnectActive)
	_ = m.cfg.reg.Unregister(m.writeBytesTotal)
	_ = m.cfg.reg.Unregister(m.writeErrorsTotal)
	_ = m.cfg.reg.Unregister(m.writeTimeSeconds)
	_ = m.cfg.reg.Unregister(m.readBytesTotal)
	_ = m.cfg.reg.Unregister(m.readErrorsTotal)
	_ = m.cfg.reg.Unregister(m.readTimeSeconds)
}

func (m *Metrics) OnHandleConn(_ context.Context, cs stats.ConnStats) {
	switch cs.(type) {
	case *stats.ConnBegin:
		m.connConnectsTotal.Inc()
		m.connConnectActive.Inc()
	case *stats.ConnEnd:
		m.connDisconnectsTotal.Inc()
		m.connConnectActive.Dec()
	}
}

func (m *Metrics) OnHandleRPC(_ context.Context, rs stats.RPCStats) {
	switch st := rs.(type) {
	case *stats.End:
		if st.Client {
			if st.Error != nil {
				m.writeErrorsTotal.Inc()
			}
			m.writeTimeSeconds.Observe(st.EndTime.Sub(st.BeginTime).Seconds())
		} else {
			if st.Error != nil {
				m.readErrorsTotal.Inc()
			}
			m.readTimeSeconds.Observe(st.EndTime.Sub(st.BeginTime).Seconds())
		}
	case *stats.InPayload:
		m.readBytesTotal.Add(float64(st.WireLength))
	case *stats.OutPayload:
		m.writeBytesTotal.Add(float64(st.WireLength))
	}
}

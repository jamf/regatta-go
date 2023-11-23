// Copyright JAMF Software, LLC

package rprom

import (
	"context"
	"time"

	client "github.com/jamf/regatta-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc/stats"
)

// NewMetrics returns a new Metrics that adds prometheus metrics to the
// registry under the given namespace.
func NewMetrics(opts ...Opt) *Metrics {
	return &Metrics{cfg: newCfg(opts...)}
}

type Metrics struct {
	cfg

	connConnectsTotal    prometheus.Counter
	connDisconnectsTotal prometheus.Counter
	connConnectActive    prometheus.Gauge

	// Write
	writeBytesTotal prometheus.Counter

	// Read
	readBytesTotal prometheus.Counter

	// Request
	requestDuration *prometheus.HistogramVec
	requestErrors   *prometheus.CounterVec
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

	// Read

	m.readBytesTotal = factory.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "read_bytes_total",
		Help:        "Total number of bytes read",
	})

	// Request

	m.requestDuration = factory.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "request_duration_seconds",
		Help:        "Time spent executing Regatta KV operation",
		Buckets:     getHistogramBuckets(ReadTime),
	}, []string{"table", "op"})

	m.requestErrors = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
		Name:        "request_errors_total",
		Help:        "Total number of errors while executing KV operation",
	}, []string{"table", "op"})
}

func (m *Metrics) OnClientClose(_ *client.Client) {
	_ = m.cfg.reg.Unregister(m.connConnectsTotal)
	_ = m.cfg.reg.Unregister(m.connDisconnectsTotal)
	_ = m.cfg.reg.Unregister(m.connConnectActive)
	_ = m.cfg.reg.Unregister(m.writeBytesTotal)
	_ = m.cfg.reg.Unregister(m.readBytesTotal)
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
	case *stats.InPayload:
		m.readBytesTotal.Add(float64(st.WireLength))
	case *stats.OutPayload:
		m.writeBytesTotal.Add(float64(st.WireLength))
	}
}

func (m *Metrics) OnKVCall(ctx context.Context, table string, op client.Op, fn client.KvDo) (resp client.OpResponse, err error) {
	start := time.Now()
	defer func() {
		opLabel := getOpLabel(op)
		m.requestDuration.WithLabelValues(table, opLabel).Observe(time.Since(start).Seconds())
		if err != nil {
			m.requestErrors.WithLabelValues(table, opLabel).Inc()
		}
	}()
	return fn(ctx, table, op)
}

func getOpLabel(op client.Op) string {
	opLabel := "unknown"
	switch {
	case op.IsGet():
		opLabel = "get"
	case op.IsPut():
		opLabel = "put"
	case op.IsDelete():
		opLabel = "delete"
	case op.IsTxn():
		opLabel = "txn"
	}
	return opLabel
}

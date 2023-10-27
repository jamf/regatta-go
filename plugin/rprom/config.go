// Copyright JAMF Software, LLC

package rprom

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type cfg struct {
	namespace string
	subsystem string

	reg      prometheus.Registerer
	gatherer prometheus.Gatherer

	withClientLabel bool
	histograms      map[Histogram][]float64
	defBuckets      []float64

	handlerOpts promhttp.HandlerOpts
}

func newCfg(namespace string, opts ...Opt) cfg {
	reg := prometheus.NewRegistry()
	cfg := cfg{
		namespace:  namespace,
		reg:        reg,
		gatherer:   reg,
		defBuckets: DefBuckets,
	}

	for _, opt := range opts {
		opt.apply(&cfg)
	}
	return cfg
}

// Opt is an option to configure Metrics.
type Opt interface {
	apply(*cfg)
}

type opt struct{ fn func(*cfg) }

func (o opt) apply(c *cfg) { o.fn(c) }

// DefBuckets are the default Histogram buckets. The default buckets are
// tailored to broadly measure the kafka timings (in seconds).
var DefBuckets = []float64{0.001, 0.002, 0.004, 0.008, 0.016, 0.032, 0.064, 0.128, 0.256, 0.512, 1.024, 2.048}

// A Histogram is an identifier for a rprom histogram that can be enabled
type Histogram uint8

const (
	ReadTime  Histogram = iota // Enables {ns}_{ss}_read_time_seconds.
	WriteTime                  // Enables {ns}_{ss}_write_time_seconds.
)

// HistogramOpts allows histograms to be enabled with custom buckets
type HistogramOpts struct {
	Enable  Histogram
	Buckets []float64
}

type RegistererGatherer interface {
	prometheus.Registerer
	prometheus.Gatherer
}

// Registry sets the registerer and gatherer to add metrics to, rather than a
// new registry. Use this option if you want to configure both Gatherer and
// Registerer with the same object.
func Registry(rg RegistererGatherer) Opt {
	return opt{func(c *cfg) {
		c.reg = rg
		c.gatherer = rg
	}}
}

// Registerer sets the registerer to add register to, rather than a new registry.
func Registerer(reg prometheus.Registerer) Opt {
	return opt{func(c *cfg) { c.reg = reg }}
}

// Gatherer sets the gatherer to add gather to, rather than a new registry.
func Gatherer(gatherer prometheus.Gatherer) Opt {
	return opt{func(c *cfg) { c.gatherer = gatherer }}
}

// WithClientLabel adds a "cliend_id" label to all metrics.
func WithClientLabel() Opt {
	return opt{func(c *cfg) { c.withClientLabel = true }}
}

// Subsystem sets the subsystem for the rprom metrics, overriding the default
// empty string.
func Subsystem(ss string) Opt {
	return opt{func(c *cfg) { c.subsystem = ss }}
}

// Buckets sets the buckets to be used with Histograms, overriding the default
// of [rprom.DefBuckets]. If custom buckets per histogram is needed,
// HistogramOpts can be used.
func Buckets(buckets []float64) Opt {
	return opt{func(c *cfg) { c.defBuckets = buckets }}
}

// HistogramsFromOpts allows the user full control of what histograms to enable
// and define buckets to be used with each histogram.
//
//	metrics, _ := rprom.NewMetrics(
//	 rprom.HistogramsFromOpts(
//	 	rprom.HistogramOpts{
//	 		Enable:  rprom.ReadWait,
//	 		Buckets: prometheus.LinearBuckets(10, 10, 8),
//	 	},
//	 	rprom.HistogramOpts{
//	 		Enable: rprom.ReadeTime,
//	 		// rprom default bucket will be used
//	 	},
//	 ),
//	)
func HistogramsFromOpts(hs ...HistogramOpts) Opt {
	return opt{func(c *cfg) {
		c.histograms = make(map[Histogram][]float64)
		for _, h := range hs {
			c.histograms[h.Enable] = h.Buckets
		}
	}}
}

// Histograms sets the histograms to be enabled for rprom, overiding the
// default of disabling all histograms.
//
//	metrics, _ := rprom.NewMetrics(
//		rprom.Histograms(
//			rprom.RequestDurationE2E,
//		),
//	)
func Histograms(hs ...Histogram) Opt {
	hos := make([]HistogramOpts, 0)
	for _, h := range hs {
		hos = append(hos, HistogramOpts{Enable: h})
	}
	return HistogramsFromOpts(hos...)
}

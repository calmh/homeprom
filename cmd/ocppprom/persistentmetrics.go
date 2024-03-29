package main

import (
	"encoding/binary"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type persistentMetrics struct {
	db         *leveldb.DB
	collectors map[string]prometheus.Collector
	putCache   map[string]float64
}

func newPersistentMetrics(db *leveldb.DB) *persistentMetrics {
	return &persistentMetrics{
		db:         db,
		collectors: make(map[string]prometheus.Collector),
		putCache:   make(map[string]float64),
	}
}

func (p *persistentMetrics) NewGaugeVec(opts prometheus.GaugeOpts, labels []string) *prometheus.GaugeVec {
	gv := promauto.NewGaugeVec(opts, labels)
	name := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	p.collectors[name] = gv
	p.loadMultiple(name, func(labels []string) adder { return gv.WithLabelValues(labels...) })

	return gv
}

func (p *persistentMetrics) NewCounterVec(opts prometheus.CounterOpts, labels []string) *prometheus.CounterVec {
	gv := promauto.NewCounterVec(opts, labels)
	name := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	p.collectors[name] = gv
	p.loadMultiple(name, func(labels []string) adder { return gv.WithLabelValues(labels...) })

	return gv
}

func (p *persistentMetrics) NewGauge(opts prometheus.GaugeOpts) prometheus.Gauge {
	g := promauto.NewGauge(opts)
	name := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	p.collectors[name] = g
	p.loadSingle(name, g)

	return g
}

func (p *persistentMetrics) NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	g := promauto.NewCounter(opts)
	name := prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	p.collectors[name] = g
	p.loadSingle(name, g)

	return g
}

type adder interface {
	Add(float64) // implemented by both counters and gauges
}

func (p *persistentMetrics) loadSingle(name string, g adder) {
	key := name + "\x00"
	valBytes, err := p.db.Get([]byte(key), nil)
	if err == nil {
		val := math.Float64frombits(binary.BigEndian.Uint64(valBytes))
		slog.Debug("setting", "name", name, "val", val)
		g.Add(val)
	}
}

func (p *persistentMetrics) loadMultiple(name string, gfn func(labels []string) adder) {
	baseKey := name + "\x00"
	it := p.db.NewIterator(util.BytesPrefix([]byte(baseKey)), nil)
	defer it.Release()
	for it.Next() {
		_, labels := p.parseKey(it.Key())
		val := math.Float64frombits(binary.BigEndian.Uint64(it.Value()))
		slog.Debug("setting", "name", name, "labels", labels, "val", val)
		gfn(labels).Add(val)
	}
}

func (p *persistentMetrics) Serve() {
	for range time.NewTicker(15 * time.Second).C {
		ch := make(chan nameWrappedMetric)
		go func() {
			var me io_prometheus_client.Metric
			for m := range ch {
				_ = m.Write(&me)
				var labels []string
				for _, pair := range me.Label {
					labels = append(labels, pair.GetValue())
				}
				_ = p.putFloat64(m.name, labels, me.Gauge.GetValue())
				me.Reset()
			}
		}()
		for key, g := range p.collectors {
			mch := nameWrapMetric(key, ch)
			g.Collect(mch)
			close(mch)
		}
	}
}

type nameWrappedMetric struct {
	name string
	prometheus.Metric
}

func nameWrapMetric(name string, out chan nameWrappedMetric) chan prometheus.Metric {
	ch := make(chan prometheus.Metric)
	go func() {
		for m := range ch {
			out <- nameWrappedMetric{name: name, Metric: m}
		}
	}()
	return ch
}

func (p *persistentMetrics) parseKey(key []byte) (name string, labels []string) {
	name, labelsPart, _ := strings.Cut(string(key), "\x00")
	labels = strings.Split(labelsPart, "\x01")
	return name, labels
}

func (p *persistentMetrics) putFloat64(name string, labels []string, value float64) error {
	key := name + "\x00"
	if len(labels) > 0 {
		key += strings.Join(labels, "\x01")
	}
	if p.putCache[key] == value {
		return nil
	}
	var valBytes [8]byte
	binary.BigEndian.PutUint64(valBytes[:], math.Float64bits(value))
	slog.Debug("storing", "key", key, "val", value)
	if err := p.db.Put([]byte(key), valBytes[:], nil); err != nil {
		return err
	}
	p.putCache[key] = value
	return nil
}

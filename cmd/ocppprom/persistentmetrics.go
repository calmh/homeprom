package main

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type persistentMetrics struct {
	db *leveldb.DB
}

func (p *persistentMetrics) NewGaugeVec(opts prometheus.GaugeOpts, labels []string) *persistentGaugeVec {
	gv := promauto.NewGaugeVec(opts, labels)

	baseKey := fmt.Sprintf("%s_%s_%s\x00", opts.Namespace, opts.Subsystem, opts.Name)
	it := p.db.NewIterator(util.BytesPrefix([]byte(baseKey)), nil)
	defer it.Release()
	for it.Next() {
		key := string(it.Key())
		name, labelsPart, _ := strings.Cut(key, "\x00")
		labels := strings.Split(labelsPart, "\x01")
		val := math.Float64frombits(binary.BigEndian.Uint64(it.Value()))
		slog.Debug("setting", "name", name, "labels", labels, "val", val)
		gv.WithLabelValues(labels...).Set(val)
	}

	return &persistentGaugeVec{pm: p, opts: opts, labels: labels, gv: gv}
}

func (p *persistentMetrics) NewGauge(opts prometheus.GaugeOpts) *persistentGauge {
	g := promauto.NewGauge(opts)

	baseKey := fmt.Sprintf("%s_%s_%s\x00", opts.Namespace, opts.Subsystem, opts.Name)
	valBytes, err := p.db.Get([]byte(baseKey), nil)
	if err == nil {
		val := math.Float64frombits(binary.BigEndian.Uint64(valBytes))
		slog.Debug("setting", "name", baseKey, "val", val)
		g.Set(val)
	}

	return &persistentGauge{pm: p, opts: opts, g: g}
}

type persistentGaugeVec struct {
	pm     *persistentMetrics
	opts   prometheus.GaugeOpts
	labels []string
	gv     *prometheus.GaugeVec
}

func (p *persistentGaugeVec) Set(value float64, labelValues ...string) {
	p.gv.WithLabelValues(labelValues...).Set(value)

	dbKey := fmt.Sprintf("%s_%s_%s\x00%s", p.opts.Namespace, p.opts.Subsystem, p.opts.Name, strings.Join(labelValues, "\x01"))
	var valBytes [8]byte
	binary.BigEndian.PutUint64(valBytes[:], math.Float64bits(value))
	slog.Debug("storing", "key", dbKey, "val", value)
	_ = p.pm.db.Put([]byte(dbKey), valBytes[:], nil)
}

type persistentGauge struct {
	pm   *persistentMetrics
	opts prometheus.GaugeOpts
	g    prometheus.Gauge
}

func (p *persistentGauge) Set(value float64) {
	p.g.Set(value)

	dbKey := fmt.Sprintf("%s_%s_%s\x00", p.opts.Namespace, p.opts.Subsystem, p.opts.Name)
	var valBytes [8]byte
	binary.BigEndian.PutUint64(valBytes[:], math.Float64bits(value))
	slog.Debug("storing", "key", dbKey, "val", value)
	_ = p.pm.db.Put([]byte(dbKey), valBytes[:], nil)
}

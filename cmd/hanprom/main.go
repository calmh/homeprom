package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var gauges = make(map[string]prometheus.Gauge)

func main() {
	addr := flag.String("addr", "localhost:2113", "HAN address")
	listen := flag.String("listen", "0.0.0.0:2115", "HTTP listener address")
	flag.Parse()

	go func() {
		if err := http.ListenAndServe(*listen, promhttp.Handler()); err != nil {
			log.Fatal(err)
		}
	}()

	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}

	framer := NewFramer(conn)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(time.Minute)); err != nil {
			log.Fatal(err)
		}
		frame, err := framer.Read()
		if err != nil {
			log.Fatal(err)
		}

		for _, d := range frame.Data {
			val, err := Parse(d)
			if err != nil {
				log.Fatal(err)
			}

			if strings.HasPrefix(val.Unit, "kW") {
				val.Value = val.Value * 1000
				val.Unit = val.Unit[1:]
			}

			name := counterName(val)
			gauge, ok := gauges[name]
			if !ok {
				gauge = prometheus.NewGauge(prometheus.GaugeOpts{Name: name})
				prometheus.MustRegister(gauge)
				gauges[name] = gauge
			}
			gauge.Set(val.Value)
		}
	}
}

func counterName(v *Value) string {
	name := sanitizeString(IdentDescr[v.Ident])
	if v.Unit != "" {
		name += "_" + v.Unit
	}
	return "han_" + name
}

func sanitizeString(s string) string {
	// Remove diacritics.
	t := transform.Chain(
		// Split runes with diacritics into base character and mark.
		norm.NFD,
		runes.Remove(runes.Predicate(func(r rune) bool {
			return unicode.Is(unicode.Mn, r) || r > unicode.MaxASCII
		})))
	res, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return strings.ReplaceAll(strings.ToLower(res), " ", "_")
}

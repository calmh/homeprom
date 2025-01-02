package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/alecthomas/kong"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/thejerf/suture/v4"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	gauges    = make(map[string]prometheus.Gauge)
	gaugeVecs = make(map[string]prometheus.GaugeVec)
)

type CLI struct {
	Addr   string `default:"localhost:2113" help:"HAN address"`
	Listen string `default:"0.0.0.0:2115" help:"HTTP listener address"`

	MQTTBroker   string `help:"MQTT broker address" env:"MQTT_BROKER"`
	MQTTUsername string `help:"MQTT username" default:"" env:"MQTT_USERNAME"`
	MQTTPassword string `help:"MQTT password" default:"" env:"MQTT_PASSWORD"`
}

func main() {
	var cli CLI
	kong.Parse(&cli)

	main := suture.NewSimple("main")
	main.ServeBackground(context.Background())

	go func() {
		slog.Info("Listening on HTTP", "address", cli.Listen)
		if err := http.ListenAndServe(cli.Listen, promhttp.Handler()); err != nil {
			slog.Error("Failed to listen", "address", cli.Listen, "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("Dialing HAN", "address", cli.Addr)
	conn, err := net.DialTimeout("tcp", cli.Addr, time.Minute)
	if err != nil {
		slog.Error("Failed to connect", "address", cli.Addr, "error", err)
		os.Exit(1)
	}

	var mqttClient *mqttClient
	if cli.MQTTBroker != "" {
		mqttClient, err = getClient(&cli)
		if err != nil {
			slog.Error("Failed to create MQTT client", "broker", cli.MQTTBroker, "error", err)
			os.Exit(1)
		}
		main.Add(mqttClient)
	}

	framer := NewFramer(conn)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(time.Minute)); err != nil {
			slog.Error("Failed to set read deadline", "error", err)
			os.Exit(1)
		}
		frame, err := framer.Read()
		if err != nil {
			slog.Error("Failed to read frame", "error", err)
			os.Exit(1)
		}

		for _, d := range frame.Data {
			val, err := Parse(d)
			if err != nil {
				slog.Error("Failed to parse data", "error", err)
				os.Exit(1)
			}

			if strings.HasPrefix(val.Unit, "kW") {
				val.Value = val.Value * 1000
				val.Unit = val.Unit[1:]
			}

			switch name, instance := counterName(val); instance {
			case "":
				// Just a gauge
				gauge, ok := gauges[name]
				if !ok {
					gauge = prometheus.NewGauge(prometheus.GaugeOpts{Name: name})
					prometheus.MustRegister(gauge)
					gauges[name] = gauge
				}
				gauge.Set(val.Value)

			default:
				// A phase vector
				gaugeVec, ok := gaugeVecs[name]
				if !ok {
					gaugeVec = *prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name}, []string{"fas"})
					prometheus.MustRegister(gaugeVec)
					gaugeVecs[name] = gaugeVec
				}
				gaugeVec.WithLabelValues(instance).Set(val.Value)
			}

			if mqttClient != nil {
				slog.Debug("Publishing to MQTT", "frame", frame, "value", val)
				mqttClient.Publish(frame, val)
			}
		}
	}
}

var lExp = regexp.MustCompile(`^L[1-3]`)

func counterName(v *Value) (name, instance string) {
	name = IdentDescr[v.Ident]
	if lExp.MatchString(name) {
		instance = name[:2]
		name = "fas_" + name[3:]
	}

	name = sanitizeString(name)
	if v.Unit != "" {
		name += "_" + v.Unit
	}
	name = "han_" + name

	return
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

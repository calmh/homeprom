package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
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

var gauges = make(map[string]prometheus.Gauge)

type CLI struct {
	Addr   string `default:"localhost:2113" help:"HAN address"`
	Listen string `default:"0.0.0.0:2115" help:"HTTP listener address"`

	MQTTBroker   string `help:"MQTT broker address" env:"MQTT_BROKER"`
	MQTTClientID string `help:"MQTT client ID" env:"MQTT_CLIENT_ID"`
	MQTTUsername string `help:"MQTT username" default:"" env:"MQTT_USERNAME"`
	MQTTPassword string `help:"MQTT password" default:"" env:"MQTT_PASSWORD"`
}

func main() {
	var cli CLI
	kong.Parse(&cli)

	main := suture.NewSimple("main")
	go main.ServeBackground(context.Background())

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

			name := counterName(val)
			gauge, ok := gauges[name]
			if !ok {
				gauge = prometheus.NewGauge(prometheus.GaugeOpts{Name: name})
				prometheus.MustRegister(gauge)
				gauges[name] = gauge
			}
			gauge.Set(val.Value)

			if mqttClient != nil {
				slog.Debug("Publishing to MQTT", "frame", frame, "value", val)
				mqttClient.Publish(frame, val)
			}
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

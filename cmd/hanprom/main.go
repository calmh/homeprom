package main

import (
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/alecthomas/kong"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	go func() {
		if err := http.ListenAndServe(cli.Listen, promhttp.Handler()); err != nil {
			log.Fatal(err)
		}
	}()

	conn, err := net.DialTimeout("tcp", cli.Addr, time.Minute)
	if err != nil {
		log.Fatal(err)
	}

	var mqttClient mqtt.Client
	if cli.MQTTBroker != "" {
		mqttClient, err = getClient(&cli)
		if err != nil {
			log.Fatal(err)
		}
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

			if mqttClient != nil {
				publishMQTT(mqttClient, &cli, frame, val)
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

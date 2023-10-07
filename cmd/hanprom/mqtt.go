package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"

	"calmh.dev/hassmqtt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var unitToClass = map[string]string{
	"Wh": "energy",
	"V":  "voltage",
	"A":  "current",
	"W":  "power",
	"VA": "apparent_power",
}

var mqttMetrics = make(map[string]*hassmqtt.Metric)

func getClient(cli *CLI) (mqtt.Client, error) {
	if cli.MQTTClientID == "" {
		hn, _ := os.Hostname()
		home, _ := os.UserHomeDir()
		hf := sha256.New()
		fmt.Fprintf(hf, "%s\n%s\n", hn, home)
		cli.MQTTClientID = fmt.Sprintf("h%x", hf.Sum(nil))[:12]
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cli.MQTTBroker)
	opts.SetClientID(cli.MQTTClientID)
	if cli.MQTTUsername != "" && cli.MQTTPassword != "" {
		opts.SetUsername(cli.MQTTUsername)
		opts.SetPassword(cli.MQTTPassword)
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, err
	}
	return client, nil
}

func publishMQTT(client mqtt.Client, cli *CLI, frame *Frame, val *Value) {
	if cl, ok := unitToClass[val.Unit]; ok {
		id := sanitizeString(IdentDescr[val.Ident])
		metric, ok := mqttMetrics[id]
		if !ok {
			metric = &hassmqtt.Metric{
				Device: &hassmqtt.Device{
					Namespace: "han",
					ClientID:  cli.MQTTClientID,
					ID:        frame.Ident,
					Name:      frame.Ident,
				},
				ID:          id,
				DeviceType:  "sensor",
				DeviceClass: cl,
				Unit:        val.Unit,
				Name:        IdentDescr[val.Ident],
			}
			mqttMetrics[id] = metric
		}
		if err := metric.Publish(client, val.Value); err != nil {
			log.Println("Publish:", err)
		}
	}
}

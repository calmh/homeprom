package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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

type mqttClient struct {
	opts        *mqtt.ClientOptions
	mqttMetrics map[string]*hassmqtt.Metric
	outbox      chan message
}

type message struct {
	frame *Frame
	val   *Value
}

func getClient(cli *CLI) (*mqttClient, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cli.MQTTBroker)
	opts.SetClientID(hassmqtt.ClientID("hanprom"))
	if cli.MQTTUsername != "" && cli.MQTTPassword != "" {
		opts.SetUsername(cli.MQTTUsername)
		opts.SetPassword(cli.MQTTPassword)
	}
	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(5 * time.Second)
	opts.SetWriteTimeout(5 * time.Second)

	return &mqttClient{
		opts:        opts,
		mqttMetrics: make(map[string]*hassmqtt.Metric),
		outbox:      make(chan message, 100),
	}, nil
}

func (c *mqttClient) Serve(ctx context.Context) error {
	slog.Info("Connecting to MQTT", "broker", c.opts.Servers[0], "client_id", c.opts.ClientID)
	client := mqtt.NewClient(c.opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		slog.Error("Failed to connect to MQTT", "broker", c.opts.Servers[0], "client_id", c.opts.ClientID, "error", err)
		return fmt.Errorf("failed to connect: %s", err) // intentionally not wrapped
	}
	defer client.Disconnect(250)

	for msg := range c.outbox {
		if err := c.publish(client, msg.frame, msg.val); err != nil {
			slog.Error("Failed to publish to MQTT", "broker", c.opts.Servers[0], "client_id", c.opts.ClientID, "error", err)
			return fmt.Errorf("failed to publish: %s", err) // intentionally not wrapped
		}
	}
	return nil
}

func (c *mqttClient) Publish(frame *Frame, val *Value) {
	select {
	case c.outbox <- message{frame, val}:
	default:
	}
}

func (c *mqttClient) publish(client mqtt.Client, frame *Frame, val *Value) error {
	if cl, ok := unitToClass[val.Unit]; ok {
		id := sanitizeString(IdentDescr[val.Ident])
		metric, ok := c.mqttMetrics[id]
		if !ok {
			metric = &hassmqtt.Metric{
				Device: &hassmqtt.Device{
					Namespace: "han",
					ClientID:  c.opts.ClientID,
					ID:        frame.Ident,
					Name:      frame.Ident,
				},
				ID:          id,
				DeviceType:  "sensor",
				DeviceClass: cl,
				Unit:        val.Unit,
				Name:        IdentDescr[val.Ident],
			}
			if val.Ident.Cumulative == 8 {
				metric.StateClass = "total"
			}
			c.mqttMetrics[id] = metric
		}
		return metric.Publish(client, val.Value)
	}
	return nil
}

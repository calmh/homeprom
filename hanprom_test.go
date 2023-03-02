package main

import (
	"strings"
	"testing"
)

const sampleData = "/ELL5\x5c253833635_A\r\n\r\n" +
	"0-0:1.0.0(210217184019W)\r\n" +
	"1-0:1.8.0(00006678.394*kWh)\r\n" +
	"1-0:2.8.0(00000000.000*kWh)\r\n" +
	"1-0:3.8.0(00000021.988*kvarh)\r\n" +
	"1-0:4.8.0(00001020.971*kvarh)\r\n" +
	"1-0:1.7.0(0001.727*kW)\r\n" +
	"1-0:2.7.0(0000.000*kW)\r\n" +
	"1-0:3.7.0(0000.000*kvar)\r\n" +
	"1-0:4.7.0(0000.309*kvar)\r\n" +
	"1-0:21.7.0(0001.023*kW)\r\n" +
	"1-0:41.7.0(0000.350*kW)\r\n" +
	"1-0:61.7.0(0000.353*kW)\r\n" +
	"1-0:22.7.0(0000.000*kW)\r\n" +
	"1-0:42.7.0(0000.000*kW)\r\n" +
	"1-0:62.7.0(0000.000*kW)\r\n" +
	"1-0:23.7.0(0000.000*kvar)\r\n" +
	"1-0:43.7.0(0000.000*kvar)\r\n" +
	"1-0:63.7.0(0000.000*kvar)\r\n" +
	"1-0:24.7.0(0000.009*kvar)\r\n" +
	"1-0:44.7.0(0000.161*kvar)\r\n" +
	"1-0:64.7.0(0000.138*kvar)\r\n" +
	"1-0:32.7.0(240.3*V)\r\n" +
	"1-0:52.7.0(240.1*V)\r\n" +
	"1-0:72.7.0(241.3*V)\r\n" +
	"1-0:31.7.0(004.2*A)\r\n" +
	"1-0:51.7.0(001.6*A)\r\n" +
	"1-0:71.7.0(001.7*A)\r\n" +
	"!\x79\x45"

func TestFramerParser(t *testing.T) {
	br := strings.NewReader(sampleData)
	framer := NewFramer(br)
	frame, err := framer.Read()
	if err != nil {
		t.Fatal(err)
	}
	if frame.FlagID != "ELL" {
		t.Error("invalid flag id", frame.FlagID)
	}
	if frame.BaudRate != "5" {
		t.Error("invalid baud rate", frame.BaudRate)
	}
	if frame.Ident != "\x5c253833635_A" {
		t.Error("invalid ident", frame.Ident)
	}
	if len(frame.Data) != 27 {
		t.Error("invalid data length", len(frame.Data))
	}
	if frame.Checksum != 0x7945 {
		t.Error("invalid checksum", frame.Checksum)
	}

	for _, d := range frame.Data {
		val, err := Parse(d)
		if err != nil {
			t.Error(d, err)
			continue
		}
		t.Log(d, val)
	}
}

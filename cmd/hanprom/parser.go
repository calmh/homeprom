package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Ident struct {
	Medium      int
	Channel     int
	Measurement int
	Cumulative  int
	Tariff      int
	Period      int
}

type Value struct {
	Ident Ident
	Value float64
	Unit  string
}

var DateTimeIdent = Ident{0, 0, 1, 0, 0, 0}

var IdentDescr = map[Ident]string{
	DateTimeIdent:       "Datum och tid",
	{1, 0, 1, 8, 0, 0}:  "Mätarställning Aktiv Energi Uttag",
	{1, 0, 2, 8, 0, 0}:  "Mätarställning Aktiv Energi Inmatning",
	{1, 0, 3, 8, 0, 0}:  "Mätarställning Reaktiv Energi Uttag",
	{1, 0, 4, 8, 0, 0}:  "Mätarställning Reaktiv Energi Inmatning",
	{1, 0, 1, 7, 0, 0}:  "Aktiv Effekt Uttag",
	{1, 0, 2, 7, 0, 0}:  "Aktiv Effekt Inmatning",
	{1, 0, 3, 7, 0, 0}:  "Reaktiv Effekt Uttag",
	{1, 0, 4, 7, 0, 0}:  "Reaktiv Effekt Inmatning",
	{1, 0, 21, 7, 0, 0}: "L1 Aktiv Effekt Uttag",
	{1, 0, 22, 7, 0, 0}: "L1 Aktiv Effekt Inmatning",
	{1, 0, 41, 7, 0, 0}: "L2 Aktiv Effekt Uttag",
	{1, 0, 42, 7, 0, 0}: "L2 Aktiv Effekt Inmatning",
	{1, 0, 61, 7, 0, 0}: "L3 Aktiv Effekt Uttag",
	{1, 0, 62, 7, 0, 0}: "L3 Aktiv Effekt Inmatning",
	{1, 0, 23, 7, 0, 0}: "L1 Reaktiv Effekt Uttag",
	{1, 0, 24, 7, 0, 0}: "L1 Reaktiv Effekt Inmatning",
	{1, 0, 43, 7, 0, 0}: "L2 Reaktiv Effekt Uttag",
	{1, 0, 44, 7, 0, 0}: "L2 Reaktiv Effekt Inmatning",
	{1, 0, 63, 7, 0, 0}: "L3 Reaktiv Effekt Uttag",
	{1, 0, 64, 7, 0, 0}: "L3 Reaktiv Effekt Inmatning",
	{1, 0, 32, 7, 0, 0}: "L1 Fasspänning",
	{1, 0, 52, 7, 0, 0}: "L2 Fasspänning",
	{1, 0, 72, 7, 0, 0}: "L3 Fasspänning",
	{1, 0, 31, 7, 0, 0}: "L1 Fasström",
	{1, 0, 51, 7, 0, 0}: "L2 Fasström",
	{1, 0, 71, 7, 0, 0}: "L3 Fasström",
}

var seLoc, _ = time.LoadLocation("Europe/Stockholm")

func Parse(line string) (*Value, error) {
	ident, val, ok := strings.Cut(line, "(")
	if !ok {
		return nil, errors.New("invalid line (no ident)")
	}

	var v Value
	var err error

	part, rest, ok := strings.Cut(ident, "-")
	if !ok {
		return nil, errors.New("invalid line (A)")
	}
	v.Ident.Medium, err = strconv.Atoi(part)
	if err != nil {
		return nil, fmt.Errorf("invalid medium: %s", part)
	}

	part, rest, ok = strings.Cut(rest, ":")
	if !ok {
		return nil, errors.New("invalid line (B)")
	}
	v.Ident.Channel, err = strconv.Atoi(part)
	if err != nil {
		return nil, fmt.Errorf("invalid channel: %s", part)
	}

	part, rest, ok = strings.Cut(rest, ".")
	if !ok {
		return nil, errors.New("invalid line (C)")
	}
	v.Ident.Measurement, err = strconv.Atoi(part)
	if err != nil {
		return nil, fmt.Errorf("invalid measurement: %s", part)
	}

	part, rest, ok = strings.Cut(rest, ".")
	if !ok {
		return nil, errors.New("invalid line (D)")
	}
	v.Ident.Cumulative, err = strconv.Atoi(part)
	if err != nil {
		return nil, fmt.Errorf("invalid cumulative: %s", part)
	}

	part, rest, _ = strings.Cut(rest, ".")
	v.Ident.Tariff, err = strconv.Atoi(part)
	if err != nil {
		return nil, fmt.Errorf("invalid tariff: %s", part)
	}
	v.Ident.Period, _ = strconv.Atoi(rest)

	if v.Ident == DateTimeIdent {
		// The date stamp. "Svensk normaltid."
		date := strings.TrimRight(val, "ABCDEFGHIJKLMNOPQRSTUVWXYZ)")
		ts, err := time.ParseInLocation("060102150405", date, seLoc)
		if err != nil {
			return nil, fmt.Errorf("invalid date: %q", date)
		}
		v.Value = float64(ts.Unix())
	} else {
		part, rest, ok = strings.Cut(val, "*")
		if !ok {
			return nil, errors.New("invalid line (unit)")
		}
		v.Value, err = strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value: %s", part)
		}
		v.Unit = strings.TrimRight(rest, ")")
	}

	return &v, nil
}

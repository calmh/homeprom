package main

import (
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/aliml92/ocpp"
	v16 "github.com/aliml92/ocpp/v16"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/syndtr/goleveldb/leveldb"
)

var csms *ocpp.Server

var (
	chargerInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "charger_info",
	}, []string{"vendor", "model", "serial"})
	chargerState         prometheus.Gauge
	chargerMeterValue    *prometheus.GaugeVec
	chargerLastHeartbeat prometheus.Gauge
)

var chargerStates = []string{"", "Available", "Preparing", "Charging", "SuspendedEV", "SuspendedEVSE", "Finishing", "Reserved", "Unavailable", "Faulted"}

type CLI struct {
	HTTPListen            string `default:":2118" env:"HTTP_LISTEN"`
	OCPPListen            string `default:":8999" env:"OCPP_LISTEN"`
	SampleIntervalS       int    `default:"15" env:"SAMPLE_INTERVAL_S"`
	ClockAlignedIntervalS int    `default:"900" env:"CLOCK_ALIGNED_INTERVAL_S"`
	Measurands            string `default:"Energy.Active.Import.Register" env:"MEASURANDS"`
	MinStatusDurationS    int    `default:"30" env:"MIN_STATUS_DURATION_S"`
	StateDatabase         string `default:"~/ocppprom.db" env:"STATE_DATABASE" type:"path"`
	Debug                 bool   `env:"DEBUG"`
}

func main() {
	var cli CLI
	kong.Parse(&cli)

	level := slog.LevelInfo
	if cli.Debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      level,
			TimeFormat: time.TimeOnly,
			NoColor:    !isatty.IsTerminal(os.Stdout.Fd()),
		}),
	))

	db, err := leveldb.OpenFile(cli.StateDatabase, nil)
	if err != nil {
		slog.Error("Failed to open database", "err", err)
		os.Exit(1)
	}

	pm := newPersistentMetrics(db)
	go pm.Serve()
	chargerState = pm.NewGauge(prometheus.GaugeOpts{Name: "charger_state"})
	chargerMeterValue = pm.NewGaugeVec(prometheus.GaugeOpts{Name: "charger_meter_value"}, []string{"measurand"})
	chargerLastHeartbeat = pm.NewGauge(prometheus.GaugeOpts{Name: "charger_last_heartbeat"})

	slog.Info("Starting", "ocpp", cli.OCPPListen, "http", cli.HTTPListen)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(cli.HTTPListen, nil); err != nil {
			slog.Error("Failed to listen for metrics", "err", err)
		}
	}()

	csms = ocpp.NewServer()

	csms.AddSubProtocol("ocpp1.6")
	csms.SetCheckOriginHandler(func(r *http.Request) bool { return true })
	csms.SetPreUpgradeHandler(customPreUpgradeHandler)
	csms.SetCallQueueSize(32)

	csms.On("BootNotification", cast(bootNotification))
	csms.On("Authorize", cast(authorize))
	csms.On("DataTransfer", cast(dataTransfer))
	csms.On("DiagnosticsStatusNotification", cast(diagnosticsStatusNotification))
	csms.On("FirmwareStatusNotification", cast(firmwareStatusNotification))
	csms.On("Heartbeat", cast(heartbeat))
	csms.On("MeterValues", cast(meterValues))
	csms.On("StartTransaction", cast(startTransaction))
	csms.On("StatusNotification", cast(statusNotification))
	csms.On("StopTransaction", cast(stopTransaction))

	cfg := &config{
		MeterValueSampleIntervalS: cli.SampleIntervalS,
		ClockAlignedDataIntervalS: cli.ClockAlignedIntervalS,
		Measurands:                cli.Measurands,
		MinimumStatusDurationS:    cli.MinStatusDurationS,
	}
	csms.After("BootNotification", cfg.changeConfigration)

	csms.Start(cli.OCPPListen, "/ws/", nil)
}

func customPreUpgradeHandler(w http.ResponseWriter, r *http.Request) bool {
	u, _, ok := r.BasicAuth()
	if !ok {
		slog.Error("Error parsing basic auth")
		w.WriteHeader(401)
		return false
	}
	path := strings.Split(r.URL.Path, "/")
	id := path[len(path)-1]
	if u != id {
		slog.Error("Username provided is incorrect")
		w.WriteHeader(401)
		return false
	}
	return true
}

func bootNotification(cp *ocpp.ChargePoint, p *v16.BootNotificationReq) *v16.BootNotificationConf {
	chargerInfo.WithLabelValues(p.ChargePointVendor, p.ChargePointModel, p.ChargePointSerialNumber).Set(1)
	slog.Info("Charge point connected", "vendor", p.ChargePointVendor, "model", p.ChargePointModel, "serial", p.ChargePointSerialNumber)
	return &v16.BootNotificationConf{
		CurrentTime: time.Now().UTC().Format(time.RFC3339Nano),
		Interval:    60,
		Status:      "Accepted",
	}
}

type config struct {
	MeterValueSampleIntervalS int
	ClockAlignedDataIntervalS int
	MinimumStatusDurationS    int
	Measurands                string
}

func (c *config) changeConfigration(cp *ocpp.ChargePoint, _ ocpp.Payload) {
	type changeConfigurationReq struct {
		Key   string `json:"key" validate:"required,max=50"`
		Value string `json:"value" validate:"required,max=500"`
	}
	reqs := []changeConfigurationReq{
		{Key: "MeterValuesAlignedData", Value: c.Measurands},
		{Key: "MeterValuesSampledData", Value: c.Measurands},
		{Key: "ClockAlignedDataInterval", Value: strconv.Itoa(c.ClockAlignedDataIntervalS)},
		{Key: "MeterValueSampleInterval", Value: strconv.Itoa(c.MeterValueSampleIntervalS)},
		{Key: "MinimumStatusDuration", Value: strconv.Itoa(c.MinimumStatusDurationS)},
	}
	for _, req := range reqs {
		res, err := cp.Call("ChangeConfiguration", req)
		if err != nil {
			slog.Error("Failed to change configuration", "key", req.Key, "val", req.Value, "err", err)
			continue
		}
		if conf, ok := res.(*v16.ChangeConfigurationConf); !ok {
			slog.Error("Response of bad type", "res", res)
			continue
		} else if conf.Status != "Accepted" {
			slog.Error("Failed to change configuration", "key", req.Key, "val", req.Value, "status", conf.Status)
		} else {
			slog.Debug("Set config", "key", req.Key, "val", req.Value)
		}
	}

	res, err := cp.Call("TriggerMessage", v16.TriggerMessageReq{RequestedMessage: "MeterValues"})
	if err != nil {
		slog.Error("Failed to trigger message", "err", err)
	}
	if conf, ok := res.(*v16.TriggerMessageConf); !ok {
		slog.Error("Response of bad type", "res", res)
	} else if conf.Status != "Accepted" {
		slog.Error("Failed to trigger message", "status", conf.Status)
	} else {
		slog.Debug("Triggered meter values message")
	}
}

func authorize(cp *ocpp.ChargePoint, p *v16.AuthorizeReq) *v16.AuthorizeConf {
	return &v16.AuthorizeConf{
		IdTagInfo: v16.IdTagInfo{
			Status: "Accepted",
		},
	}
}

func dataTransfer(cp *ocpp.ChargePoint, p *v16.DataTransferReq) *v16.DataTransferConf {
	slog.Debug("DataTransfer", "p", p)
	return &v16.DataTransferConf{
		Status: "Accepted",
	}
}

func diagnosticsStatusNotification(cp *ocpp.ChargePoint, p *v16.DiagnosticsStatusNotificationReq) *v16.DiagnosticsStatusNotificationConf {
	slog.Debug("DiagnosticsStatusNotification", "p", p)
	return &v16.DiagnosticsStatusNotificationConf{}
}

func firmwareStatusNotification(cp *ocpp.ChargePoint, p *v16.FirmwareStatusNotificationReq) *v16.FirmwareStatusNotificationConf {
	slog.Debug("FirmwareStatusNotification", "p", p)
	return &v16.FirmwareStatusNotificationConf{}
}

func heartbeat(cp *ocpp.ChargePoint, p *v16.HeartbeatReq) *v16.HeartbeatConf {
	chargerLastHeartbeat.Set(float64(time.Now().UnixNano() / int64(time.Millisecond)))
	return &v16.HeartbeatConf{
		CurrentTime: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func meterValues(cp *ocpp.ChargePoint, p *v16.MeterValuesReq) *v16.MeterValuesConf {
	slog.Debug("MeterValues", "p", p)
	for _, mv := range p.MeterValue {
		for _, sv := range mv.SampledValue {
			key := strings.Join([]string{sv.Measurand, sv.Phase, sv.Unit}, "/")
			val, err := strconv.ParseFloat(sv.Value, 64)
			if err != nil {
				slog.Error("Failed to parse meter value", "key", key, "val", sv.Value, "err", err)
				continue
			}
			key = strings.ReplaceAll(key, "//", "/")
			slog.Debug("Set meter value", "key", key, "val", val)
			chargerMeterValue.WithLabelValues(key).Set(val)
		}
	}
	return &v16.MeterValuesConf{}
}

func startTransaction(cp *ocpp.ChargePoint, p *v16.StartTransactionReq) *v16.StartTransactionConf {
	slog.Info("Start transaction", "meter", ptrv(p.MeterStart))
	return &v16.StartTransactionConf{
		IdTagInfo: v16.IdTagInfo{
			Status: "Accepted",
		},
		TransactionId: int(time.Now().Unix()),
	}
}

func statusNotification(cp *ocpp.ChargePoint, p *v16.StatusNotificationReq) *v16.StatusNotificationConf {
	idx := slices.Index(chargerStates, p.Status)
	slog.Info("Status notification", "status", p.Status, "statusIdx", idx, "info", p.Info)
	chargerState.Set(float64(idx))
	return &v16.StatusNotificationConf{}
}

func stopTransaction(cp *ocpp.ChargePoint, p *v16.StopTransactionReq) *v16.StopTransactionConf {
	slog.Info("Stop transaction", "meter", ptrv(p.MeterStop), "reason", p.Reason)
	return &v16.StopTransactionConf{
		IdTagInfo: v16.IdTagInfo{
			Status: "Accepted",
		},
	}
}

func cast[R, C any](fn func(cp *ocpp.ChargePoint, p R) C) func(cp *ocpp.ChargePoint, p ocpp.Payload) ocpp.Payload {
	return func(cp *ocpp.ChargePoint, p ocpp.Payload) ocpp.Payload {
		r, ok := p.(R)
		if !ok {
			slog.Error("Failed to cast", "p", p, "r", new(R))
			return nil
		}
		return fn(cp, r)
	}
}

func ptrv(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

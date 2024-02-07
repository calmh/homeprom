package main

import (
	"flag"
	"log"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/aliml92/ocpp"
	v16 "github.com/aliml92/ocpp/v16"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var csms *ocpp.Server

var (
	chargerInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "charger_info",
	}, []string{"vendor", "model", "serial"})
	chargerState = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "charger_state",
	})
	chargerLastHeartbeat = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "charger_last_heartbeat",
	})
)

var chargerStates = []string{"", "Available", "Preparing", "Charging", "SuspendedEV", "SuspendedEVSE", "Finishing", "Reserved", "Unavailable", "Faulted"}

func main() {
	httpAddr := flag.String("http-listen", ":2118", "HTTP address to listen on")
	ocppAddr := flag.String("ocpp-listen", ":8999", "OCPP address to listen on")
	flag.Parse()

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(*httpAddr, nil))
	}()

	csms = ocpp.NewServer()

	csms.AddSubProtocol("ocpp1.6")
	csms.SetCheckOriginHandler(func(r *http.Request) bool { return true })
	csms.SetPreUpgradeHandler(customPreUpgradeHandler)
	csms.SetCallQueueSize(32)

	csms.On("BootNotification", cast(bootNotification))
	csms.On("Authorize", cast(authorize))
	csms.On("Heartbeat", cast(heartbeat))
	csms.On("StatusNotification", cast(statusNotification))
	csms.On("MeterValues", cast(meterValues))
	csms.On("DataTransfer", cast(dataTransfer))

	slog.Info("starting", "ocpp", *ocppAddr, "http", *httpAddr)
	csms.Start(*ocppAddr, "/ws/", nil)
}

func customPreUpgradeHandler(w http.ResponseWriter, r *http.Request) bool {
	u, _, ok := r.BasicAuth()
	if !ok {
		slog.Info("error parsing basic auth")
		w.WriteHeader(401)
		return false
	}
	path := strings.Split(r.URL.Path, "/")
	id := path[len(path)-1]
	if u != id {
		slog.Info("username provided is incorrect")
		w.WriteHeader(401)
		return false
	}
	return true
}

func bootNotification(cp *ocpp.ChargePoint, p *v16.BootNotificationReq) *v16.BootNotificationConf {
	chargerInfo.WithLabelValues(p.ChargePointVendor, p.ChargePointModel, p.ChargePointSerialNumber).Set(1)
	return &v16.BootNotificationConf{
		CurrentTime: time.Now().UTC().Format(time.RFC3339Nano),
		Interval:    60,
		Status:      "Accepted",
	}
}

func heartbeat(cp *ocpp.ChargePoint, p *v16.HeartbeatReq) *v16.HeartbeatConf {
	chargerLastHeartbeat.Set(float64(time.Now().UnixNano() / int64(time.Millisecond)))
	return &v16.HeartbeatConf{
		CurrentTime: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func dataTransfer(cp *ocpp.ChargePoint, p *v16.DataTransferReq) *v16.DataTransferConf {
	slog.Info("DataTransfer", "p", p)
	return &v16.DataTransferConf{
		Status: "Accepted",
	}
}

func statusNotification(cp *ocpp.ChargePoint, p *v16.StatusNotificationReq) *v16.StatusNotificationConf {
	idx := slices.Index(chargerStates, p.Status)
	slog.Info("Status", "p", p, "idx", idx)
	chargerState.Set(float64(idx))
	return &v16.StatusNotificationConf{}
}

func meterValues(cp *ocpp.ChargePoint, p *v16.MeterValuesReq) *v16.MeterValuesConf {
	slog.Info("MeterValues", "p", p)
	return &v16.MeterValuesConf{}
}

func authorize(cp *ocpp.ChargePoint, p *v16.AuthorizeReq) *v16.AuthorizeConf {
	return &v16.AuthorizeConf{
		IdTagInfo: v16.IdTagInfo{
			Status: "Accepted",
		},
	}
}

func cast[R, C any](fn func(cp *ocpp.ChargePoint, p R) C) func(cp *ocpp.ChargePoint, p ocpp.Payload) ocpp.Payload {
	return func(cp *ocpp.ChargePoint, p ocpp.Payload) ocpp.Payload {
		r, ok := p.(R)
		if !ok {
			slog.Error("failed to cast", "p", p, "r", new(R))
			return nil
		}
		return fn(cp, r)
	}
}

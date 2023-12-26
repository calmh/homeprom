package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	listenAddr := flag.String("listen", "localhost:2114", "Address to listen on")
	gpsAddr := flag.String("nmea", "localhost:4001", "Address of NMEA GPS")
	flag.Parse()

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(*listenAddr, nil))
	}()

	for {
		if err := process(*gpsAddr); err != nil {
			log.Printf("Error: %v", err)
		}
		time.Sleep(5 * time.Second)
	}
}

var (
	gpsFix = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gps_fix",
	}, []string{"system"})
	gpsUsed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gps_satellites_used",
	}, []string{"system"})
	gpsPDOP = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gps_pdop",
	}, []string{"system"})
	gpsHDOP = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gps_hdop",
	}, []string{"system"})
	gpsVDOP = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gps_vdop",
	}, []string{"system"})
	timingTIE = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "timing_tie_ns",
	})
	timingControl = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "timing_control",
	})
)

var sysIDs = map[int]string{
	1: "GPS",
	2: "GLONASS",
	3: "Galileo",
	4: "Beidou",
	5: "QZSS",
}

func process(upsdAddr string) error {
	conn, err := net.Dial("tcp", upsdAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	log.Println("Connected to", conn.RemoteAddr())

	br := bufio.NewReader(conn)

	vars := make(map[string]*prometheus.GaugeVec)
	_ = vars
	for {
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return err
		}
		line, err := br.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		fields := strings.Split(line, "*")
		fields = strings.Split(fields[0], ",")
		if len(fields[0]) != 6 {
			continue
		}
		tc := fields[0][3:6]
		switch tc {
		case "GSA":
			// Standard GSA sentence
			fix, _ := strconv.Atoi(fields[2])
			used := 0
			for i := 3; i < 15; i++ {
				if fields[i] != "" {
					used++
				}
			}
			pdop, _ := strconv.ParseFloat(fields[15], 64)
			hdop, _ := strconv.ParseFloat(fields[16], 64)
			vdop, _ := strconv.ParseFloat(fields[17], 64)
			sysID, _ := strconv.Atoi(fields[18])
			gpsFix.WithLabelValues(sysIDs[sysID]).Set(float64(fix))
			gpsUsed.WithLabelValues(sysIDs[sysID]).Set(float64(used))
			gpsPDOP.WithLabelValues(sysIDs[sysID]).Set(pdop)
			gpsHDOP.WithLabelValues(sysIDs[sysID]).Set(hdop)
			gpsVDOP.WithLabelValues(sysIDs[sysID]).Set(vdop)
		case "TXT":
			// Totally nonstandard vendor-specific sentence
			if len(fields) < 9 {
				continue
			}
			tie, _ := strconv.Atoi(fields[7])
			control, _ := strconv.Atoi(fields[8])
			timingTIE.Set(float64(tie))
			timingControl.Set(float64(control))
		}
	}
}

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	listenAddr := flag.String("listen", "localhost:2112", "Address to listen on")
	upsdAddr := flag.String("upsd", "localhost:3493", "Address of upsd")
	flag.Parse()

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(*listenAddr, nil))
	}()

	vars := make(map[string]*prometheus.GaugeVec)
	for {
		if err := process(*upsdAddr, vars); err != nil {
			log.Printf("Error: %v", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func process(upsdAddr string, vars map[string]*prometheus.GaugeVec) error {
	conn, err := net.Dial("tcp", upsdAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	log.Println("Connected to", conn.RemoteAddr())

	br := bufio.NewReader(conn)

	var ups []string
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(conn, "LIST UPS\n"); err != nil {
		return err
	}

	if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return err
	}
upsList:
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return err
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "UPS":
			ups = append(ups, fields[1])
		case "END":
			break upsList
		}
	}

	log.Println("Got list of UPSs:", ups)

	for {
	varList:
		for _, u := range ups {
			if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(conn, "LIST VAR %s\n", u); err != nil {
				return err
			}

			if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
				return err
			}
			for {
				line, err := br.ReadString('\n')
				if err != nil {
					return err
				}

				fields := strings.Fields(line)
				switch fields[0] {
				case "VAR":
					key := strings.ReplaceAll(fields[2], ".", "_")
					val := strings.Trim(fields[3], "\"")
					if fval, err := strconv.ParseFloat(val, 64); err == nil {
						if _, ok := vars[key]; !ok {
							vars[key] = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: key}, []string{"ups"})
							if err := prometheus.Register(vars[key]); err != nil {
								log.Println("Error registering", key, ":", err)
							}
						}
						vars[key].WithLabelValues(u).Set(fval)
					}
				case "END":
					break varList
				}
			}
		}
		time.Sleep(15 * time.Second)
	}
}

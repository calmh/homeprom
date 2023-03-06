package main

import (
	"flag"
	"io"
	"log"
	"net"

	"go.bug.st/serial"
)

func main() {
	port := flag.String("serial", "/dev/ttyUSB0", "Serial port")
	listen := flag.String("listen", "0.0.0.0:2113", "Listen address")
	flag.Parse()

	fd, err := serial.Open(*port, &serial.Mode{
		BaudRate: 115200,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		log.Fatalf("open %s: %v", *port, err)
	}

	lines := NewFanout[string]()
	go readLines(fd, lines)

	list, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := list.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn, lines)
	}
}

func readLines(r io.Reader, lines *fanout[string]) {
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		lines.Publish(string(buf[:n]))
	}
}

func handleConn(conn net.Conn, lines *fanout[string]) {
	sub := lines.Listen()
	defer sub.Close()
	defer conn.Close()
	for line := range sub.Channel() {
		_, err := conn.Write([]byte(line))
		if err != nil {
			log.Println(err)
			return
		}
	}
}

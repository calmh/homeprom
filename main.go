package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func main() {
	pg := flag.String("pushgateway", "https://nmea.calmh.dev", "Pushgateway URL")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Printf("Usage: %s <serial port>\n", os.Args[0])
		os.Exit(2)
	}

	fd, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	framer := NewFramer(fd)
	buf := new(bytes.Buffer)
	for {
		frame, err := framer.Read()
		if err != nil {
			log.Fatal(err)
		}

		instance := sanitizeString(frame.FlagID + "_" + frame.Ident)
		for _, d := range frame.Data {
			val, err := Parse(d)
			if err != nil {
				log.Fatal(err)
			}
			name := counterName(val)
			switch val.Ident.Cumulative {
			case 7:
				fmt.Fprintf(buf, "# TYPE %s gauge\n", name)
			case 8:
				fmt.Fprintf(buf, "# TYPE %s counter\n", name)
			}
			fmt.Fprintf(buf, "%s %f\n", name, val.Value)
		}
		resp, err := http.Post(*pg+"/metrics/job/hanprom/instance/"+instance, "text/plain", buf)
		if err != nil {
			log.Println("Push:", err)
		} else if resp.StatusCode != http.StatusOK {
			fmt.Println("Push:", resp.Status)
			io.Copy(os.Stdout, resp.Body)
		}
		resp.Body.Close()
		buf.Reset()
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

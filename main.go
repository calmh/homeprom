package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func main() {
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
	for {
		frame, err := framer.Read()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s %s %s\n", frame.FlagID, frame.BaudRate, frame.Ident)
		for _, d := range frame.Data {
			val, err := Parse(d)
			if err != nil {
				log.Fatal(err)
			}
			if val.Ident == DateTimeIdent {
				fmt.Println(counterName(val), time.Unix(int64(val.Value), 0))
				continue
			}
			fmt.Println(counterName(val), val.Value)
		}
	}
}

func counterName(v *Value) string {
	name := sanitizeString(IdentDescr[v.Ident])
	if v.Unit != "" {
		name += "_" + v.Unit
	}
	return name
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

package main

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

type Framer struct {
	br *bufio.Reader
}

func NewFramer(r io.Reader) *Framer {
	return &Framer{br: bufio.NewReader(r)}
}

type Frame struct {
	FlagID   string
	BaudRate string
	Ident    string
	Data     []string
	Checksum uint16
}

func (f *Framer) Read() (*Frame, error) {
	frame := &Frame{}
	state := 0
loop:
	for {
		line, err := f.br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		switch state {
		case 0:
			if line[0] == '/' {
				frame.FlagID = line[1:4]
				frame.BaudRate = line[4:5]
				frame.Ident = line[5:]
				state = 1
			}
		case 1:
			if line[0] == '!' {
				checksum, _ := strconv.ParseInt(line[1:], 16, 16)
				frame.Checksum = uint16(checksum)
				state = 2
				break loop
			} else {
				frame.Data = append(frame.Data, line)
			}
		}
	}
	if state != 2 {
		return nil, errors.New("invalid frame")
	}
	return frame, nil
}

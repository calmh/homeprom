package main

import (
	"bufio"
	"errors"
	"io"
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
		switch state {
		case 0:
			line, err := f.br.ReadString('\n')
			if err != nil {
				return nil, err
			}
			line = strings.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			if line[0] == '/' {
				frame.FlagID = line[1:4]
				frame.BaudRate = line[4:5]
				frame.Ident = line[5:]
				state = 1
			}
		case 1:
			peek, err := f.br.Peek(1)
			if err != nil {
				return nil, err
			}
			if peek[0] == '!' {
				_, err := f.br.Discard(1)
				if err != nil {
					return nil, err
				}
				high, err := f.br.ReadByte()
				if err != nil {
					return nil, err
				}
				low, err := f.br.ReadByte()
				if err != nil {
					return nil, err
				}
				frame.Checksum = uint16(high)<<8 | uint16(low)
				state = 2
				break loop
			} else {
				line, err := f.br.ReadString('\n')
				if err != nil {
					return nil, err
				}
				line = strings.TrimSpace(line)
				if len(line) == 0 {
					continue
				}
				frame.Data = append(frame.Data, line)
			}
		}
	}
	if state != 2 {
		return nil, errors.New("invalid frame")
	}
	return frame, nil
}

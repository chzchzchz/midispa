package main

import (
	"encoding"
	"errors"
	"os"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/sysex"
)

var errNoHandshake = errors.New("no handshake")
var errBadPacket = errors.New("bad packet number")
var errCancel = errors.New("canceled")

func fileDump(seq *alsa.Seq, sa alsa.SeqAddr, f *os.File) error {
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	send := func(m encoding.BinaryMarshaler) error {
		v, err := m.MarshalBinary()
		if err != nil {
			return err
		}
		return seq.Write(alsa.SeqEvent{sa, v})
	}
	fsz := int(fi.Size())
	hdr := sysex.FileDumpHeader{
		FileDumpRequest: sysex.FileDumpRequest{
			DeviceId: 1,
			SourceId: 1,
			Type:     "file",
			Name:     fi.Name(),
		},
		Length: fsz,
	}
	if err := send(&hdr); err != nil {
		return err
	}
	pkt := 0
	for i := 0; i < fsz; i += sysex.FileDumpMaxChunk {
		toWrite := sysex.FileDumpMaxChunk
		if i+toWrite > fsz {
			toWrite = fsz - i
		}
		buf := make([]byte, toWrite)
		if _, err := f.Read(buf); err != nil {
			return err
		}
		// Send data packet and loop until ack
		// TODO: timeouts
		data := &sysex.FileDumpDataPacket{1, pkt & 0x7f, buf}
		for {
			if err := send(data); err != nil {
				return err
			}
			ev, err := seq.ReadSysEx()
			if err != nil {
				return err
			}
			msg := sysex.Decode(ev.Data)
			if msg == nil {
				return errNoHandshake
			}
			if hs, ok := msg.(*sysex.Handshake); ok {
				if hs.Packet != data.Packet {
					return errBadPacket
				} else if hs.SubId == sysex.SubIdNAK {
					continue
				} else if hs.SubId == sysex.SubIdACK {
					break
				} else if hs.SubId == sysex.SubIdCancel {
					return errCancel
				}
			}
			return errNoHandshake
		}
		pkt++
	}
	eof := &sysex.Handshake{DeviceId: 1, SubId: sysex.SubIdEOF, Packet: pkt & 0x7f}
	return send(eof)
}

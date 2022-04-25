package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/util"
)

type Connection struct {
	Source      string
	Sink        string
	Description string
	sa          *alsa.SeqAddr
}

type DeviceTracker struct {
	disconnected map[string]struct{}
	connected    map[string]alsa.SeqAddr
	aseq         *alsa.Seq
	donec        chan struct{}
	cancelc      chan struct{}
	updatec      chan struct{}
	mu           sync.Mutex
	sub          func(s string)
}

func NewDeviceTracker(aseq *alsa.Seq, subf func(string)) *DeviceTracker {
	dt := &DeviceTracker{
		disconnected: make(map[string]struct{}),
		connected:    make(map[string]alsa.SeqAddr),
		aseq:         aseq,
		donec:        make(chan struct{}),
		cancelc:      make(chan struct{}),
		updatec:      make(chan struct{}, 1),
		sub:          subf,
	}
	go dt.run()
	return dt
}

func (dt *DeviceTracker) run() {
	timer := time.NewTimer(time.Second)
	tickc := timer.C
	defer func() {
		if !timer.Stop() {
			<-timer.C
		}
		close(dt.donec)
	}()
	for {
		select {
		case <-dt.cancelc:
			return
		case <-dt.updatec:
			// Don't immediately try to reconnect.
			log.Println("sleeping 5s waiting for device bounce")
			time.Sleep(5 * time.Second)
		case <-tickc:
			tickc = nil
		}
		dt.mu.Lock()
		for s := range dt.disconnected {
			if sa, err := dt.aseq.PortAddress(s); err == nil {
				dt.connected[s] = sa
				dt.sub(s)
				delete(dt.disconnected, s)
			}
		}
		dlen := len(dt.disconnected)
		dt.mu.Unlock()
		if dlen == 0 {
			tickc = nil
		} else if dlen != 0 && tickc == nil {
			timer.Stop()
			select {
			case <-timer.C:
			default:
			}
			timer.Reset(time.Second)
			tickc = timer.C
		}
	}
}

func (dt *DeviceTracker) Close() {
	select {
	case dt.cancelc <- struct{}{}:
		<-dt.donec
	case <-dt.donec:
	}
}

func (dt *DeviceTracker) Add(s string) {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	if _, ok := dt.connected[s]; ok {
		return
	}
	if _, ok := dt.disconnected[s]; ok {
		return
	}
	sa, err := dt.aseq.PortAddress(s)
	if err != nil {
		log.Printf("waiting on device %q", s)
		dt.disconnected[s] = struct{}{}
		return
	}
	log.Printf("tracking device %q on %s", s, sa.String())
	if err := dt.aseq.OpenPortWrite(sa); err != nil {
		panic(err)
	}
	dt.connected[s] = sa
	dt.sub(s)
}

func (dt *DeviceTracker) Unsubscribe(sa alsa.SeqAddr) {
	dt.mu.Lock()
	name := ""
	for c, csa := range dt.connected {
		if sa == csa {
			name = c
			delete(dt.connected, name)
			break
		}
	}
	if name != "" {
		dt.disconnected[name] = struct{}{}
	}
	dt.mu.Unlock()
	log.Printf("lost connection to %q", name)
	select {
	case dt.updatec <- struct{}{}:
	case <-dt.donec:
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: midiconn config.json")
		return
	}
	aseq, err := alsa.OpenSeq("midiconn")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()

	conns := util.MustLoadJSONFile[Connection](os.Args[1])
	subf := func(s string) {
		sa, err := aseq.PortAddress(s)
		if err != nil {
			panic(err)
		}
		log.Printf("activating device %q", s)
		for _, c := range conns {
			var dst, src alsa.SeqAddr
			if c.Source == s {
				src = sa
				if dst, err = aseq.PortAddress(c.Sink); err != nil {
					continue
				}
			} else if c.Sink == s {
				dst = sa
				if src, err = aseq.PortAddress(c.Source); err != nil {
					continue
				}
			} else {
				continue
			}
			log.Printf("connect %q -> %q", c.Source, c.Sink)
			args := []string{src.String(), dst.String()}
			cmd := exec.Command("aconnect", args...)
			cmd.Stderr, cmd.Stdout = os.Stderr, os.Stdout
			if err := cmd.Run(); err != nil {
				log.Printf("failed to connect %s", err)
			}
		}
		log.Printf("activated device %q", s)
	}
	dt := NewDeviceTracker(aseq, subf)
	for _, c := range conns {
		dt.Add(c.Source)
		dt.Add(c.Sink)
	}
	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		dat := ev.Data
		if dat[0] == 0 {
			dt.Unsubscribe(alsa.SeqAddr{int(dat[3]), int(dat[4])})
		}
	}
}

package http

import (
	"context"
	"encoding/json"
	//"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	amidi "github.com/scgolang/midi"

	"github.com/chzchzchz/midispa/sequencer"
	"github.com/chzchzchz/midispa/track"
)

type Config struct {
	ListenServ string
	MidiPath   string
	WebPath    string
}

type midiHandler struct {
	Config
	tracks map[string]*trackDev

	wg sync.WaitGroup
	mu sync.RWMutex
}

type trackDev struct {
	d *amidi.Device

	Requests map[time.Time]*trackReq
	ctx      context.Context
	cancel   context.CancelFunc

	pattern *track.Pattern
	seq     *sequencer.Sequencer
}

type trackReq struct {
	ctx    context.Context
	cancel context.CancelFunc
	t      *track.Track

	Name    string
	Issued  time.Time
	Started time.Time
}

func newTrackDev(d *amidi.Device) *trackDev {
	ctx, cancel := context.WithCancel(context.TODO())
	pat := track.NewPattern(ctx)
	outc := make(chan track.TickMessage, 10)
	seq := sequencer.NewDevice(outc, d)
	c := sequencer.NewClocker(pat.Chan(), outc, &seq.Sequencer)
	return &trackDev{
		d:        d,
		pattern:  pat,
		seq:      &c.Sequencer,
		ctx:      ctx,
		cancel:   cancel,
		Requests: make(map[time.Time]*trackReq),
	}
}

func (td *trackDev) Close() {
	td.cancel()
	td.seq.Close()
	td.d.Close()
}

func Serve(cfg Config) error {
	mh := &midiHandler{
		Config: cfg,
		tracks: make(map[string]*trackDev),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/device/", mh.handleDevice)
	mux.HandleFunc("/pattern/", mh.handlePattern)
	mux.HandleFunc("/track/", mh.handleTrack)
	mux.HandleFunc("/", mh.handleIndex)

	return http.ListenAndServe(cfg.ListenServ, mux)
}

func (mh *midiHandler) handleIndex(w http.ResponseWriter, r *http.Request) {
}

func (mh *midiHandler) handleDevice(w http.ResponseWriter, r *http.Request) {
	devs, err := amidi.Devices()
	if err != nil {
		panic(err)
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(devs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (mh *midiHandler) handleTrack(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mh.mu.RLock()
		js, err := json.Marshal(mh.tracks)
		mh.mu.RUnlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(js)
		}
	case http.MethodPost:
		if err := mh.postTrack(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

type TrackPostRequest struct {
	Pattern string
	Loops   int
}

func (mh *midiHandler) postTrack(r *http.Request) error {
	ctx, cancel := context.WithCancel(r.Context())

	devId := path.Base(r.URL.Path)

	var tpr TrackPostRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&tpr); err != nil {
		return err
	}
	patPath := filepath.Clean(tpr.Pattern)
	t, err := track.NewSMF(ctx, filepath.Join(mh.MidiPath, patPath))
	if err != nil {
		return err
	}
	pat := track.NewLoop(ctx, t)
	tr := trackReq{ctx: ctx, cancel: cancel, t: pat, Name: patPath, Issued: time.Now()}

	mh.mu.Lock()
	td, err := mh.getTrackDev(devId)
	if err != nil {
		mh.mu.Unlock()
		return err
	}
	for {
		if _, ok := td.Requests[tr.Issued]; !ok {
			break
		}
		tr.Issued = time.Now()
	}
	mh.mu.Unlock()

	defer func() {
		mh.mu.Lock()
		delete(td.Requests, tr.Issued)
		mh.mu.Unlock()
	}()

	select {
	case td.pattern.TrackChan() <- t:
		tr.Started = time.Now()
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case td.seq.Tempoc <- sequencer.Tempo{t.MidiTimeSig, time.Now(), 0}:
	case <-ctx.Done():
		return ctx.Err()
	}
	<-pat.Done()

	return nil
}

func (mh *midiHandler) getTrackDev(devId string) (*trackDev, error) {
	// mh.mu must be locked
	if td, ok := mh.tracks[devId]; ok {
		return td, nil
	}
	panic("STUB")
	/*
	d, err := sequencer.OpenDeviceById(devId)
	if err != nil {
		return nil, err
	} else if d != nil {
		return nil, fmt.Errorf("could not find device id %s", devId)
	}
	td := newTrackDev(d)
	mh.tracks[devId] = td
	return td, nil
	*/
}

func (mh *midiHandler) handlePattern(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	paths := walk(mh.MidiPath)
	for i := range paths {
		paths[i] = strings.TrimPrefix(paths[i], mh.MidiPath)
	}
	if err := enc.Encode(paths); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func walk(dir string) (ret []string) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, de := range des {
		if de.IsDir() {
			ret = append(ret, walk(filepath.Join(dir, de.Name()))...)
		} else {
			ret = append(ret, filepath.Join(dir, de.Name()))
		}
	}
	return ret
}

package main

import (
	"encoding"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/sysex"
)

type httpHandler struct {
	aseq *alsa.Seq
	// TODO: per-device locks
	mu sync.RWMutex
}

func replyJSON(resp http.ResponseWriter, iface interface{}) error {
	resp.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(resp).Encode(iface)
}

func (h *httpHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	errReply := func(err error) {
		if err == nil {
			return
		}
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write([]byte(err.Error()))
		panic(err)
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("error recovered %q", r)
		}
	}()

	accept := req.Header.Get("Accept")
	mediatype, isRead := "", false
	var sysexIface interface{}
	if accept != "" {
		mt, params, err := mime.ParseMediaType(accept)
		errReply(err)
		isRead = mt == "application/json" || mt == "application/octet-stream"
		if isRead {
			content, ok := params["content"]
			if !ok {
				errReply(fmt.Errorf("no decode type given"))
			}
			sysexIface, err = copyTypeInterface(content)
			errReply(err)
		}
		mediatype = mt
	}
	switch req.Method {
	case http.MethodPost:
		pathParts := strings.Split(req.URL.Path, "/")
		devName, ty := pathParts[1], strings.Join(pathParts[2:], "/")
		defer req.Body.Close()
		rty, err := readReflectedJson(ty, req.Body)
		errReply(err)
		bm, ok := rty.(encoding.BinaryMarshaler)
		if !ok {
			errReply(fmt.Errorf("couldn't binary marshal %s", req.URL.Path))
		}
		msg, err := bm.MarshalBinary()
		errReply(err)

		h.mu.Lock()
		defer h.mu.Unlock()

		log.Printf("opening %q", devName)
		sa, err := h.aseq.PortAddress(devName)
		errReply(err)
		errReply(h.aseq.OpenPortWrite(sa))
		defer h.aseq.ClosePortWrite(sa)
		if isRead {
			errReply(h.aseq.OpenPortRead(sa))
			defer h.aseq.ClosePortRead(sa)
		}
		rws := rwSysEx{h.aseq, sa, msg, sysexIface, req.URL.Path}
		inSysEx, err := rws.doAllSysEx()
		errReply(err)
		if isRead {
			replyJSON(resp, inSysEx)
		} else {
			resp.WriteHeader(http.StatusOK)
		}
	case http.MethodGet:
		pathParts := strings.Split(req.URL.Path, "/")
		if len(pathParts) < 2 {
			d, err := alsa.Devices()
			errReply(err)
			errReply(replyJSON(resp, d))
			return
		}
		devName := pathParts[1]

		h.mu.Lock()
		defer h.mu.Unlock()
		log.Printf("opening %q", devName)
		sa, err := h.aseq.PortAddress(devName)
		errReply(err)
		errReply(h.aseq.OpenPortRead(sa))
		defer h.aseq.ClosePortRead(sa)
		rws := rwSysEx{h.aseq, sa, nil, sysexIface, req.URL.Path}
		inSysEx, err := rws.read()
		errReply(err)
		if mediatype == "application/json" {
			replyJSON(resp, inSysEx)
		} else {
			sysex := (inSysEx[0]).(*sysex.SysEx)
			resp.Write(sysex.Data)
		}
	default:
		resp.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func main() {
	listenFlag := flag.String("l", "localhost:4567", "interface and port to listen")
	flag.Parse()

	aseq, err := alsa.OpenSeq("midijson")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()

	mh := &httpHandler{aseq: aseq}
	log.Println("listening on", *listenFlag)
	if err := http.ListenAndServe(*listenFlag, mh); err != nil {
		panic(err)
	}
}

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
)

type httpHandler struct {
	aseq *alsa.Seq
	// TODO: per-device locks
	mu sync.RWMutex
}

func (h *httpHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	errReply := func(err error) {
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write([]byte(err.Error()))
		panic(err)
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("error recovered %q", r)
		}
	}()

	replyJSON := func(iface interface{}) {
		resp.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(resp)
		if err := enc.Encode(iface); err != nil {
			errReply(err)
		}
	}

	switch req.Method {
	case http.MethodPost:
		pathParts := strings.Split(req.URL.Path, "/")
		devName, ty := pathParts[1], strings.Join(pathParts[2:], "/")
		defer req.Body.Close()
		rty, err := readReflectedJson(ty, req.Body)
		if err != nil {
			errReply(err)
		}
		bm, ok := rty.(encoding.BinaryMarshaler)
		if !ok {
			errReply(fmt.Errorf("couldn't binary marshal %s", req.URL.Path))
		}
		msg, err := bm.MarshalBinary()
		if err != nil {
			errReply(err)
		}

		accept := req.Header.Get("Accept")
		isRead := false
		var sysexIface interface{}
		if accept != "" {
			mt, params, err := mime.ParseMediaType(accept)
			if err != nil {
				errReply(err)
			}
			if isRead = mt == "application/json"; isRead {
				content, ok := params["content"]
				if !ok {
					errReply(fmt.Errorf("no decode type given"))
				}
				sysexIface, err = copyTypeInterface(content)
				if err != nil {
					errReply(err)
				}
			}
		}

		h.mu.Lock()
		defer h.mu.Unlock()
		// get device, apply sysex
		sa, err := h.aseq.PortAddress(devName)
		if err != nil {
			errReply(err)
		}
		if err := h.aseq.OpenPortWrite(sa); err != nil {
			errReply(err)
		}
		defer h.aseq.ClosePortWrite(sa)
		if isRead {
			if err := h.aseq.OpenPortRead(sa); err != nil {
				errReply(err)
			}
			defer h.aseq.ClosePortRead(sa)
		}
		if err := h.aseq.Write(alsa.SeqEvent{sa, msg}); err != nil {
			errReply(err)
		}
		log.Printf("wrote sysex for %q", req.URL.Path)

		if isRead {
			var msg []byte
			// TODO: timeout if read takes too long
			for {
				ev, err := h.aseq.Read()
				if err != nil {
					errReply(err)
				}
				log.Printf("%+v", ev)
				msg = append(msg, ev.Data...)
				if msg[len(msg)-1] == 0xf7 {
					break
				}
			}
			bu, ok := sysexIface.(encoding.BinaryUnmarshaler)
			if !ok {
				errReply(fmt.Errorf("content %q not binary unmarshaller", accept))
			}
			if err := bu.UnmarshalBinary(msg); err != nil {
				errReply(err)
			}
			replyJSON(sysexIface)
		} else {
			resp.WriteHeader(http.StatusOK)
		}
	case http.MethodGet:
		d, err := alsa.Devices()
		if err != nil {
			errReply(err)
		}
		replyJSON(d)
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

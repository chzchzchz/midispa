//go:build js && wasm && !windows && !linux && !darwin
// +build js,wasm,!windows,!linux,!darwin

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"syscall/js"
	"time"

	"github.com/chzchzchz/midispa/midi"
	"github.com/coder/websocket"
)

const inputBufferSize = 16

func log(msg string) {
	document := js.Global().Get("document")
	p := document.Call("createElement", "p")
	p.Set("innerHTML", msg)
	document.Get("body").Call("appendChild", p)
}

func logElement(id, msg string) {
	document := js.Global().Get("document")
	p := document.Call("createElement", "p")
	p.Set("innerHTML", msg)
	e := document.Call("getElementById", id)
	e.Call("appendChild", p)
}

func writeElement(id, msg string) {
	document := js.Global().Get("document")
	e := document.Call("getElementById", id)
	e.Set("innerHTML", msg)
}

func e(err error) {
	if err == nil {
		return
	}
	log(fmt.Sprintf("<span style=\"color: red\">🛑 ERROR:</span> %s", err.Error()))
	log("Refresh and try again")
	//panic(err)
	os.Exit(1)
}

func getString(id string) string {
	document := js.Global().Get("document")
	e := document.Call("getElementById", id)
	return e.Get("innerText").String()
}

func handleChoose() <-chan int {
	ch := make(chan int)
	document := js.Global().Get("document")
	inputHandler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		in := document.Call("getElementById", "in_port")
		text := in.Get("value").String()
		var v int
		fmt.Sscanf(text, "%d", &v)
		ch <- v
		close(ch)
		return nil
	})
	inputElement := document.Call("getElementById", "choose_button")
	inputElement.Call("addEventListener", "click", inputHandler)
	return ch
}

func writeChooser(ins []*webmidiPort) {
	var bf bytes.Buffer
	fmt.Fprintf(&bf, "<select id=\"in_port\">\n")
	for i, in := range ins {
		fmt.Fprintf(&bf, "<option value=\"%d\">%v<br/>\n", i, in)
	}
	fmt.Fprintf(&bf, "</select>\n")
	fmt.Fprintf(&bf, "<button type=\"button\" id=\"choose_button\">Select</button>")
	writeElement("chooser", bf.String())
}

var keyMap = map[string]byte{
	"a": 56,
	"z": 57,
	"s": 58,
	"x": 59,
	"c": 60,
	"f": 61,
	"v": 62,
	"g": 63,
	"b": 64,
	"n": 65,
	"j": 66,
	"m": 67,
	"k": 68,
	",": 69,
	"l": 70,
	".": 71,
	// "/" : 72,
	// "'" : 73,
}
var kbdChannel = int(2)
var kbdDownMap = make(map[string]byte)
var kbdOctave = 0

func setupKeyboard(outc chan<- []byte) {
	document := js.Global().Get("document")
	down := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		k := args[0].Get("key").String()
		if _, ok := kbdDownMap[k]; ok {
			return nil
		} else if len(k) != 1 {
			return nil
		} else if n, ok := keyMap[k]; ok {
			n += byte(12 * kbdOctave)
			kbdDownMap[k] = n
			outc <- []byte{midi.MakeNoteOn(kbdChannel), n, 100}
		} else if k[0] >= '0' && k[0] <= '9' {
			kbdChannel = int(k[0] - '0')
		} else if k[0] == '[' && kbdOctave > -4 {
			kbdDownMap[k] = 0
			kbdOctave--
		} else if k[0] == ']' && kbdOctave < 4 {
			kbdDownMap[k] = 0
			kbdOctave++
		}
		return nil
	})
	up := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		k := args[0].Get("key").String()
		if n, ok := kbdDownMap[k]; ok {
			delete(kbdDownMap, k)
			if n > 0 {
				outc <- []byte{midi.MakeNoteOff(kbdChannel), n, 0x7f}
			}
		}
		return nil
	})
	document.Call("addEventListener", "keyup", up)
	document.Call("addEventListener", "keydown", down)
}

var PPQN = 24.0
var BPM = 120.0

func main() {
	wm, err := newWebMidi()
	e(err)
	ins, err := wm.Ins()
	e(err)
	writeChooser(ins)
	ch := handleChoose()
	idx := <-ch
	logElement("status", "🫴🏿Selected midi port <b>"+fmt.Sprintf("%s (%d)", ins[idx], idx)+"</b>")

	// Send midi messages over msgc channel.
	msgc := make(chan []byte, inputBufferSize)
	cb := func(msg []byte, ms int32) { msgc <- msg }
	err = ins[idx].Listen(cb)
	e(err)

	// Send virtual midi keyboard events over msgc.
	setupKeyboard(msgc)

	// Periodically send clocks.
	go func() {
		t := time.Now()
		clockMsg := []byte{midi.Clock}
		clocksPerSecond := float64((BPM / 60.0) * PPQN)
		dur := time.Duration(float64(time.Second) / clocksPerSecond)
		for {
			t = t.Add(dur)
			msgc <- clockMsg
			time.Sleep(time.Until(t))
		}
	}()

	// Send midi to server over websocket.
	ctx := context.Background()
	wsURL := getString("wsurl")
	wsc, _, err := websocket.Dial(ctx, wsURL, nil)
	logElement("status", fmt.Sprintf("👋🏿 Connecting to <b>%q</b>", wsURL))
	e(err)
	logElement("status", fmt.Sprintf("👍🏿 Connected to <b>%q</b>", wsURL))
	defer wsc.CloseNow()
	msgs := 0
	for msg := range msgc {
		//log(fmt.Sprintf("got: %+v<br />", msg))
		err = wsc.Write(ctx, websocket.MessageBinary, msg)
		e(err)
		if msg[0] != midi.Clock {
			msgs++
			writeElement("counter", strconv.Itoa(msgs))
		}
	}
	wsc.Close(websocket.StatusNormalClosure, "")
}

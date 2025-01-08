//go:build js && wasm && !windows && !linux && !darwin
// +build js,wasm,!windows,!linux,!darwin

package main

import (
	"fmt"
	"math"
	"syscall/js"
)

type WebMidi struct {
	inputsJS  js.Value
	outputsJS js.Value
}

func newWebMidi() (*WebMidi, error) {
	jsDoc := js.Global().Get("navigator")
	if !jsDoc.Truthy() {
		return nil, fmt.Errorf("unable to get navigator object")
	}

	opts := map[string]interface{}{"sysex": false}
	jsOpts := js.ValueOf(opts)

	midiaccess := jsDoc.Call("requestMIDIAccess", jsOpts)
	if !midiaccess.Truthy() {
		return nil, fmt.Errorf("unable to get requestMIDIAccess")
	}

	d := &WebMidi{}
	errc := make(chan error, 1)
	success := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) != 1 {
			return "wrong number of arguments"
		}
		d.inputsJS = args[0].Get("inputs")
		d.outputsJS = args[0].Get("outputs")
		errc <- nil
		return nil
	})
	failed := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		errc <- fmt.Errorf("failed accessing MIDI devices")
		return nil
	})
	midiaccess.Call("then", success, failed)
	return d, <-errc
}

func (d *WebMidi) ports(v js.Value) (p []*webmidiPort, err error) {
	if !v.Truthy() {
		return nil, fmt.Errorf("no ports")
	}
	eachIn := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		jsport := args[0]
		name := jsport.Get("name").String()
		p = append(p, &webmidiPort{name, jsport})
		return nil
	})
	v.Call("forEach", eachIn)
	return p, nil

}
func (d *WebMidi) Ins() ([]*webmidiPort, error)  { return d.ports(d.inputsJS) }
func (d *WebMidi) Outs() ([]*webmidiPort, error) { return d.ports(d.outputsJS) }

type webmidiPort struct {
	name   string
	jsport js.Value
}

func (i *webmidiPort) String() string { return i.name }
func (i *webmidiPort) Close()         { i.jsport.Call("close") }
func (i *webmidiPort) Open()          { i.jsport.Call("open") }

func (i *webmidiPort) Listen(cb func(msg []byte, milliseconds int32)) error {
	jsCallback := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		jsdata := args[0].Get("data")
		jstime := args[0].Get("receivedTime")
		data := make([]byte, 0, 16)
		for i := 0; true; i++ {
			ji := jsdata.Index(i)
			if ji.IsUndefined() {
				break
			}
			data = append(data, byte(ji.Int()))
		}
		t := int32(-1)
		if jstime.Truthy() {
			// round to milliseconds
			t = int32(math.Round(jstime.Float()))
		}
		cb(data, t)
		return nil
	})
	go i.jsport.Call("addEventListener", "midimessage", jsCallback)
	return nil
}

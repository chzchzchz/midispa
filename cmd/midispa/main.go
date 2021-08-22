package main

import (
	"github.com/chzchzchz/midispa/http"
)

func main() {
	err := http.Serve(
		http.Config{ListenServ: "localhost:12999",
		WebPath: "dat/web/",
		MidiPath: "dat/midi/"},
	)
	if err != nil {
		panic(err)
	}
}

package main

import (
	"fmt"
	//"github.com/chzchzchz/midispa/jack"
)

func main() {
	//fmt.Println(jack.Ports())
	m := Start()
	fmt.Print("AAAAAAAA")
	fmt.Println(m.RecordPort)
	fmt.Println(m.PlaybackPorts)
}

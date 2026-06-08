package main

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/chzchzchz/midispa/jack"
)

func getBaseDir() string {
	usr, err := user.Current()
	if err != nil {
		panic("error getting current user: " + err.Error())
	}
	wavtrackDir := filepath.Join(usr.HomeDir, ".wavtrack")
	if err := os.MkdirAll(wavtrackDir, 0755); err != nil {
		panic("error creating wavtrack dir: " + err.Error())
	}
	return wavtrackDir
}

func main() {
	ports, err := jack.Ports()
	if err != nil {
		panic("error getting jack ports: " + err.Error())
	}
	wavtrackDir := getBaseDir()

	m := Start(ports)

	if err := m.ProjectConfig.Save(wavtrackDir); err != nil {
		panic("error saving project: " + err.Error())
	}
}

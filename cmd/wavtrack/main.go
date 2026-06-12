package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	//	"time"

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
	var project string
	flag.StringVar(&project, "project", "", "Load project from .wavtrack directory and skip start UI")
	flag.Parse()

	ports, err := jack.Ports()
	if err != nil {
		panic("error getting jack ports: " + err.Error())
	}
	wavtrackDir := getBaseDir()

	var cfg *ProjectConfig
	if project != "" {
		fmt.Printf("Loading project: %s\n", project)
		if cfg, err = LoadProject(wavtrackDir, project); err != nil {
			panic("error loading project: " + err.Error())
		}
	} else {
		m := Start(ports)
		m.ProjectConfig.BaseDir = wavtrackDir

		rec, err := NewRecord(m.ProjectConfig.RecordPort)
		if err != nil {
			panic("couldn't record:" + err.Error())
		}
		m.ProjectConfig.SampleRate = int(rec.Port.Client.GetSampleRate())
		rec.Close()

		if err := m.ProjectConfig.Save(); err != nil {
			panic("error saving project: " + err.Error())
		}
		cfg = &m.ProjectConfig
	}

	s := &State{cfg: *cfg}
	TrackUI(s)
	/*
	   rec, err := NewRecord(cfg.RecordPort)

	   	if err != nil {
	   		panic("couldn't record:" + err.Error())
	   	}

	   defer rec.Close()
	   rec.Start("abc.wav")
	   time.Sleep(10 * time.Second)
	*/
}

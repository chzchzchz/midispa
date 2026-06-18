package main

import (
	"flag"
	"fmt"
	"log"
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

func setupLog(path string) *os.File {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
	return f
}

func main() {
	var project string
	var projectDir string
	flag.StringVar(&project, "project", "", "Load project from .wavtrack directory and skip start UI")
	flag.StringVar(&projectDir, "project-dir", "", "Base project path instead of ~/.wavtrack")
	flag.Parse()

	ports, err := jack.Ports()
	if err != nil {
		panic("error getting jack ports: " + err.Error())
	}

	wavtrackDir := projectDir
	if wavtrackDir == "" {
		wavtrackDir = getBaseDir()
	}

	logFile := setupLog(filepath.Join(wavtrackDir, "debug.log"))
	defer logFile.Close()
	log.SetPrefix("[init] ")

	var s *State
	if project != "" {
		log.Printf("Loading project: %s\n", project)
		cfg, err := LoadConfig(wavtrackDir, project)
		if err != nil {
			panic("error loading project: " + err.Error())
		}
		s = NewState(cfg)
		s.Load()
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
		s = NewState(&m.ProjectConfig)
	}

	log.SetPrefix(fmt.Sprintf("[%s] ", s.cfg.ProjectName))

	if s.Play, err = NewPlay([]string{s.cfg.LeftPlayback, s.cfg.RightPlayback}); err != nil {
		panic("couldn't create play ports:" + err.Error())
	}
	defer s.Play.Close()

	if s.rec, err = NewRecord(s.cfg.RecordPort); err != nil {
		panic("couldn't record:" + err.Error())
	}
	defer s.rec.Close()

	TrackUI(s)
}

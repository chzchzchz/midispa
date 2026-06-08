package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ProjectConfig struct {
	ProjectName   string `json:"project_name"`
	LeftPlayback  string `json:"left_playback_port"`
	RightPlayback string `json:"right_playback_port"`
	RecordPort    string `json:"record_port"`
}

func (p *ProjectConfig) Save(wavtrackDir string) error {
	dir := filepath.Join(wavtrackDir, p.ProjectName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	cfgBytes, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "project.json"), cfgBytes, 0644)
}

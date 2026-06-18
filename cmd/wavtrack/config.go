package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ProjectConfig struct {
	BaseDir       string
	ProjectName   string `json:"project_name"`
	LeftPlayback  string `json:"left_playback_port"`
	RightPlayback string `json:"right_playback_port"`
	RecordPort    string `json:"record_port"`
	SampleRate    int    `json:"sample_rate"`
}

func (p *ProjectConfig) Dir() string {
	return filepath.Join(p.BaseDir, p.ProjectName)
}

func (p *ProjectConfig) Save() error {
	if err := os.MkdirAll(p.Dir(), 0755); err != nil {
		return err
	}
	cfgBytes, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(p.Dir(), "project.json"), cfgBytes, 0644)
}

func LoadConfig(baseDir, projectName string) (*ProjectConfig, error) {
	p := &ProjectConfig{
		BaseDir:     baseDir,
		ProjectName: projectName,
	}
	cfgBytes, err := os.ReadFile(filepath.Join(p.Dir(), "project.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(cfgBytes, p); err != nil {
		return nil, err
	}
	return p, nil
}

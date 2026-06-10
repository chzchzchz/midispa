package main

type State struct {
	cfg      ProjectConfig
	running  bool
	position SampleTick
	Tracks   []Track
	Markers  []Marker
}

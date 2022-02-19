package main

type Voice struct {
	Name    string
	Note    int
	Channel int // [1,16] if defined; use device channel it not set

	device *Device // backpointer
}

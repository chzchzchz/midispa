package main

import (
	"github.com/chzchzchz/midispa/sysex/akai"
)

var oledRed = [3]int{127, 0, 0}
var oledOrange = [3]int{127, 64, 0}
var oledYellow = [3]int{127, 127, 0}
var oledChartreuse = [3]int{64, 127, 0}
var oledGreen = [3]int{0, 127, 0}
var oledLime = [3]int{0, 127, 64}
var oledCyan = [3]int{0, 127, 127}
var oledAzure = [3]int{0, 64, 127}
var oledBlue = [3]int{0, 0, 127}
var oledViolet = [3]int{64, 0, 127}
var oledMagenta = [3]int{127, 0, 127}
var oledWhite = [3]int{127, 127, 127}
var oledGray = [3]int{64, 64, 64}
var oledBlack = [3]int{0, 0, 0}

var oledColorTable = [][3]int{
	oledRed,
	oledOrange,
	oledYellow,
	oledChartreuse,
	oledGreen,
	oledLime,
	oledCyan,
	oledAzure,
	oledBlue,
	oledViolet,
	oledMagenta,
	oledWhite,
	oledGray,
}

func Dim(c [3]int, n int) [3]int {
	return [3]int{c[0] / n, c[1] / n, c[2] / n}
}

// eachChannel applies a rule to one channel at a time, which is how every step colour in
// this package is stated. The three channels are independent, so a rule that needs to see
// them together is a different kind of rule and does not belong here.
func eachChannel(color [3]int, rule func(int) int) [3]int {
	var out [3]int
	for i, value := range color {
		out[i] = rule(value)
	}
	return out
}

func makePad(x, y int, c [3]int) akai.Pad {
	return akai.Pad{Idx: y*16 + x, Red: c[0], Green: c[1], Blue: c[2]}
}

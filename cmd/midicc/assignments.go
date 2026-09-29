package main

import (
	"fmt"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/util"
)

type Assignments struct {
	Title     string
	InDevice  string
	OutDevice string
	Maps      [][2]string // in, out

	in2out map[string]*Mapping
	out2in map[string]string

	saIn  alsa.SeqAddr
	saOut alsa.SeqAddr

	writeback bool
}

func (a *Assignments) feedback(write func(alsa.SeqEvent) error, data []byte) error {
	if !a.writeback {
		return nil
	}
	return write(alsa.SeqEvent{SeqAddr: a.saIn, Data: data})
}

// armPrefix marks a target that arms other inputs instead of driving a
// control, as in "Arm:Knob1,Knob2" or "Arm:*" for every direct mapping.
const armPrefix = "Arm:"

type Mapping struct {
	OutControl string
	Channel    int
	MayArm     bool // button controls when this is used
	Armed      bool // assignment is active

	arms []string
}

// setupMap builds the in2out and out2in lookups. A target is one of
// "control", "control/channel" or "Arm:a,b", where the last arms other inputs
// when this one is pressed.
func (a *Assignments) setupMap() error {
	mayArm, mayArmAll := make(map[string]struct{}), false
	a.in2out = make(map[string]*Mapping)
	a.out2in = make(map[string]string)
	for _, v := range a.Maps {
		control, channel, hasChannel := strings.Cut(v[1], "/")
		m := &Mapping{OutControl: v[1], Armed: true}
		if hasChannel {
			m.OutControl = control
			if _, err := fmt.Sscanf(channel, "%d", &m.Channel); err != nil {
				return fmt.Errorf("map %q: target %q has no channel number: %w", v[0], v[1], err)
			}
		} else if arg, ok := strings.CutPrefix(v[1], armPrefix); ok {
			// The armed inputs are what follows "Arm:", not what precedes the
			// colon, which is just the prefix itself.
			m.OutControl = ""
			m.arms = strings.Split(arg, ",")
			if arg == "*" {
				mayArmAll = true
			} else {
				for _, s := range m.arms {
					mayArm[s] = struct{}{}
				}
			}
		}
		a.in2out[v[0]] = m
		if m.OutControl != "" {
			a.out2in[m.OutControl] = v[0]
		}
	}
	for k, m := range a.in2out {
		if mayArmAll && len(m.arms) == 0 {
			m.MayArm = true
		} else if _, ok := mayArm[k]; ok {
			m.MayArm = true
		}
	}
	return nil
}

func (a *Assignments) Enable() {
	for _, m := range a.in2out {
		m.Armed = !m.MayArm
	}
}

func (a *Assignments) Arm(in string) (ret []string) {
	m := a.in2out[in]
	if m == nil {
		return nil
	}
	for _, v := range m.arms {
		if v == "*" {
			for k, m2 := range a.in2out {
				if m2.Armed {
					m2.Armed = true
					ret = append(ret, k)
				}
			}
			break
		}
		if m2 := a.in2out[v]; m2 != nil && !m2.Armed {
			m2.Armed = true
			ret = append(ret, v)
		}
	}
	return ret
}

func (a *Assignments) InToOut(in string) (string, int) {
	if m := a.in2out[in]; m != nil {
		if !m.Armed {
			return "", -1
		}
		return m.OutControl, m.Channel
	}
	return "", -1
}

func mustLoadAssignments(path string) (m []Assignments) {
	m = util.MustLoadJSONFile[Assignments](path)
	for i := range m {
		if err := m[i].setupMap(); err != nil {
			panic(fmt.Sprintf("%s: assignment %d (%q): %v", path, i, m[i].Title, err))
		}
	}
	return m
}

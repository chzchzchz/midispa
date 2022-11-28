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

	saOut alsa.SeqAddr
}

type Mapping struct {
	OutControl string
	Channel    int
	MayArm     bool // button controls when this is used
	Armed      bool // assignment is active

	arms []string
}

func (a *Assignments) setupMap() {
	mayArm, mayArmAll := make(map[string]struct{}), false
	a.in2out = make(map[string]*Mapping)
	for _, v := range a.Maps {
		slash := strings.Split(v[1], "/")
		m := &Mapping{OutControl: v[1], Armed: true}
		if len(slash) > 1 {
			m.OutControl = slash[0]
			if _, err := fmt.Sscanf(slash[1], "%d", &m.Channel); err != nil {
				panic(err)
			}
		} else if strings.HasPrefix(v[1], "Arm:") {
			arg := strings.Split(v[1], ":")[0]
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
	}
	for k, m := range a.in2out {
		if mayArmAll && len(m.arms) == 0 {
			m.MayArm = true
		} else if _, ok := mayArm[k]; ok {
			m.MayArm = true
		}
	}
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
		m[i].setupMap()
	}
	return m
}

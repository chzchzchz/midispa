package jack

import (
	"log"
	"strings"
	"sync"

	"github.com/xthexder/go-jack"
)

type Port struct {
	PortConfig

	f  JackProcessCallback
	fl uint64

	Client *jack.Client

	portExternal map[string]*jack.Port
	portInternal *jack.Port

	portc      chan *jack.Port
	connecting bool
	wg         sync.WaitGroup
}

type PortConfig struct {
	ClientName string
	PortName   string

	MatchName []string
}

func (pc *PortConfig) isNameMatch(s string) bool {
	ret := false
	for _, mn := range pc.MatchName {
		ret = ret || strings.Contains(s, mn)
	}
	return ret
}

type JackProcessCallback func([]jack.AudioSample) int

func NewReadPort(pc PortConfig, f JackProcessCallback) (*Port, error) {
	return NewJackPort(pc, jack.PortIsInput|jack.PortIsTerminal, f)
}

func NewWritePort(pc PortConfig, f JackProcessCallback) (*Port, error) {
	return NewJackPort(pc, jack.PortIsOutput|jack.PortIsTerminal, f)
}

func (j *Port) GetBuffer(nf int) []jack.AudioSample {
	return j.portInternal.GetBuffer(uint32(nf))
}

func NewJackPort(pc PortConfig, fl uint64, f JackProcessCallback) (*Port, error) {
	client, status := jack.ClientOpen(pc.ClientName, jack.NoStartServer)
	if status != 0 {
		return nil, jack.StrError(status)
	}
	j := &Port{
		PortConfig:   pc,
		Client:       client,
		portExternal: make(map[string]*jack.Port),
		f:            f,
		fl:           fl,
		portc:        make(chan *jack.Port, 2),
	}
	if code := j.Client.SetPortRegistrationCallback(j.portRegistration); code != 0 {
		j.Client.Close()
		return nil, jack.StrError(code)
	}
	if code := client.SetProcessCallback(j.process); code != 0 {
		j.Client.Close()
		return nil, jack.StrError(code)
	}
	if code := client.SetPortConnectCallback(j.portConnect); code != 0 {
		j.Client.Close()
		return nil, jack.StrError(code)
	}
	if code := client.Activate(); code != 0 {
		j.Client.Close()
		return nil, jack.StrError(code)
	}
	log.Println("jack activated")
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		for p := range j.portc {
			j.connectExternal(p)
		}
	}()

	j.portInternal = j.Client.PortRegister(pc.PortName, jack.DEFAULT_AUDIO_TYPE, fl, 8192)

	for _, mn := range j.MatchName {
		srcs := j.ports(mn)
		for _, src := range srcs {
			if err := j.connectExternal(src); err != nil {
				j.Close()
				return nil, err
			}
		}
		if len(srcs) == 0 {
			log.Printf("matching port not found on %s; will wait to register", mn)
		}
	}
	return j, nil
}

func (j *Port) process(nFrames uint32) int {
	if len(j.portExternal) == 0 {
		return 0
	}
	return j.f(j.portInternal.GetBuffer(nFrames))
}

func (j *Port) portConnect(a, b jack.PortId, is_connect bool) {}

func (j *Port) portRegistration(id jack.PortId, made bool) {
	p := j.Client.GetPortById(id)
	name := p.GetName()
	if !made {
		if _, ok := j.portExternal[name]; ok {
			log.Println("unregistered:", name)
			delete(j.portExternal, name)
		}
		return
	}
	if strings.HasPrefix(name, j.ClientName) || !j.isNameMatch(name) {
		log.Println("ignoring non-match:", name)
	} else if _, ok := j.portExternal[name]; ok || len(j.portc) > 0 || j.connecting {
		log.Println("ignoring match:", name)
	} else {
		log.Println("matched:", name)
		j.connecting = true
		j.portc <- p
	}
}

func (j *Port) ports(name string) (ret []*jack.Port) {
	for _, port := range j.Client.GetPorts(name, "", 0) {
		if !strings.HasPrefix(port, j.ClientName) {
			p := j.Client.GetPortByName(port)
			ret = append(ret, p)
		}
	}
	return ret
}

func (j *Port) connectExternal(ext *jack.Port) error {
	src, dst := j.portInternal, ext
	if j.fl&jack.PortIsOutput != 0 {
		dst, src = ext, j.portInternal
	}
	log.Printf("connecting src=%q to dst=%q", src.GetName(), dst.GetName())
	if code := j.Client.ConnectPorts(src, dst); code != 0 {
		log.Println("failed to connect ports")
		return jack.StrError(code)
	}
	j.connecting, j.portExternal[ext.GetName()] = false, ext
	return nil
}

func LogPorts() {
	client, status := jack.ClientOpen("logports", jack.NoStartServer)
	if status != 0 {
		panic("couldn't open jack client")
	}
	defer client.Close()
	for _, port := range client.GetPorts("", "", 0) {
		p := client.GetPortByName(port)
		log.Println("port:", p.GetName())
	}
}

func (j *Port) Close() {
	j.Client.Deactivate()
	j.Client.Close()
	close(j.portc)
	j.wg.Wait()
}

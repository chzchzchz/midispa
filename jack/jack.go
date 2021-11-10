package jack

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/xthexder/go-jack"
)

type Port struct {
	extName    string
	clientName string
	f          JackProcessCallback
	fl         uint64

	client       *jack.Client
	portExternal *jack.Port
	portInternal *jack.Port

	portc      chan *jack.Port
	connecting bool
	wg         sync.WaitGroup
}

type JackProcessCallback func([]jack.AudioSample) int

func NewReadPort(cn, extName string, f JackProcessCallback) (*Port, error) {
	return NewJackPort(cn, extName, jack.PortIsInput|jack.PortIsTerminal, f)
}

func NewWritePort(cn, extName string, f JackProcessCallback) (*Port, error) {
	return NewJackPort(cn, extName, jack.PortIsOutput|jack.PortIsTerminal, f)
}

func (j *Port) GetBuffer(nf int) []jack.AudioSample {
	return j.portInternal.GetBuffer(uint32(nf))
}

func NewJackPort(cn, extName string, fl uint64, f JackProcessCallback) (*Port, error) {
	client, status := jack.ClientOpen(cn, jack.NoStartServer)
	if status != 0 {
		return nil, jack.StrError(status)
	}
	j := &Port{
		extName:    extName,
		clientName: cn,
		client:     client,
		f:          f,
		fl:         fl,
		portc:      make(chan *jack.Port, 2),
	}
	if code := j.client.SetPortRegistrationCallback(j.portRegistration); code != 0 {
		j.client.Close()
		return nil, jack.StrError(code)
	}
	if code := client.SetProcessCallback(j.process); code != 0 {
		j.client.Close()
		return nil, jack.StrError(code)
	}
	if code := client.SetPortConnectCallback(j.portConnect); code != 0 {
		j.client.Close()
		return nil, jack.StrError(code)
	}
	if code := client.Activate(); code != 0 {
		j.client.Close()
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

	j.portInternal = j.client.PortRegister(
		fmt.Sprintf("%s-%s", j.clientName, extName),
		jack.DEFAULT_AUDIO_TYPE,
		fl,
		8192)

	if srcs := j.ports(j.extName); len(srcs) > 0 {
		if err := j.connectExternal(srcs[0]); err != nil {
			j.Close()
			return nil, err
		}
	} else {
		log.Println("matching port not found; will wait to register")
	}
	return j, nil
}

func (j *Port) process(nFrames uint32) int {
	if j.portExternal == nil {
		return 0
	}
	return j.f(j.portInternal.GetBuffer(nFrames))
}

func (j *Port) portConnect(a, b jack.PortId, is_connect bool) {}

func (j *Port) portRegistration(id jack.PortId, made bool) {
	p := j.client.GetPortById(id)
	name := p.GetName()
	if !made {
		if j.portExternal != nil && name == j.portExternal.GetName() {
			log.Println("unregistered:", name)
			j.portExternal = nil
		}
		return
	}
	if strings.HasPrefix(name, j.clientName) || !strings.Contains(name, j.extName) {
		log.Println("ignoring non-match:", name)
	} else if j.portExternal != nil || len(j.portc) > 0 || j.connecting {
		log.Println("ignoring match:", name)
	} else {
		log.Println("matched:", name)
		j.connecting = true
		j.portc <- p
	}
}

func (j *Port) ports(name string) (ret []*jack.Port) {
	for _, port := range j.client.GetPorts(name, "", 0) {
		if !strings.HasPrefix(port, j.clientName) {
			p := j.client.GetPortByName(port)
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
	if code := j.client.ConnectPorts(src, dst); code != 0 {
		log.Println("failed to connect ports")
		return jack.StrError(code)
	}
	j.connecting, j.portExternal = false, ext
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
	j.client.Deactivate()
	j.client.Close()
	close(j.portc)
	j.wg.Wait()
}

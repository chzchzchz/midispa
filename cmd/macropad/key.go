package main

type CC struct {
	cc   int
	data int
}

type SetCCFunc func(bool, *CC)
type SetRGBFunc func(bool, *RgbQueue)

type Key struct {
	idx    int
	on     bool
	rgbOn  [3]byte
	rgbOff [3]byte
	desc   string
	setCC  SetCCFunc
	setRGB SetRGBFunc

	*CC
	bank *Bank
}

type Bank struct {
	keys []*Key
}

func (b *Bank) add(k *Key) {
	if k.bank != nil {
		panic("bank already assigned")
	}
	b.keys = append(b.keys, k)
	k.bank = b
}

func (k *Key) updateCC() {
	if k.setCC != nil {
		k.setCC(k.on, k.CC)
		return
	}
	if k.on {
		k.data = 0x7f
	} else {
		k.data = 0
	}
}

func (k *Key) updateRGB(rgb *RgbQueue) {
	c := k.rgbOn
	if !k.on {
		c = k.rgbOff
	}
	rgb.Write(k.idx, c)
}

func (k *Key) off(rgb *RgbQueue) {
	rgb.Write(k.idx, [3]byte{0, 0, 0})
}

func (k *Key) toggle(rgb *RgbQueue) {
	k.on = !k.on
	// At most one key may be active for a bank.
	if b := k.bank; k.on && b != nil {
		for _, kk := range b.keys {
			if kk.on && kk != k {
				kk.on = false
				kk.updateCC()
				kk.updateRGB(rgb)
			}
		}
	}
	k.updateCC()
	k.updateRGB(rgb)
	if k.setRGB != nil {
		k.setRGB(k.on, rgb)
	}
}

func off(rgb *RgbQueue) {
	for i := 0; i < 24; i++ {
		rgb.Write(i, [3]byte{0, 0, 0})
	}
}

func reset(rgb *RgbQueue, keys []*Key) {
	for _, k := range keys {
		if k != nil {
			k.updateRGB(rgb)
		}
	}
}

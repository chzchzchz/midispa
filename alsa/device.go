package alsa

import (
	amidi "github.com/scgolang/midi"
)

func openBy(f func(d *amidi.Device) bool) (*amidi.Device, error) {
	devs, err := amidi.Devices()
	if err != nil {
		return nil, err
	}
	for _, dev := range devs {
		if f(dev) {
			if err = dev.Open(); err != nil {
				return nil, err
			}
			return dev, nil
		}
	}
	return nil, nil
}

func OpenDeviceById(id string) (*amidi.Device, error) {
	return openBy(func(d *amidi.Device) bool { return d.ID == id })
}

func OpenDeviceByName(name string) (d *amidi.Device, err error) {
	return openBy(func(d *amidi.Device) bool { return d.Name == name })
}

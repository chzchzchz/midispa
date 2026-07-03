package wav

import (
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

func WriteFile(path string, data []float32, rate int) error {
	wf, err := OpenWriter(path, rate)
	if err != nil {
		return err
	}
	if err := wf(data); err != nil {
		return err
	}
	return wf(nil)
}

type WriteFunc func([]float32) error

func OpenWriter(path string, rate int) (WriteFunc, error) {
	w, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, err
	}
	enc := wav.NewEncoder(w, rate, 16, 1 /* chans */, 1 /* fmt */)
	wf := func(d []float32) error {
		if len(d) == 0 {
			if err := enc.Close(); err != nil {
				return err
			}
			return w.Close()
		}
		// wav encoder will normalize ints to [-1.0,1.0] but won't expand back.
		renormalizedData := make([]float32, len(d))
		for i := range d {
			renormalizedData[i] = d[i] * float32((1<<15)-1)
		}
		buf := audio.PCMBuffer{
			Format:         &audio.Format{NumChannels: 1, SampleRate: rate},
			F32:            renormalizedData,
			DataType:       audio.DataTypeF32,
			SourceBitDepth: 2,
		}
		return enc.Write(buf.AsIntBuffer())
	}
	return wf, nil
}

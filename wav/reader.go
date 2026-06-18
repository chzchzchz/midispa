package wav

import (
	"io"
	"os"

	"github.com/go-audio/audio"
	gowav "github.com/go-audio/wav"
)

type WavReader struct {
	dec      *gowav.Decoder
	f        *os.File
	format   *audio.Format
	depth    int
	pcmStart int64
}

func OpenReader(path string) (*WavReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	dec := gowav.NewDecoder(f)
	dec.ReadInfo()
	if dec.Err() != nil {
		f.Close()
		return nil, dec.Err()
	}
	if err := dec.FwdToPCM(); err != nil {
		f.Close()
		return nil, err
	}
	pcmStart, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &WavReader{
		dec:      dec,
		f:        f,
		format:   dec.Format(),
		depth:    int(dec.SampleBitDepth()),
		pcmStart: pcmStart,
	}, nil
}

func (r *WavReader) Read(buf []int) (int, error) {
	ibuf := &audio.IntBuffer{
		Data:           buf,
		Format:         r.format,
		SourceBitDepth: r.depth,
	}
	n, err := r.dec.PCMBuffer(ibuf)
	if n < len(buf) {
		for i := n; i < len(buf); i++ {
			buf[i] = 0
		}
	}
	return n, err
}

func (r *WavReader) Seek(samples, whence int) error {
	// Rewind is needed because of the way the decoder handles buffering.
	if err := r.dec.Rewind(); err != nil {
		return err
	}
	bps := (r.depth-1)/8 + 1
	byteOffset := int64(samples) * int64(bps)
	switch whence {
	case io.SeekStart:
		byteOffset += r.pcmStart
	case io.SeekEnd:
		byteOffset += r.pcmStart + int64(r.dec.PCMLen())
	default:
		panic("bad whence")
	}
	_, err := r.dec.Seek(byteOffset, io.SeekStart)
	return err
}

func (r *WavReader) Samples() int {
	bps := (r.depth-1)/8 + 1
	return int(r.dec.PCMLen() / int64(bps))
}

func (r *WavReader) Close() {
	r.f.Close()
}

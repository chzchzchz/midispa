package wav

import (
	"io"
	"os"

	"github.com/go-audio/audio"
	gowav "github.com/go-audio/wav"
)

type reader struct {
	dec      *gowav.Decoder
	f        *os.File
	format   *audio.Format
	depth    int
	pcmStart int64
	pos      int
}

type Reader interface {
	Read(buf []int) (int, error)
	Seek(samples, whence int) error
	Close() error
}

type sliceReader struct {
	s   []int
	pos int
}

func NewSliceReader(s []int) Reader { return &sliceReader{s: s} }

func (sr *sliceReader) Seek(off, whence int) error {
	if whence != io.SeekStart {
		panic("unsupported whence")
	}
	sr.pos = off
	return nil
}

func (sr *sliceReader) Read(buf []int) (int, error) {
	end := min(sr.pos+len(buf), len(sr.s))
	n := end - sr.pos
	copy(buf, sr.s[sr.pos:end])
	sr.pos = end
	if sr.pos == len(sr.s) {
		return n, io.EOF
	}
	return n, nil
}

func (sr *sliceReader) Close() error { return nil }

func OpenReader(path string) (Reader, error) {
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
	return &reader{
		dec:      dec,
		f:        f,
		format:   dec.Format(),
		depth:    int(dec.SampleBitDepth()),
		pcmStart: pcmStart,
	}, nil
}

func (r *reader) Read(buf []int) (int, error) {
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
	r.pos += n
	return n, err
}

func (r *reader) Seek(samples, whence int) error {
	if whence == io.SeekStart && samples == r.pos {
		// No need to rewind/seek.
		return nil
	}
	// Rewind is needed because of the way the decoder handles buffering.
	if err := r.dec.Rewind(); err != nil {
		return err
	}
	bps := (r.depth-1)/8 + 1
	byteOffset := int64(samples) * int64(bps)
	switch whence {
	case io.SeekStart:
		byteOffset += r.pcmStart
	default:
		panic("bad whence")
	}
	_, err := r.dec.Seek(byteOffset, io.SeekStart)
	return err
}

func (r *reader) Samples() int {
	bps := (r.depth-1)/8 + 1
	return int(r.dec.PCMLen() / int64(bps))
}

func (r *reader) Close() error { return r.f.Close() }

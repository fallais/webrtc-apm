// Package framer rebuffers arbitrary-sized PCM chunks into fixed-size
// frames suitable for the WebRTC audio_processing module's 10ms frame
// contract. It is pure Go and has no system dependencies.
package framer

// Framer holds buffered samples and emits frames of a fixed size.
// Framer is not safe for concurrent use.
type Framer struct {
	frameSize int
	buf       []int16
}

// New returns a Framer that emits frames of frameSize int16 samples.
// Pass the value returned by apm.Processor.SamplesPer10ms() to match a
// Processor.
func New(frameSize int) *Framer {
	return &Framer{
		frameSize: frameSize,
		buf:       make([]int16, 0, frameSize*4),
	}
}

// Push appends samples to the internal buffer.
func (f *Framer) Push(samples []int16) {
	f.buf = append(f.buf, samples...)
}

// Pop copies the next frame into dst and returns true. Returns false
// when len(dst) does not match the configured frame size or fewer
// samples than required are buffered.
func (f *Framer) Pop(dst []int16) bool {
	if len(dst) != f.frameSize || len(f.buf) < f.frameSize {
		return false
	}
	copy(dst, f.buf[:f.frameSize])
	n := copy(f.buf, f.buf[f.frameSize:])
	f.buf = f.buf[:n]
	return true
}

// FrameSize reports the frame size configured at construction.
func (f *Framer) FrameSize() int { return f.frameSize }

// Buffered returns the number of samples currently queued.
func (f *Framer) Buffered() int { return len(f.buf) }

// Reset drops all buffered samples.
func (f *Framer) Reset() { f.buf = f.buf[:0] }

package crnnoise

/*
#cgo pkg-config: rnnoise

#include "crnnoise.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// Processor wraps a single rnnoise DenoiseState.
type Processor struct {
	p *C.crnnoise_t
}

// New constructs a Processor using the default RNN model bundled with
// rnnoise.
func New() (*Processor, error) {
	p := C.crnnoise_create()
	if p == nil {
		return nil, errors.New("crnnoise: rnnoise_create returned NULL")
	}
	return &Processor{p: p}, nil
}

// Close releases the native state.
func (p *Processor) Close() {
	if p == nil || p.p == nil {
		return
	}
	C.crnnoise_destroy(p.p)
	p.p = nil
}

// FrameSize is the fixed number of float samples per call. rnnoise
// operates at 48 kHz mono so this is 480 (10 ms).
func FrameSize() int { return int(C.crnnoise_frame_size()) }

// ProcessFrame denoises in into out and returns the voice-activity
// probability in [0,1]. in and out must both have length FrameSize().
// In-place use (out == in) is supported.
func (p *Processor) ProcessFrame(out, in []float32) (float32, error) {
	expected := FrameSize()
	if len(in) != expected || len(out) != expected {
		return 0, errors.New("crnnoise: ProcessFrame requires len==FrameSize() (480)")
	}
	prob := C.crnnoise_process_frame(
		p.p,
		(*C.float)(unsafe.Pointer(&out[0])),
		(*C.float)(unsafe.Pointer(&in[0])),
	)
	return float32(prob), nil
}

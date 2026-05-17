package capm

/*
#cgo pkg-config: webrtc-audio-processing-2
#cgo CXXFLAGS: -std=c++17 -fPIC

#include "capm.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// Config mirrors capm_config_t.
type Config struct {
	SampleRateHz int
	NumChannels  int

	EnableAEC bool
	EnableNS  bool
	NSLevel   int

	EnableAGC bool
	AGCMode   int
}

// Processor wraps the native capm_t handle.
type Processor struct {
	p *C.capm_t
}

// New constructs a native Processor.
func New(cfg Config) (*Processor, error) {
	c := C.capm_config_t{
		sample_rate_hz: C.int(cfg.SampleRateHz),
		num_channels:   C.int(cfg.NumChannels),
		enable_aec:     boolToInt(cfg.EnableAEC),
		enable_ns:      boolToInt(cfg.EnableNS),
		ns_level:       C.int(cfg.NSLevel),
		enable_agc:     boolToInt(cfg.EnableAGC),
		agc_mode:       C.int(cfg.AGCMode),
	}
	p := C.capm_create(&c)
	if p == nil {
		return nil, errors.New("capm: native AudioProcessing creation failed")
	}
	return &Processor{p: p}, nil
}

// Close releases the native handle.
func (p *Processor) Close() {
	if p == nil || p.p == nil {
		return
	}
	C.capm_destroy(p.p)
	p.p = nil
}

// ProcessStream processes one 10ms near-end frame in place.
func (p *Processor) ProcessStream(frame []int16) error {
	if len(frame) == 0 {
		return nil
	}
	rc := C.capm_process_stream(
		p.p,
		(*C.int16_t)(unsafe.Pointer(&frame[0])),
		C.int(len(frame)),
	)
	if rc != 0 {
		return &Error{Op: "capm_process_stream", Code: int(rc)}
	}
	return nil
}

// ProcessReverseStream supplies the far-end reference frame for AEC.
func (p *Processor) ProcessReverseStream(frame []int16) error {
	if len(frame) == 0 {
		return nil
	}
	rc := C.capm_process_reverse_stream(
		p.p,
		(*C.int16_t)(unsafe.Pointer(&frame[0])),
		C.int(len(frame)),
	)
	if rc != 0 {
		return &Error{Op: "capm_process_reverse_stream", Code: int(rc)}
	}
	return nil
}

func boolToInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// Error wraps a non-zero return code from the native bridge.
type Error struct {
	Op   string
	Code int
}

func (e *Error) Error() string {
	return e.Op + ": native error code " + itoa(e.Code)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

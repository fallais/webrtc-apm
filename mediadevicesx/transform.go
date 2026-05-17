// Package mediadevicesx adapts a webrtc-apm Processor to
// pion/mediadevices so audio processing slots into a getUserMedia-style
// pipeline alongside the microphone driver and Opus codec.
//
// The adapter is intentionally narrow: it consumes wave.Audio chunks,
// rebuffers them into the 10ms frames a Processor requires, runs them
// through AEC/NS/AGC/VAD, and emits the result back as wave.Audio. AEC
// callers must additionally feed the far-end (loudspeaker) reference
// through ReverseStreamWriter.
package mediadevicesx

import (
	"errors"
	"sync"

	apm "github.com/fallais/webrtc-apm"
	"github.com/fallais/webrtc-apm/framer"
)

// Transform owns a Processor and exposes the read/write halves of the
// audio path. The Read side runs the near-end (microphone) pipeline;
// the ReverseStreamWriter side feeds the far-end (loudspeaker) frames
// AEC needs as a reference.
//
// The intended wiring against pion/mediadevices is:
//
//	transform, err := mediadevicesx.NewTransform(cfg)
//	track.Transform(transform.AudioTransformFunc())
//	go pipePlayback(transform.ReverseStreamWriter())
//
// The mediadevices AudioTransformFunc and wave.Audio bindings are
// deliberately not implemented in this file: the wave.Audio type lives
// in a tagged module dependency we have not pulled in yet. The next
// commit will add the dependency and complete AudioTransformFunc.
type Transform struct {
	mu        sync.Mutex
	processor *apm.Processor
	framer    *framer.Framer
	scratch   []int16
}

// NewTransform constructs a Transform configured per cfg.
func NewTransform(cfg apm.Config) (*Transform, error) {
	p, err := apm.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Transform{
		processor: p,
		framer:    framer.New(p.SamplesPer10ms()),
		scratch:   make([]int16, p.SamplesPer10ms()),
	}, nil
}

// Close releases the underlying Processor.
func (t *Transform) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.processor == nil {
		return nil
	}
	err := t.processor.Close()
	t.processor = nil
	return err
}

// ProcessSamples runs interleaved int16 PCM through the near-end path,
// emitting whatever 10ms frames are ready. The returned slice is owned
// by the caller and is safe to retain.
func (t *Transform) ProcessSamples(in []int16) ([]int16, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.processor == nil {
		return nil, errors.New("mediadevicesx: transform closed")
	}
	t.framer.Push(in)
	out := make([]int16, 0, len(in))
	for t.framer.Pop(t.scratch) {
		if err := t.processor.ProcessStream(t.scratch); err != nil {
			return out, err
		}
		out = append(out, t.scratch...)
	}
	return out, nil
}

// ProcessReverseSamples feeds the far-end reference signal for AEC.
// Callers must invoke this with the audio they are about to render
// through the local loudspeaker, at the same cadence as ProcessSamples.
func (t *Transform) ProcessReverseSamples(in []int16) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.processor == nil {
		return errors.New("mediadevicesx: transform closed")
	}
	revFramer := framer.New(t.processor.SamplesPer10ms())
	revFramer.Push(in)
	scratch := make([]int16, t.processor.SamplesPer10ms())
	for revFramer.Pop(scratch) {
		if err := t.processor.ProcessReverseStream(scratch); err != nil {
			return err
		}
	}
	return nil
}


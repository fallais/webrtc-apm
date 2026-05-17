// Package apm wraps webrtc-audio-processing 1.x with an optional
// rnnoise DNN pre-stage so Go programs can run production-grade voice
// processing — echo cancellation, noise suppression and automatic gain
// control — on real-time PCM frames.
package apm

import (
	"github.com/fallais/webrtc-apm/internal/capm"
	"github.com/fallais/webrtc-apm/internal/crnnoise"
)

// NSLevel controls noise-suppression aggressiveness.
type NSLevel int

const (
	NSLow NSLevel = iota
	NSModerate
	NSHigh
	NSVeryHigh
)

// AGCMode controls automatic-gain-control behaviour.
type AGCMode int

const (
	AGCAdaptiveAnalog AGCMode = iota
	AGCAdaptiveDigital
	AGCFixedDigital
)

// Config configures a Processor.
//
// When EnableRNNoise is true, SampleRate must be 48000 and Channels
// must be 1 — rnnoise operates at a fixed 48 kHz mono rate. rnnoise
// runs as a pre-stage ahead of APM on every ProcessStream call.
type Config struct {
	SampleRate int // 8000, 16000, 32000 or 48000
	Channels   int // 1 or 2

	EnableRNNoise bool

	EnableAEC bool
	EnableNS  bool
	NSLevel   NSLevel
	EnableAGC bool
	AGCMode   AGCMode
}

// Processor applies the configured pipeline to 10 ms PCM frames.
//
// Every call to ProcessStream and ProcessReverseStream must supply
// exactly 10 ms of int16 PCM at the configured sample rate and channel
// count, interleaved when stereo. Use github.com/fallais/webrtc-apm/framer
// to rebuffer arbitrary chunks into 10 ms frames.
type Processor struct {
	apm            *capm.Processor
	rnn            *crnnoise.Processor // nil when EnableRNNoise=false
	samplesPer10ms int

	rnnIn  []float32
	rnnOut []float32
}

// New constructs a Processor configured per cfg.
func New(cfg Config) (*Processor, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	var (
		rnn            *crnnoise.Processor
		rnnIn, rnnOut  []float32
	)
	if cfg.EnableRNNoise {
		r, err := crnnoise.New()
		if err != nil {
			return nil, err
		}
		rnn = r
		sz := crnnoise.FrameSize()
		rnnIn = make([]float32, sz)
		rnnOut = make([]float32, sz)
	}

	inner, err := capm.New(capm.Config{
		SampleRateHz: cfg.SampleRate,
		NumChannels:  cfg.Channels,
		EnableAEC:    cfg.EnableAEC,
		EnableNS:     cfg.EnableNS,
		NSLevel:      int(cfg.NSLevel),
		EnableAGC:    cfg.EnableAGC,
		AGCMode:      int(cfg.AGCMode),
	})
	if err != nil {
		if rnn != nil {
			rnn.Close()
		}
		return nil, err
	}

	return &Processor{
		apm:            inner,
		rnn:            rnn,
		samplesPer10ms: cfg.SampleRate / 100 * cfg.Channels,
		rnnIn:          rnnIn,
		rnnOut:         rnnOut,
	}, nil
}

// SamplesPer10ms reports the frame size, in interleaved int16 samples,
// expected by ProcessStream and ProcessReverseStream.
func (p *Processor) SamplesPer10ms() int { return p.samplesPer10ms }

// Close releases native resources. Safe to call multiple times.
func (p *Processor) Close() error {
	if p == nil {
		return nil
	}
	if p.rnn != nil {
		p.rnn.Close()
		p.rnn = nil
	}
	if p.apm != nil {
		p.apm.Close()
		p.apm = nil
	}
	return nil
}

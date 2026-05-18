// Package pionapm bridges webrtc-apm into the pion/mediadevices audio
// pipeline. A Bridge wraps an apm.Processor and exposes a Transform()
// method that returns an audio.TransformFunc suitable for passing to
// (*mediadevices.AudioTrack).Transform.
//
// AEC limitation: pion/mediadevices' TransformFunc is one-way (one
// Reader in, one Reader out) and has no native slot for AEC's far-end
// reference signal. When apm.Config.EnableAEC is true, the caller must
// drive FeedReverse from their playback path so APM has the reference
// it needs. Without it, AEC will not converge.
package pionapm

import (
	"fmt"
	"sync"

	"github.com/pion/mediadevices/pkg/io/audio"
	"github.com/pion/mediadevices/pkg/wave"

	apm "github.com/fallais/webrtc-apm"
	"github.com/fallais/webrtc-apm/framer"
)

// Bridge couples an apm.Processor with the pion/mediadevices Transform
// contract. Safe for concurrent use across the Transform's Read
// goroutine and a caller-driven FeedReverse goroutine.
type Bridge struct {
	cfg       apm.Config
	frameSize int

	// procMu serialises every call into the underlying apm.Processor
	// (and therefore the C++ AudioProcessing instance, which is not
	// thread-safe across ProcessStream / ProcessReverseStream).
	procMu sync.Mutex
	proc   *apm.Processor
	cap    *framer.Framer
	cWork  []int16
	rev    *framer.Framer
	rWork  []int16
}

// New constructs a Bridge configured per cfg. The Bridge owns the
// underlying apm.Processor; call Close to release native resources.
func New(cfg apm.Config) (*Bridge, error) {
	p, err := apm.New(cfg)
	if err != nil {
		return nil, err
	}
	fs := p.SamplesPer10ms()
	return &Bridge{
		proc:      p,
		cfg:       cfg,
		frameSize: fs,
		cap:       framer.New(fs),
		cWork:     make([]int16, fs),
		rev:       framer.New(fs),
		rWork:     make([]int16, fs),
	}, nil
}

// Close releases native resources. Safe to call multiple times.
func (b *Bridge) Close() error {
	return b.proc.Close()
}

// Transform returns an audio.TransformFunc suitable for passing to
// (*mediadevices.AudioTrack).Transform.
//
//	bridge, _ := pionapm.New(cfg)
//	defer bridge.Close()
//	track.Transform(bridge.Transform())
//
// The transform accepts any of the four wave.Audio variants
// (Int16Interleaved, Int16NonInterleaved, Float32Interleaved,
// Float32NonInterleaved) and emits chunks of the same variant. It
// requires the upstream chunk's SampleRate and Channels to match the
// apm.Config passed to New; mismatches produce a Read error.
//
// Output chunks may be larger or smaller than input chunks: the
// transform internally rebuffers upstream input into 10 ms frames for
// APM and emits whatever was processed in each Read. Downstream
// encoders (Opus, etc.) handle variable chunk sizes correctly.
func (b *Bridge) Transform() audio.TransformFunc {
	return func(src audio.Reader) audio.Reader {
		return audio.ReaderFunc(func() (wave.Audio, func(), error) {
			out := make([]int16, 0, b.frameSize*4)
			var ci wave.ChunkInfo
			var firstChunk wave.Audio
			for {
				chunk, release, err := src.Read()
				if err != nil {
					return nil, nil, err
				}
				samples, cc, err := toInt16Interleaved(chunk)
				if release != nil {
					release()
				}
				if err != nil {
					return nil, nil, err
				}
				if cc.SamplingRate != b.cfg.SampleRate || cc.Channels != b.cfg.Channels {
					return nil, nil, fmt.Errorf("pionapm: chunk %d Hz / %d ch does not match apm.Config %d Hz / %d ch",
						cc.SamplingRate, cc.Channels, b.cfg.SampleRate, b.cfg.Channels)
				}
				if firstChunk == nil {
					firstChunk = chunk
					ci = cc
				}

				b.procMu.Lock()
				b.cap.Push(samples)
				var perr error
				for b.cap.Pop(b.cWork) {
					if e := b.proc.ProcessStream(b.cWork); e != nil {
						perr = e
						break
					}
					out = append(out, b.cWork...)
				}
				b.procMu.Unlock()

				if perr != nil {
					return nil, nil, perr
				}
				if len(out) > 0 {
					break
				}
				// Buffered but not enough samples for a frame yet; loop and read more.
			}

			emitted := fromInt16Interleaved(firstChunk, out, ci)
			return emitted, func() {}, nil
		})
	}
}

// FeedReverse supplies far-end (loudspeaker / playback) audio for AEC.
// Drive it from your playback path in 10 ms or larger chunks of int16
// PCM. Samples must be at apm.Config.SampleRate and apm.Config.Channels;
// the caller is responsible for matching those (a slice of int16
// carries no rate information for us to validate).
//
// Cadence should track the near-end Read rate roughly: APM correlates
// the reverse stream against the near-end signal to estimate the
// round-trip delay, and large drifts between the two will slow
// convergence.
//
// When cfg.EnableAEC is false, FeedReverse is a no-op.
func (b *Bridge) FeedReverse(samples []int16) error {
	if !b.cfg.EnableAEC {
		return nil
	}
	if len(samples) == 0 {
		return nil
	}
	if len(samples)%b.cfg.Channels != 0 {
		return fmt.Errorf("pionapm: FeedReverse: len(samples)=%d is not a multiple of Channels=%d",
			len(samples), b.cfg.Channels)
	}
	b.procMu.Lock()
	defer b.procMu.Unlock()
	b.rev.Push(samples)
	for b.rev.Pop(b.rWork) {
		if err := b.proc.ProcessReverseStream(b.rWork); err != nil {
			return err
		}
	}
	return nil
}

// toInt16Interleaved unwraps any wave.Audio variant into an interleaved
// []int16 slice. Float samples are clamped to the int16 range.
func toInt16Interleaved(chunk wave.Audio) ([]int16, wave.ChunkInfo, error) {
	ci := chunk.ChunkInfo()
	if ci.Channels == 0 || ci.Len == 0 {
		return nil, ci, nil
	}
	n := ci.Len * ci.Channels
	switch a := chunk.(type) {
	case *wave.Int16Interleaved:
		out := make([]int16, n)
		copy(out, a.Data)
		return out, ci, nil

	case *wave.Int16NonInterleaved:
		out := make([]int16, n)
		for i := 0; i < ci.Len; i++ {
			for ch := 0; ch < ci.Channels; ch++ {
				out[i*ci.Channels+ch] = a.Data[ch][i]
			}
		}
		return out, ci, nil

	case *wave.Float32Interleaved:
		out := make([]int16, n)
		for i, f := range a.Data {
			out[i] = floatToInt16(f)
		}
		return out, ci, nil

	case *wave.Float32NonInterleaved:
		out := make([]int16, n)
		for i := 0; i < ci.Len; i++ {
			for ch := 0; ch < ci.Channels; ch++ {
				out[i*ci.Channels+ch] = floatToInt16(a.Data[ch][i])
			}
		}
		return out, ci, nil

	default:
		return nil, ci, fmt.Errorf("pionapm: unsupported wave.Audio variant %T", chunk)
	}
}

// fromInt16Interleaved wraps processed samples back into the same
// wave.Audio variant as the input template.
func fromInt16Interleaved(template wave.Audio, samples []int16, src wave.ChunkInfo) wave.Audio {
	outCI := wave.ChunkInfo{
		Len:          len(samples) / src.Channels,
		Channels:     src.Channels,
		SamplingRate: src.SamplingRate,
	}
	switch template.(type) {
	case *wave.Int16Interleaved:
		return &wave.Int16Interleaved{Data: samples, Size: outCI}

	case *wave.Int16NonInterleaved:
		chans := make([][]int16, outCI.Channels)
		for ch := range chans {
			chans[ch] = make([]int16, outCI.Len)
			for i := 0; i < outCI.Len; i++ {
				chans[ch][i] = samples[i*outCI.Channels+ch]
			}
		}
		return &wave.Int16NonInterleaved{Data: chans, Size: outCI}

	case *wave.Float32Interleaved:
		floats := make([]float32, len(samples))
		for i, s := range samples {
			floats[i] = int16ToFloat(s)
		}
		return &wave.Float32Interleaved{Data: floats, Size: outCI}

	case *wave.Float32NonInterleaved:
		chans := make([][]float32, outCI.Channels)
		for ch := range chans {
			chans[ch] = make([]float32, outCI.Len)
			for i := 0; i < outCI.Len; i++ {
				chans[ch][i] = int16ToFloat(samples[i*outCI.Channels+ch])
			}
		}
		return &wave.Float32NonInterleaved{Data: chans, Size: outCI}
	}
	// Unreachable: toInt16Interleaved would have errored on an unknown variant.
	return &wave.Int16Interleaved{Data: samples, Size: outCI}
}

func floatToInt16(f float32) int16 {
	v := f * 32767.0
	if v > 32767.0 {
		return 32767
	}
	if v < -32768.0 {
		return -32768
	}
	return int16(v)
}

func int16ToFloat(s int16) float32 {
	return float32(s) / 32768.0
}

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
	proc      *apm.Processor
	cfg       apm.Config
	frameSize int

	capMu sync.Mutex
	cap   *framer.Framer
	cWork []int16

	revMu sync.Mutex
	rev   *framer.Framer
	rWork []int16
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
// The transform expects upstream chunks to be *wave.Int16Interleaved at
// the SampleRate and Channels declared in cfg. Other wave variants or
// rate / channel mismatches will produce a Read error. Configure the
// mediadevices microphone driver accordingly.
func (b *Bridge) Transform() audio.TransformFunc {
	return func(src audio.Reader) audio.Reader {
		return audio.ReaderFunc(func() (wave.Audio, func(), error) {
			out := make([]int16, 0, b.frameSize*4)
			var ci wave.ChunkInfo
			for {
				chunk, release, err := src.Read()
				if err != nil {
					return nil, nil, err
				}
				iv, ok := chunk.(*wave.Int16Interleaved)
				if !ok {
					if release != nil {
						release()
					}
					return nil, nil, fmt.Errorf("pionapm: unsupported chunk type %T (need *wave.Int16Interleaved)", chunk)
				}
				cc := iv.ChunkInfo()
				if cc.SamplingRate != b.cfg.SampleRate || cc.Channels != b.cfg.Channels {
					if release != nil {
						release()
					}
					return nil, nil, fmt.Errorf("pionapm: chunk %d Hz / %d ch does not match apm.Config %d Hz / %d ch",
						cc.SamplingRate, cc.Channels, b.cfg.SampleRate, b.cfg.Channels)
				}
				ci = cc

				b.capMu.Lock()
				b.cap.Push(iv.Data)
				var perr error
				for b.cap.Pop(b.cWork) {
					if e := b.proc.ProcessStream(b.cWork); e != nil {
						perr = e
						break
					}
					out = append(out, b.cWork...)
				}
				b.capMu.Unlock()

				if release != nil {
					release()
				}
				if perr != nil {
					return nil, nil, perr
				}
				if len(out) > 0 {
					break
				}
				// Buffered but not enough samples for a frame yet; loop and read more.
			}

			return &wave.Int16Interleaved{
				Data: out,
				Size: wave.ChunkInfo{
					Len:          len(out) / ci.Channels,
					Channels:     ci.Channels,
					SamplingRate: ci.SamplingRate,
				},
			}, func() {}, nil
		})
	}
}

// FeedReverse supplies far-end (loudspeaker / playback) audio for AEC.
// Drive it from your playback path in 10 ms or larger chunks of int16
// PCM at the rate / channels declared in cfg. Cadence should match the
// near-end Read rate.
//
// When cfg.EnableAEC is false, FeedReverse is a no-op.
func (b *Bridge) FeedReverse(samples []int16) error {
	if !b.cfg.EnableAEC {
		return nil
	}
	if len(samples) == 0 {
		return nil
	}
	b.revMu.Lock()
	defer b.revMu.Unlock()
	b.rev.Push(samples)
	for b.rev.Pop(b.rWork) {
		if err := b.proc.ProcessReverseStream(b.rWork); err != nil {
			return err
		}
	}
	return nil
}

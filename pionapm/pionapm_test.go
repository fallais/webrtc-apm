package pionapm

import (
	"io"
	"math"
	"testing"

	"github.com/pion/mediadevices/pkg/io/audio"
	"github.com/pion/mediadevices/pkg/wave"

	apm "github.com/fallais/webrtc-apm"
)

// sineReader emits chunks of a 1 kHz sine wave at 48 kHz mono int16.
type sineReader struct {
	chunkLen  int
	remaining int
	phase     float64
	channels  int
	rate      int
	make      func(samples []int16, ci wave.ChunkInfo) wave.Audio
}

func (r *sineReader) Read() (wave.Audio, func(), error) {
	if r.remaining <= 0 {
		return nil, nil, io.EOF
	}
	n := r.chunkLen
	if n > r.remaining {
		n = r.remaining
	}
	samples := make([]int16, n*r.channels)
	for i := 0; i < n; i++ {
		r.phase += 2 * math.Pi * 1000.0 / float64(r.rate)
		v := int16(math.Sin(r.phase) * 16384)
		for ch := 0; ch < r.channels; ch++ {
			samples[i*r.channels+ch] = v
		}
	}
	r.remaining -= n
	ci := wave.ChunkInfo{Len: n, Channels: r.channels, SamplingRate: r.rate}
	return r.make(samples, ci), func() {}, nil
}

func makeInt16Interleaved(samples []int16, ci wave.ChunkInfo) wave.Audio {
	return &wave.Int16Interleaved{Data: samples, Size: ci}
}

func makeInt16NonInterleaved(samples []int16, ci wave.ChunkInfo) wave.Audio {
	chans := make([][]int16, ci.Channels)
	for ch := range chans {
		chans[ch] = make([]int16, ci.Len)
		for i := 0; i < ci.Len; i++ {
			chans[ch][i] = samples[i*ci.Channels+ch]
		}
	}
	return &wave.Int16NonInterleaved{Data: chans, Size: ci}
}

func makeFloat32Interleaved(samples []int16, ci wave.ChunkInfo) wave.Audio {
	floats := make([]float32, len(samples))
	for i, s := range samples {
		floats[i] = float32(s) / 32768.0
	}
	return &wave.Float32Interleaved{Data: floats, Size: ci}
}

func makeFloat32NonInterleaved(samples []int16, ci wave.ChunkInfo) wave.Audio {
	chans := make([][]float32, ci.Channels)
	for ch := range chans {
		chans[ch] = make([]float32, ci.Len)
		for i := 0; i < ci.Len; i++ {
			chans[ch][i] = float32(samples[i*ci.Channels+ch]) / 32768.0
		}
	}
	return &wave.Float32NonInterleaved{Data: chans, Size: ci}
}

func rms(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sumSq float64
	for _, s := range samples {
		v := float64(s)
		sumSq += v * v
	}
	return math.Sqrt(sumSq / float64(len(samples)))
}

func extract(t *testing.T, chunk wave.Audio) []int16 {
	t.Helper()
	samples, _, err := toInt16Interleaved(chunk)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return samples
}

// runTransform feeds 0.5 s of a 1 kHz sine into Bridge.Transform via
// the given factory, drains the output, and returns the total int16
// samples received.
func runTransform(t *testing.T, mk func([]int16, wave.ChunkInfo) wave.Audio) []int16 {
	t.Helper()

	bridge, err := New(apm.Config{
		SampleRate: 48000,
		Channels:   1,
		EnableAEC:  false,
		EnableNS:   false,
		EnableAGC:  true,
		AGCMode:    apm.AGCAdaptiveDigital,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })

	src := &sineReader{
		chunkLen:  4800, // 100 ms chunks at 48 kHz
		remaining: 24000,
		channels:  1,
		rate:      48000,
		make:      mk,
	}
	out := bridge.Transform()(src)

	var got []int16
	for {
		chunk, release, err := out.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		got = append(got, extract(t, chunk)...)
		if release != nil {
			release()
		}
	}
	return got
}

func TestTransform_Int16Interleaved(t *testing.T) {
	got := runTransform(t, makeInt16Interleaved)
	if len(got) == 0 {
		t.Fatal("no samples emitted")
	}
	if r := rms(got); r < 1000 {
		t.Fatalf("output RMS=%.1f, too low — APM may have silenced the input", r)
	}
}

func TestTransform_Int16NonInterleaved(t *testing.T) {
	got := runTransform(t, makeInt16NonInterleaved)
	if len(got) == 0 {
		t.Fatal("no samples emitted")
	}
	if r := rms(got); r < 1000 {
		t.Fatalf("output RMS=%.1f, too low", r)
	}
}

func TestTransform_Float32Interleaved(t *testing.T) {
	got := runTransform(t, makeFloat32Interleaved)
	if len(got) == 0 {
		t.Fatal("no samples emitted")
	}
	if r := rms(got); r < 1000 {
		t.Fatalf("output RMS=%.1f, too low", r)
	}
}

func TestTransform_Float32NonInterleaved(t *testing.T) {
	got := runTransform(t, makeFloat32NonInterleaved)
	if len(got) == 0 {
		t.Fatal("no samples emitted")
	}
	if r := rms(got); r < 1000 {
		t.Fatalf("output RMS=%.1f, too low", r)
	}
}

func TestTransform_RejectsRateMismatch(t *testing.T) {
	bridge, err := New(apm.Config{SampleRate: 48000, Channels: 1, EnableAGC: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer bridge.Close()

	// Upstream at 16 kHz — should be rejected since apm.Config says 48 kHz.
	src := &sineReader{
		chunkLen:  1600,
		remaining: 8000,
		channels:  1,
		rate:      16000,
		make:      makeInt16Interleaved,
	}
	_, _, err = bridge.Transform()(src).Read()
	if err == nil {
		t.Fatal("expected error for rate mismatch, got nil")
	}
}

func TestTransform_RejectsChannelMismatch(t *testing.T) {
	bridge, err := New(apm.Config{SampleRate: 48000, Channels: 1, EnableAGC: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer bridge.Close()

	src := &sineReader{
		chunkLen:  4800,
		remaining: 24000,
		channels:  2,
		rate:      48000,
		make:      makeInt16Interleaved,
	}
	_, _, err = bridge.Transform()(src).Read()
	if err == nil {
		t.Fatal("expected error for channel mismatch, got nil")
	}
}

func TestFeedReverse_AECDisabledIsNoOp(t *testing.T) {
	bridge, err := New(apm.Config{SampleRate: 48000, Channels: 1, EnableAGC: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer bridge.Close()

	// AEC is off — FeedReverse should silently accept anything.
	if err := bridge.FeedReverse(make([]int16, 480)); err != nil {
		t.Fatalf("FeedReverse with AEC off: %v", err)
	}
}

func TestFeedReverse_RejectsChannelMisalignment(t *testing.T) {
	bridge, err := New(apm.Config{
		SampleRate: 48000,
		Channels:   2,
		EnableAEC:  true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer bridge.Close()

	// 481 is not a multiple of 2 → should error.
	if err := bridge.FeedReverse(make([]int16, 481)); err == nil {
		t.Fatal("expected error for samples not aligned to Channels")
	}
}

func TestFeedReverse_HappyPath(t *testing.T) {
	bridge, err := New(apm.Config{
		SampleRate: 48000,
		Channels:   1,
		EnableAEC:  true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer bridge.Close()

	// Feed a few 10 ms frames worth — should accept without error.
	for i := 0; i < 5; i++ {
		if err := bridge.FeedReverse(make([]int16, 480)); err != nil {
			t.Fatalf("FeedReverse #%d: %v", i, err)
		}
	}
}

// Make sure Transform() satisfies audio.TransformFunc — this is the
// integration claim the README rests on.
func TestTransform_SatisfiesAudioTransformFunc(t *testing.T) {
	bridge, err := New(apm.Config{SampleRate: 48000, Channels: 1, EnableAGC: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer bridge.Close()
	var _ audio.TransformFunc = bridge.Transform()
}

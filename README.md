# webrtc-apm

WebRTC-grade audio processing (AEC / NS / AGC) plus optional
[rnnoise](https://github.com/xiph/rnnoise) DNN noise suppression for
Go. Designed to plug into
[pion/mediadevices](https://github.com/pion/mediadevices).

`pion/webrtc` and `pion/mediadevices` together cover the whole stack a
Go WebRTC application needs except the one layer browsers always have:
**audio processing**. Without echo cancellation, noise suppression, and
automatic gain control, real voice calls sound broken. This library
fills that gap.

## Backends

| Layer | Library | Role |
|---|---|---|
| DNN pre-stage (optional) | [`rnnoise`](https://github.com/xiph/rnnoise) | Stationary + non-stationary noise suppression |
| Main pipeline | [`webrtc-audio-processing 2.x`](https://gitlab.freedesktop.org/pulseaudio/webrtc-audio-processing) | AEC3, modern NS, AGC1 |

Both noise suppressors are useful in noisy environments but trade off
voice naturalness for noise reduction — see [Tuning notes](#tuning-notes)
below for when to enable each.

## Install

Both backends are libraries with no stock Ubuntu 24.04 binary package.
Source-build them once with the included Makefile target:

```sh
sudo apt install -y \
    build-essential pkg-config \
    meson ninja-build libabsl-dev \
    autoconf automake libtool \
    git

make deps          # builds both, installs into /usr/local (uses sudo)
sudo ldconfig
```

User-local install (no sudo):

```sh
make deps PREFIX=$HOME/.local
export PKG_CONFIG_PATH=$HOME/.local/lib/pkgconfig:$PKG_CONFIG_PATH
export LD_LIBRARY_PATH=$HOME/.local/lib:$LD_LIBRARY_PATH
```

Then:

```sh
go get github.com/fallais/webrtc-apm
```

## Quickstart

```go
package main

import (
    "log"

    apm "github.com/fallais/webrtc-apm"
)

func main() {
    // Recommended defaults for voice in a typical environment:
    // echo cancellation + automatic gain control, NS off (NS adds frame
    // gating artifacts that sound like "cracks" on clean speech).
    // Flip EnableNS / EnableRNNoise on for genuinely noisy environments
    // (call center, open office, café) — see Tuning notes below.
    p, err := apm.New(apm.Config{
        SampleRate: 48000,
        Channels:   1,
        EnableAEC:  true,
        EnableAGC:  true,
        AGCMode:    apm.AGCAdaptiveDigital,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer p.Close()

    frame := make([]int16, p.SamplesPer10ms())
    // ... fill `frame` with one 10 ms chunk from the microphone ...
    if err := p.ProcessStream(frame); err != nil {
        log.Fatal(err)
    }
    // `frame` is now echo-cancelled and gain-controlled.
}
```

If you enable `EnableRNNoise`, `SampleRate` must be `48000` and
`Channels` must be `1` — rnnoise is fixed at 48 kHz mono. APM itself
supports 8/16/32/48 kHz at mono or stereo.

For arbitrary-sized chunks, wrap a `framer.Framer` to rebuffer into
10 ms frames:

```go
import "github.com/fallais/webrtc-apm/framer"

f := framer.New(p.SamplesPer10ms())
frame := make([]int16, p.SamplesPer10ms())

f.Push(arbitraryChunk)
for f.Pop(frame) {
    if err := p.ProcessStream(frame); err != nil {
        log.Fatal(err)
    }
}
```

`framer` is pure Go and has no system dependencies, so it builds and
tests on any machine even without the native libraries installed.

## With pion/mediadevices

```go
import (
    "log"

    "github.com/pion/mediadevices"
    "github.com/pion/mediadevices/pkg/prop"
    _ "github.com/pion/mediadevices/pkg/driver/microphone"

    apm "github.com/fallais/webrtc-apm"
    "github.com/fallais/webrtc-apm/pionapm"
)

// Capture at 48 kHz mono so APM's config matches the chunk rate / channels.
stream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
    Audio: func(c *mediadevices.MediaTrackConstraints) {
        c.SampleRate   = prop.Int(48000)
        c.ChannelCount = prop.Int(1)
    },
})
if err != nil { log.Fatal(err) }

bridge, err := pionapm.New(apm.Config{
    SampleRate: 48000, Channels: 1,
    EnableAEC: true, EnableAGC: true,
    // NS / rnnoise off — see Tuning notes.
})
if err != nil { log.Fatal(err) }
defer bridge.Close()

track := stream.GetAudioTracks()[0].(*mediadevices.AudioTrack)
track.Transform(bridge.Transform())
```

`bridge.Transform()` returns a
[`audio.TransformFunc`](https://pkg.go.dev/github.com/pion/mediadevices/pkg/io/audio#TransformFunc),
which is the exact contract that `(*mediadevices.AudioTrack).Transform`
accepts. No glue code is needed; the transform sits inside the track's
broadcaster pipeline and runs on every chunk before downstream
consumers (Opus encoder, `TrackLocalStaticSample`, etc.) see it.

### AEC with mediadevices needs a far-end reference

`pion/mediadevices`' transform contract is one-way, so AEC's reverse
stream has no native place to live. When `EnableAEC: true`, you must
also feed the bridge what you're about to play through the
loudspeakers:

```go
// In your playback path:
for chunk := range playbackChunks { // []int16, 48 kHz mono
    _ = bridge.FeedReverse(chunk)
    sendToPlayback(chunk)
}
```

Without `FeedReverse`, AEC has no reference and will not converge — it
won't actively *hurt* anything, but it won't actively help either. If
you don't have a playback path or don't need AEC, leave
`EnableAEC: false` and skip `FeedReverse` entirely.

### Constraints

`bridge.Transform()` requires upstream chunks to be
`*wave.Int16Interleaved` at the same `SampleRate` and `Channels` you
declared in `apm.Config`. Any mismatch produces an error from `Read()`.
Configure your mediadevices `MediaTrackConstraints` accordingly (as in
the snippet above).

## Tuning notes

The four stages — AEC, NS, AGC, rnnoise — are independently toggleable
because the right combination depends on your environment. Empirical
findings from A/B testing on this library:

| Stage | Recommended default | When to flip it |
|---|---|---|
| **AEC** | **on** | Always on if your application captures and plays audio (any voice call, conferencing, voice assistant). Removes speaker→mic echo. Linear — no audible artifacts on clean speech. |
| **AGC** | **on** | Almost always on for voice calls — normalizes your voice level so the remote party hears you consistently. Off if you want raw, un-compressed levels (broadcast, music). |
| **NS** (classical) | **off** | On for noisy environments only (call center, café, open office, audible fan/AC). Off in quiet rooms — its frame-by-frame gating produces audible "cracks" and "musical noise" at speech onsets / offsets, which costs voice naturalness. |
| **rnnoise** (DNN) | **off** | Same logic as NS but more aggressive. The RNN can attenuate borderline speech frames (sibilants, breath) and produces a "watery" / DNN-processed quality. Only when noise removal matters more than naturalness. |

If you do enable NS or rnnoise, prefer them **alone**, not stacked. Two
noise suppressors operating on the same signal compound their gating
artifacts and rarely sound better than either alone.

If you enable rnnoise: `SampleRate` must be 48000 and `Channels` must
be 1 (the library returns an error otherwise).

## What this is not

- **Not a Chrome-quality drop-in.** APM 2.x gets you close on AEC and
  classical processing but Chrome ships newer libwebrtc with additional
  ML noise suppression (model proprietary) and OS-level audio-path
  integration.
- **Not pure Go.** Both backends are C/C++ and reached via cgo.

## License

BSD-3-Clause, matching the underlying libraries.

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
    "github.com/pion/mediadevices"
    _ "github.com/pion/mediadevices/pkg/driver/microphone"

    apm "github.com/fallais/webrtc-apm"
    "github.com/fallais/webrtc-apm/mediadevicesx"
)

transform, err := mediadevicesx.NewTransform(apm.Config{
    SampleRate: 48000, Channels: 1,
    EnableAEC: true, EnableAGC: true,
    // EnableNS / EnableRNNoise off by default — see Tuning notes.
})
// then feed transform.ProcessSamples / ProcessReverseSamples
// from the mediadevices audio track and your playback graph.
```

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

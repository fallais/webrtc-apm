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
| Main pipeline | [`webrtc-audio-processing 1.x`](https://gitlab.freedesktop.org/pulseaudio/webrtc-audio-processing) | AEC3, classical NS, AGC1 |

Stacking rnnoise ahead of APM closes most of the perceptual gap to
Chrome's modern audio stack on typical voice signals.

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
    p, err := apm.New(apm.Config{
        SampleRate:    48000,
        Channels:      1,
        EnableRNNoise: true, // DNN noise suppression before APM
        EnableAEC:     true,
        EnableNS:      true,
        NSLevel:       apm.NSHigh,
        EnableAGC:     true,
        AGCMode:       apm.AGCAdaptiveDigital,
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
    // `frame` is now denoised, echo-cancelled, NS'd and gain-controlled.
}
```

`EnableRNNoise=true` requires `SampleRate=48000` and `Channels=1` —
rnnoise is fixed at 48 kHz mono. APM itself supports 8/16/32/48 kHz at
mono or stereo.

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
    EnableRNNoise: true,
    EnableAEC: true, EnableNS: true, EnableAGC: true,
})
// then feed transform.ProcessSamples / ProcessReverseSamples
// from the mediadevices audio track and your playback graph.
```

## What this is not

- **Not a Chrome-quality drop-in.** rnnoise + APM 1.x gets you very
  close on most voice signals but Chrome ships newer libwebrtc with
  additional ML and OS integration.
- **Not a VAD library.** APM 1.x dropped the standalone VAD submodule.
  Pair with [`webrtcvad-go`](https://github.com/maxhawkins/go-webrtcvad)
  or silero-vad downstream of `ProcessStream` if you need one.
- **Not pure Go.** Both backends are C/C++ and reached via cgo.

## License

BSD-3-Clause, matching the underlying libraries.

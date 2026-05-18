# examples/pion

The canonical webrtc-apm + pion/mediadevices integration example.
~100 lines, every line maps to one step in the README's "With
pion/mediadevices" section.

What it does:

1. Builds a `pionapm.Bridge` configured for 48 kHz mono, AGC on.
2. Calls `mediadevices.GetUserMedia` to open the default microphone
   with matching constraints.
3. Attaches `bridge.Transform()` to the `*mediadevices.AudioTrack`.
4. Drains the post-processing audio via `track.NewReader(false)` and
   writes the int16 PCM samples to `pion-out.wav`.

That's it. No other moving parts.

## Run

```sh
# from project root, after `make deps`
go run ./examples/pion --duration 8s
```

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `--out-dir` | `.` | directory for the output WAV file |
| `--duration` | `5s` | max record duration; Ctrl+C also stops |

Play the result with `aplay pion-out.wav`.

## AEC is not wired here

This example omits echo cancellation because the far-end (loudspeaker /
playback) signal is application-specific — there's no realistic "demo
source" to play. The README's *With pion/mediadevices → With AEC* section
shows the wiring: set `EnableAEC: true` on the `apm.Config` above and
call `bridge.FeedReverse(playbackChunk)` from whatever playback path
your application has (a remote peer's decoded audio in a WebRTC call,
a music player's output, etc.).

## What this demonstrates and what it doesn't

**Demonstrates:**
- The integration is one method call: `track.Transform(bridge.Transform())`.
- The `audio.TransformFunc` returned by `bridge.Transform()` slots into
  `pion/mediadevices`' broadcaster pipeline natively — no glue code.
- Output is the same `wave.Audio` variant as the input; downstream
  consumers (Opus encoder via `track.NewEncodedReader`, WebRTC
  peer-connection bind via `pc.AddTransceiverFromTrack`, etc.) see
  what they expect.

**Doesn't demonstrate:**
- A real WebRTC peer connection. To send the track over the network,
  see `github.com/pion/mediadevices/examples/webrtc` for the
  signaling + `pc.AddTransceiverFromTrack` flow — drop the
  `bridge.Transform()` call into that example to combine the two.
- AEC reverse stream — see top of this README.
- Codec selection. A real app would use `mediadevices.NewCodecSelector`
  with `opus.NewParams()` and pass it through `MediaStreamConstraints`.

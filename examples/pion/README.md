# examples/pion

Minimal example of attaching webrtc-apm to a `pion/mediadevices`
audio track. Every line of `main.go` maps to one step in the
project README's "With pion/mediadevices" section.

What it does:

1. Builds a `pionapm.Bridge` (AGC on; AEC / NS / rnnoise off — the
   recommended default for clean voice).
2. Calls `mediadevices.GetUserMedia` to open the default microphone.
3. Attaches `bridge.Transform()` to the resulting `*mediadevices.AudioTrack`.

That's the integration. The example then sleeps for 5 seconds so the
track is alive long enough to verify it doesn't crash; a real
application would instead bind the track to a `*webrtc.PeerConnection`,
encode it with Opus, or otherwise consume it downstream.

## Run

```sh
# from project root, after `make deps`
go run ./examples/pion
```

For the AEC wiring (`bridge.FeedReverse` driven by a playback path),
see [`examples/pion-aec`](../pion-aec).

# examples/pion-aec

Minimal example showing AEC wiring on top of the basic
[`examples/pion`](../pion) integration. AEC needs the far-end
(loudspeaker / playback) reference signal, which lives outside the
`pion/mediadevices` `TransformFunc` contract — so callers pump it
through `pionapm.Bridge.FeedReverse` from whatever playback path
their application has.

This example shows the seam, not a real playback implementation.
The `renderToSpeakers` and `playbackSource` functions in `main.go`
are stubs; in a real WebRTC application:

- `playbackSource` would be audio decoded from a remote peer's
  `*webrtc.TrackRemote`.
- `renderToSpeakers` would be your audio output library
  (malgo, oto, alsa, ...).

The two lines that matter are in the goroutine:

```go
if err := bridge.FeedReverse(chunk); err != nil { ... }
renderToSpeakers(chunk)
```

For each chunk you render to the loudspeakers, feed the same chunk
to `bridge.FeedReverse` first.

## Run

```sh
# from project root, after `make deps`
go run ./examples/pion-aec
```

With the stubs as written, the example exercises construction and
shutdown but doesn't actually exercise AEC (no audio plays). Plug in
real playback to see AEC working.

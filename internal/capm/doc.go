// Package capm is the cgo bridge to the WebRTC audio_processing module.
//
// It is not safe for direct use by application code: the public API of
// webrtc-apm lives in the root package and treats this package as an
// implementation detail.
//
// The bridge targets the webrtc-audio-processing 1.x series (the
// freedesktop / PipeWire fork). On Linux it expects the
// webrtc-audio-processing-1 pkg-config module; see the project Makefile
// for a `deps` target that source-builds it.
package capm

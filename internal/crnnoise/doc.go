// Package crnnoise is the cgo bridge to xiph/rnnoise, the recurrent
// neural network noise suppressor.
//
// rnnoise operates at a fixed sample rate (48 kHz, mono) and a fixed
// frame size (480 samples = 10 ms). The root webrtc-apm package gates
// its use accordingly: callers must configure SampleRate=48000 and
// Channels=1 when EnableRNNoise is true.
//
// The bridge is not safe for direct use by application code.
package crnnoise

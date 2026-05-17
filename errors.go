package apm

import "errors"

var (
	ErrFrameSize         = errors.New("apm: frame size does not match SamplesPer10ms")
	ErrSampleRate        = errors.New("apm: sample rate must be 8000, 16000, 32000 or 48000")
	ErrChannels          = errors.New("apm: channels must be 1 or 2")
	ErrRNNoiseSampleRate = errors.New("apm: EnableRNNoise requires SampleRate=48000")
	ErrRNNoiseChannels   = errors.New("apm: EnableRNNoise requires Channels=1")
)

func (c Config) validate() error {
	switch c.SampleRate {
	case 8000, 16000, 32000, 48000:
	default:
		return ErrSampleRate
	}
	if c.Channels != 1 && c.Channels != 2 {
		return ErrChannels
	}
	if c.EnableRNNoise {
		if c.SampleRate != 48000 {
			return ErrRNNoiseSampleRate
		}
		if c.Channels != 1 {
			return ErrRNNoiseChannels
		}
	}
	return nil
}

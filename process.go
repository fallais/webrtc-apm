package apm

// ProcessStream applies the configured pipeline to one 10 ms near-end
// frame (microphone input). The frame is modified in place and its
// length must equal SamplesPer10ms().
//
// When rnnoise is enabled it runs first, producing a denoised int16
// frame that is then fed through APM (AEC / further NS / AGC). When
// rnnoise is disabled the frame goes straight to APM.
func (p *Processor) ProcessStream(frame []int16) error {
	if len(frame) != p.samplesPer10ms {
		return ErrFrameSize
	}
	if p.rnn != nil {
		for i, s := range frame {
			p.rnnIn[i] = float32(s)
		}
		if _, err := p.rnn.ProcessFrame(p.rnnOut, p.rnnIn); err != nil {
			return err
		}
		for i, f := range p.rnnOut {
			switch {
			case f > 32767.0:
				f = 32767.0
			case f < -32768.0:
				f = -32768.0
			}
			frame[i] = int16(f)
		}
	}
	return p.apm.ProcessStream(frame)
}

// ProcessReverseStream supplies the far-end (loudspeaker / playback)
// reference frame used by AEC. rnnoise does not see this signal — it
// is fed directly to APM. The frame length must equal SamplesPer10ms()
// and frames must arrive at the same cadence as ProcessStream.
func (p *Processor) ProcessReverseStream(frame []int16) error {
	if len(frame) != p.samplesPer10ms {
		return ErrFrameSize
	}
	return p.apm.ProcessReverseStream(frame)
}

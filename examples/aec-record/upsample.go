package main

// upsample resamples an int16 mono PCM stream from inRate to outRate
// using linear interpolation. Adequate for an AEC reference signal
// (where high-frequency fidelity does not matter); not a quality
// resampler for general audio.
func upsample(in []int16, inRate, outRate uint32) []int16 {
	if inRate == outRate || len(in) == 0 {
		out := make([]int16, len(in))
		copy(out, in)
		return out
	}
	ratio := float64(outRate) / float64(inRate)
	outLen := int(float64(len(in)) * ratio)
	out := make([]int16, outLen)
	maxSrc := len(in) - 1
	for i := 0; i < outLen; i++ {
		srcPos := float64(i) / ratio
		srcIdx := int(srcPos)
		if srcIdx >= maxSrc {
			out[i] = in[maxSrc]
			continue
		}
		frac := srcPos - float64(srcIdx)
		a := float64(in[srcIdx])
		b := float64(in[srcIdx+1])
		out[i] = int16(a + (b-a)*frac)
	}
	return out
}

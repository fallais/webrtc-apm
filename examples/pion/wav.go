package main

import (
	"encoding/binary"
	"os"
)

// wavWriter streams 16-bit PCM to a WAV file. The header is written up
// front with a zero data size and rewritten with the final size on Close.
type wavWriter struct {
	f          *os.File
	sampleRate uint32
	channels   uint16
	samples    int64
}

func newWAVWriter(path string, sampleRate uint32, channels uint16) (*wavWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := &wavWriter{f: f, sampleRate: sampleRate, channels: channels}
	if _, err := f.Write(w.headerBytes(0)); err != nil {
		_ = f.Close()
		return nil, err
	}
	return w, nil
}

func (w *wavWriter) Write(samples []int16) error {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:i*2+2], uint16(s))
	}
	if _, err := w.f.Write(buf); err != nil {
		return err
	}
	w.samples += int64(len(samples))
	return nil
}

func (w *wavWriter) Close() error {
	if w.f == nil {
		return nil
	}
	dataSize := uint32(w.samples) * uint32(w.channels) * 2
	_, werr := w.f.WriteAt(w.headerBytes(dataSize), 0)
	cerr := w.f.Close()
	w.f = nil
	if werr != nil {
		return werr
	}
	return cerr
}

func (w *wavWriter) headerBytes(dataSize uint32) []byte {
	const bitsPerSample uint16 = 16
	byteRate := w.sampleRate * uint32(w.channels) * uint32(bitsPerSample) / 8
	blockAlign := w.channels * bitsPerSample / 8

	buf := make([]byte, 44)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], 36+dataSize)
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], w.channels)
	binary.LittleEndian.PutUint32(buf[24:28], w.sampleRate)
	binary.LittleEndian.PutUint32(buf[28:32], byteRate)
	binary.LittleEndian.PutUint16(buf[32:34], blockAlign)
	binary.LittleEndian.PutUint16(buf[34:36], bitsPerSample)
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], dataSize)
	return buf
}

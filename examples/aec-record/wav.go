package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// wavWriter — streamed 16-bit PCM. (Same as examples/record/wav.go; kept
// here so each example stays self-contained and copy-pasteable.)
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

// readWAV reads a 16-bit PCM mono WAV file and returns its samples,
// sample rate and channel count. Non-PCM and non-16-bit files are
// rejected. Unknown chunks before "data" are skipped.
func readWAV(path string) (samples []int16, sampleRate uint32, channels uint16, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()

	var riff [12]byte
	if _, err := io.ReadFull(f, riff[:]); err != nil {
		return nil, 0, 0, fmt.Errorf("read riff header: %w", err)
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return nil, 0, 0, errors.New("not a RIFF/WAVE file")
	}

	var bitsPerSample uint16
	var dataSize uint32

	for {
		var ch [8]byte
		if _, rerr := io.ReadFull(f, ch[:]); rerr != nil {
			return nil, 0, 0, fmt.Errorf("read chunk header: %w", rerr)
		}
		id := string(ch[0:4])
		size := binary.LittleEndian.Uint32(ch[4:8])
		switch id {
		case "fmt ":
			fmtBuf := make([]byte, size)
			if _, rerr := io.ReadFull(f, fmtBuf); rerr != nil {
				return nil, 0, 0, fmt.Errorf("read fmt chunk: %w", rerr)
			}
			if len(fmtBuf) < 16 {
				return nil, 0, 0, errors.New("fmt chunk too small")
			}
			format := binary.LittleEndian.Uint16(fmtBuf[0:2])
			if format != 1 {
				return nil, 0, 0, fmt.Errorf("only PCM (1) supported, got format %d", format)
			}
			channels = binary.LittleEndian.Uint16(fmtBuf[2:4])
			sampleRate = binary.LittleEndian.Uint32(fmtBuf[4:8])
			bitsPerSample = binary.LittleEndian.Uint16(fmtBuf[14:16])
			if bitsPerSample != 16 {
				return nil, 0, 0, fmt.Errorf("only 16-bit PCM supported, got %d", bitsPerSample)
			}
		case "data":
			dataSize = size
			data := make([]byte, dataSize)
			if _, rerr := io.ReadFull(f, data); rerr != nil {
				return nil, 0, 0, fmt.Errorf("read data chunk: %w", rerr)
			}
			n := int(dataSize) / 2
			samples = make([]int16, n)
			for i := 0; i < n; i++ {
				samples[i] = int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2]))
			}
			return samples, sampleRate, channels, nil
		default:
			// Skip unknown chunk (LIST, INFO, etc.).
			if _, serr := f.Seek(int64(size), io.SeekCurrent); serr != nil {
				return nil, 0, 0, fmt.Errorf("seek past %q chunk: %w", id, serr)
			}
		}
	}
}

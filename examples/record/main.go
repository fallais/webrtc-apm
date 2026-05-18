// Command record opens the default microphone with malgo, runs every
// 10 ms frame through webrtc-apm, and writes two WAV files: the raw mic
// input (record-in.wav) and the processed output (record-out.wav).
//
// This is the same pattern an application would use to plug webrtc-apm
// into an existing malgo-based audio path. For a pion/mediadevices
// integration, see the pionapm package instead.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/gen2brain/malgo"

	apm "github.com/fallais/webrtc-apm"
	"github.com/fallais/webrtc-apm/framer"
)

func main() {
	var (
		outDir    = flag.String("out-dir", ".", "directory to write WAV files into")
		duration  = flag.Duration("duration", 5*time.Second, "max record duration (Ctrl+C also stops)")
		// Defaults match the library's recommended config: AEC + AGC on,
		// NS / rnnoise off (they introduce gating artifacts on clean
		// speech — see README Tuning notes). Flip them on for noisy
		// environments.
		enableRNN = flag.Bool("rnn", false, "enable rnnoise DNN pre-stage")
		enableAEC = flag.Bool("aec", true, "enable echo cancellation")
		enableNS  = flag.Bool("ns", false, "enable noise suppression")
		nsLevelS  = flag.String("ns-level", "moderate", "NS level: low|moderate|high|very-high")
		enableAGC = flag.Bool("agc", true, "enable automatic gain control")
	)
	flag.Parse()

	nsLevel, err := parseNSLevel(*nsLevelS)
	if err != nil {
		log.Fatal(err)
	}

	proc, err := apm.New(apm.Config{
		SampleRate:    48000,
		Channels:      1,
		EnableRNNoise: *enableRNN,
		EnableAEC:     *enableAEC,
		EnableNS:      *enableNS,
		NSLevel:       nsLevel,
		EnableAGC:     *enableAGC,
		AGCMode:       apm.AGCAdaptiveDigital,
	})
	if err != nil {
		log.Fatalf("apm.New: %v", err)
	}
	defer proc.Close()

	inPath := filepath.Join(*outDir, "record-in.wav")
	outPath := filepath.Join(*outDir, "record-out.wav")
	inWav, err := newWAVWriter(inPath, 48000, 1)
	if err != nil {
		log.Fatalf("open %s: %v", inPath, err)
	}
	defer inWav.Close()
	outWav, err := newWAVWriter(outPath, 48000, 1)
	if err != nil {
		log.Fatalf("open %s: %v", outPath, err)
	}
	defer outWav.Close()

	fr := framer.New(proc.SamplesPer10ms())
	frame := make([]int16, proc.SamplesPer10ms())

	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		log.Fatalf("malgo InitContext: %v", err)
	}
	defer func() {
		_ = mctx.Uninit()
		mctx.Free()
	}()

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = 48000
	deviceConfig.Alsa.NoMMap = 1

	samples := make(chan []int16, 64)
	onRecv := func(_, pSample []byte, frameCount uint32) {
		if frameCount == 0 || len(pSample) == 0 {
			return
		}
		n := int(frameCount)
		src := unsafe.Slice((*int16)(unsafe.Pointer(&pSample[0])), n)
		cp := make([]int16, n)
		copy(cp, src)
		select {
		case samples <- cp:
		default:
			// drop on backpressure — main loop fell behind real-time
		}
	}

	device, err := malgo.InitDevice(mctx.Context, deviceConfig, malgo.DeviceCallbacks{Data: onRecv})
	if err != nil {
		log.Fatalf("malgo InitDevice: %v", err)
	}
	defer device.Uninit()

	if err := device.Start(); err != nil {
		log.Fatalf("malgo Start: %v", err)
	}

	fmt.Printf("recording 48kHz mono\n")
	fmt.Printf("  rnn=%v aec=%v ns=%v(%s) agc=%v\n",
		*enableRNN, *enableAEC, *enableNS, *nsLevelS, *enableAGC)
	fmt.Printf("  duration=%s (Ctrl+C to stop early)\n\n", *duration)

	rootCtx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var captured int64
	start := time.Now()
loop:
	for {
		select {
		case <-rootCtx.Done():
			break loop
		case chunk := <-samples:
			captured += int64(len(chunk))
			if err := inWav.Write(chunk); err != nil {
				log.Fatalf("inWav.Write: %v", err)
			}
			fr.Push(chunk)
			for fr.Pop(frame) {
				if err := proc.ProcessStream(frame); err != nil {
					log.Fatalf("ProcessStream: %v", err)
				}
				if err := outWav.Write(frame); err != nil {
					log.Fatalf("outWav.Write: %v", err)
				}
			}
		case <-ticker.C:
			fmt.Printf("\r  %.1fs captured", float64(captured)/48000.0)
		}
	}
	fmt.Println()
	_ = device.Stop()

	fmt.Printf("\nrecorded %.2fs (%d samples) in %s\n",
		float64(captured)/48000.0, captured, time.Since(start).Round(time.Millisecond))
	fmt.Printf("  raw : %s\n", inPath)
	fmt.Printf("  proc: %s\n", outPath)
}

func parseNSLevel(s string) (apm.NSLevel, error) {
	switch s {
	case "low":
		return apm.NSLow, nil
	case "moderate":
		return apm.NSModerate, nil
	case "high":
		return apm.NSHigh, nil
	case "very-high":
		return apm.NSVeryHigh, nil
	}
	return 0, fmt.Errorf("invalid ns-level %q (use low|moderate|high|very-high)", s)
}

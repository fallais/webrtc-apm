// Command aec-record exercises AEC by playing a known far-end speech
// sample through the loudspeakers while simultaneously capturing the
// microphone. The played audio is also fed to ProcessReverseStream so
// AEC has a perfect reference signal.
//
// Three files are produced in --out-dir:
//
//   farend.wav      - the reference signal that was played (48 kHz mono)
//   record-in.wav   - what the microphone actually heard (your voice + echo)
//   record-out.wav  - APM output, ideally only your voice with the echo removed
//
// You can speak during the playback, stay silent, or alternate — listen
// to record-in.wav vs record-out.wav to hear AEC at work.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
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

// Open Speech Repository, "Harvard Sentence List" read by an American
// male voice. ~30 s, 8 kHz mono int16 PCM. Free for testing use.
const defaultFarEndURL = "https://www.voiptroubleshooter.com/open_speech/american/OSR_us_000_0010_8k.wav"

func main() {
	var (
		outDir    = flag.String("out-dir", ".", "directory to write WAV files into")
		farEndArg = flag.String("far-end", "", "path to a far-end WAV (empty = auto-download)")
		cacheDir  = flag.String("cache-dir", "examples/aec-record/testdata", "where to cache the auto-downloaded sample")
		duration  = flag.Duration("duration", 0, "max record duration (0 = play far-end through to the end)")
		enableAEC = flag.Bool("aec", true, "enable echo cancellation")
		enableNS  = flag.Bool("ns", false, "enable noise suppression (off by default to isolate AEC)")
		nsLevelS  = flag.String("ns-level", "high", "NS level: low|moderate|high|very-high")
		enableAGC = flag.Bool("agc", false, "enable AGC (off by default to isolate AEC)")
		enableRNN = flag.Bool("rnn", false, "enable rnnoise (off by default to isolate AEC)")
	)
	flag.Parse()

	nsLevel, err := parseNSLevel(*nsLevelS)
	if err != nil {
		log.Fatal(err)
	}

	// Resolve / fetch the far-end WAV.
	farEndPath := *farEndArg
	if farEndPath == "" {
		var err error
		farEndPath, err = ensureCached(*cacheDir, defaultFarEndURL)
		if err != nil {
			log.Fatalf("fetching default far-end sample: %v", err)
		}
	}

	fmt.Printf("far-end source: %s\n", farEndPath)
	farRaw, farRate, farChannels, err := readWAV(farEndPath)
	if err != nil {
		log.Fatalf("read far-end WAV: %v", err)
	}
	if farChannels != 1 {
		log.Fatalf("far-end WAV must be mono, got %d channels", farChannels)
	}
	farEnd48 := upsample(farRaw, farRate, 48000)
	fmt.Printf("far-end loaded: %d samples @ %d Hz → %d samples @ 48 kHz (%.2f s)\n",
		len(farRaw), farRate, len(farEnd48), float64(len(farEnd48))/48000.0)

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

	// Persist what we actually played, what the mic heard, and what APM produced.
	farPath := filepath.Join(*outDir, "farend.wav")
	inPath := filepath.Join(*outDir, "record-in.wav")
	outPath := filepath.Join(*outDir, "record-out.wav")

	farWav, err := newWAVWriter(farPath, 48000, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer farWav.Close()
	inWav, err := newWAVWriter(inPath, 48000, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer inWav.Close()
	outWav, err := newWAVWriter(outPath, 48000, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer outWav.Close()

	// malgo init
	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		log.Fatalf("malgo InitContext: %v", err)
	}
	defer func() {
		_ = mctx.Uninit()
		mctx.Free()
	}()

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Duplex)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = 1
	deviceConfig.SampleRate = 48000
	deviceConfig.Alsa.NoMMap = 1

	type pair struct {
		rev []int16 // far-end about to play (= what AEC should cancel)
		cap []int16 // mic capture
	}
	pairChan := make(chan pair, 128)
	done := make(chan struct{})
	var farPos int

	onData := func(pOut, pIn []byte, frameCount uint32) {
		n := int(frameCount)
		if n == 0 {
			return
		}
		outSamples := unsafe.Slice((*int16)(unsafe.Pointer(&pOut[0])), n)
		inSamples := unsafe.Slice((*int16)(unsafe.Pointer(&pIn[0])), n)

		// Pull the next n samples from far-end (zeros if exhausted).
		rev := make([]int16, n)
		remaining := len(farEnd48) - farPos
		if remaining > 0 {
			take := n
			if take > remaining {
				take = remaining
			}
			copy(rev, farEnd48[farPos:farPos+take])
			farPos += take
		}
		copy(outSamples, rev)

		cap := make([]int16, n)
		copy(cap, inSamples)

		select {
		case pairChan <- pair{rev: rev, cap: cap}:
		default:
			// drop on backpressure
		}

		if farPos >= len(farEnd48) {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	}

	device, err := malgo.InitDevice(mctx.Context, deviceConfig, malgo.DeviceCallbacks{Data: onData})
	if err != nil {
		log.Fatalf("malgo InitDevice: %v", err)
	}
	defer device.Uninit()

	if err := device.Start(); err != nil {
		log.Fatalf("malgo Start: %v", err)
	}

	fmt.Printf("\nplaying far-end + recording at 48 kHz mono\n")
	fmt.Printf("  aec=%v ns=%v agc=%v rnn=%v\n", *enableAEC, *enableNS, *enableAGC, *enableRNN)
	fmt.Printf("  speak or stay silent during playback. listen to record-in.wav vs record-out.wav afterwards.\n\n")

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if *duration > 0 {
		rootCtx, cancel = context.WithTimeout(context.Background(), *duration)
		defer cancel()
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sig:
		case <-done:
		}
		cancel()
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	revFramer := framer.New(proc.SamplesPer10ms())
	capFramer := framer.New(proc.SamplesPer10ms())
	revFrame := make([]int16, proc.SamplesPer10ms())
	capFrame := make([]int16, proc.SamplesPer10ms())

	var captured int64
	start := time.Now()

loop:
	for {
		select {
		case <-rootCtx.Done():
			break loop
		case p := <-pairChan:
			captured += int64(len(p.cap))
			_ = farWav.Write(p.rev)
			_ = inWav.Write(p.cap)

			revFramer.Push(p.rev)
			capFramer.Push(p.cap)

			for revFramer.Buffered() >= proc.SamplesPer10ms() &&
				capFramer.Buffered() >= proc.SamplesPer10ms() {
				revFramer.Pop(revFrame)
				capFramer.Pop(capFrame)
				if err := proc.ProcessReverseStream(revFrame); err != nil {
					log.Fatalf("ProcessReverseStream: %v", err)
				}
				if err := proc.ProcessStream(capFrame); err != nil {
					log.Fatalf("ProcessStream: %v", err)
				}
				_ = outWav.Write(capFrame)
			}
		case <-ticker.C:
			fmt.Printf("\r  %.1fs captured", float64(captured)/48000.0)
		}
	}
	fmt.Println()
	_ = device.Stop()

	fmt.Printf("\nrecorded %.2fs (%d samples) in %s\n",
		float64(captured)/48000.0, captured, time.Since(start).Round(time.Millisecond))
	fmt.Printf("  far : %s\n", farPath)
	fmt.Printf("  in  : %s\n", inPath)
	fmt.Printf("  out : %s\n", outPath)
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

// ensureCached returns a local path to the file at url, downloading once.
func ensureCached(dir, url string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "farend.wav")
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path, nil
	}
	fmt.Printf("downloading %s\n  → %s\n", url, path)
	req, err := http.NewRequest(http.MethodGet, url, nil) //nolint:gosec // URL is a hardcoded constant.
	if err != nil {
		return "", err
	}
	// Some upstream hosts reject the default Go-http-client UA with 406.
	req.Header.Set("User-Agent", "webrtc-apm/0 (+https://github.com/fallais/webrtc-apm)")
	req.Header.Set("Accept", "*/*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return path, nil
}

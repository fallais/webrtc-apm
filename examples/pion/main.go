// Command pion is the canonical webrtc-apm + pion/mediadevices integration
// example. It captures from the default microphone via mediadevices'
// GetUserMedia API, attaches the webrtc-apm transform via pionapm.Bridge,
// drains the post-processing audio track, and writes the result to a
// WAV file for verification.
//
// The point of this program is not to be a useful application — it's to
// be the shortest, most direct demonstration that the integration works:
// every line maps to one step in the README's "With pion/mediadevices"
// section. Copy these ~100 lines into your own project as a starting
// point.
//
// AEC is intentionally not enabled here because the far-end (playback)
// signal is application-specific and would obscure the integration
// pattern. See pionapm.Bridge.FeedReverse in the godoc / README for the
// AEC wiring.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"

	// Register the microphone driver. Without this anonymous import,
	// GetUserMedia returns "no audio source".
	_ "github.com/pion/mediadevices/pkg/driver/microphone"

	apm "github.com/fallais/webrtc-apm"
	"github.com/fallais/webrtc-apm/pionapm"
)

func main() {
	outDir := flag.String("out-dir", ".", "directory to write the WAV file into")
	duration := flag.Duration("duration", 5*time.Second, "max record duration (Ctrl+C also stops)")
	flag.Parse()

	// 1. Build the APM bridge. Match SampleRate / Channels here to the
	//    MediaTrackConstraints below so pionapm.Bridge doesn't reject the
	//    incoming chunks. AGC on; AEC / NS / rnnoise off (see Tuning
	//    notes in README for when to enable them).
	bridge, err := pionapm.New(apm.Config{
		SampleRate: 48000,
		Channels:   1,
		EnableAGC:  true,
		AGCMode:    apm.AGCAdaptiveDigital,
	})
	if err != nil {
		log.Fatalf("pionapm.New: %v", err)
	}
	defer bridge.Close()

	// 2. Capture from the default microphone via pion/mediadevices.
	stream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Audio: func(c *mediadevices.MediaTrackConstraints) {
			c.SampleRate = prop.Int(48000)
			c.ChannelCount = prop.Int(1)
		},
	})
	if err != nil {
		log.Fatalf("GetUserMedia: %v", err)
	}
	tracks := stream.GetAudioTracks()
	if len(tracks) == 0 {
		log.Fatal("no audio track available")
	}
	audioTrack := tracks[0].(*mediadevices.AudioTrack)
	defer audioTrack.Close()

	// 3. Attach the APM transform. This is the integration point — a
	//    single call. Every chunk produced by the mic now flows through
	//    webrtc-apm before any downstream consumer sees it.
	audioTrack.Transform(bridge.Transform())

	// 4. Drain the processed audio and write it to a WAV.
	reader := audioTrack.NewReader(false)
	outPath := filepath.Join(*outDir, "pion-out.wav")
	out, err := newWAVWriter(outPath, 48000, 1)
	if err != nil {
		log.Fatalf("open %s: %v", outPath, err)
	}
	defer out.Close()

	fmt.Printf("capture → APM → %s\n", outPath)
	fmt.Printf("duration=%s (Ctrl+C to stop early)\n\n", *duration)

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	var captured int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			chunk, release, err := reader.Read()
			if err != nil {
				if err != io.EOF {
					log.Printf("Read: %v", err)
				}
				return
			}
			iv, ok := chunk.(*wave.Int16Interleaved)
			if !ok {
				log.Printf("unexpected chunk type %T", chunk)
				if release != nil {
					release()
				}
				continue
			}
			if err := out.Write(iv.Data); err != nil {
				log.Printf("Write: %v", err)
				if release != nil {
					release()
				}
				return
			}
			captured += int64(len(iv.Data))
			if release != nil {
				release()
			}
		}
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-ticker.C:
			fmt.Printf("\r  %.1fs captured", float64(captured)/48000.0)
		}
	}
	fmt.Println()

	// Closing the track stops the source, which makes the reader return
	// io.EOF and lets the drain goroutine exit cleanly.
	_ = audioTrack.Close()
	<-done

	fmt.Printf("\nrecorded %.2fs (%d samples) → %s\n",
		float64(captured)/48000.0, captured, outPath)
}

// Command pion-aec is a minimal example of the AEC wiring on top of
// the basic examples/pion integration. AEC needs a far-end (playback)
// reference signal, which lives outside the pion/mediadevices
// TransformFunc contract — so callers pump it through
// pionapm.Bridge.FeedReverse from whatever playback path their
// application has.
//
// This example shows the seam, not a real playback implementation.
// renderToSpeakers and playbackSource are stubs; replace them with
// your real audio output (malgo / oto / alsa) and your real far-end
// source (a remote peer's decoded audio, a music player, etc.).
package main

import (
	"log"
	"time"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/prop"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"

	apm "github.com/fallais/webrtc-apm"
	"github.com/fallais/webrtc-apm/pionapm"
)

func main() {
	// 1. Build the APM bridge with AEC enabled.
	bridge, err := pionapm.New(apm.Config{
		SampleRate: 48000,
		Channels:   1,
		EnableAEC:  true,
		EnableAGC:  true,
		AGCMode:    apm.AGCAdaptiveDigital,
	})
	if err != nil {
		log.Fatalf("pionapm.New: %v", err)
	}
	defer bridge.Close()

	// 2. Capture via pion/mediadevices and attach the transform —
	//    identical to the basic examples/pion demo.
	stream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Audio: func(c *mediadevices.MediaTrackConstraints) {
			c.SampleRate = prop.Int(48000)
			c.ChannelCount = prop.Int(1)
		},
	})
	if err != nil {
		log.Fatalf("GetUserMedia: %v", err)
	}

	track := stream.GetAudioTracks()[0].(*mediadevices.AudioTrack)
	defer track.Close()
	track.Transform(bridge.Transform())

	// 3. Drive the AEC reverse stream from your playback path.
	//    For each chunk you're about to render to the loudspeakers,
	//    feed the same chunk to bridge.FeedReverse first. Both calls
	//    use the same int16 PCM at apm.Config.SampleRate / Channels.
	go func() {
		for chunk := range playbackSource() {
			if err := bridge.FeedReverse(chunk); err != nil {
				log.Printf("FeedReverse: %v", err)
			}
			renderToSpeakers(chunk)
		}
	}()

	log.Println("AEC-enabled transform attached; running for 5 seconds")
	time.Sleep(5 * time.Second)
}

// playbackSource stands in for whatever source produces the audio
// you're about to render — in a WebRTC call, audio decoded from a
// remote peer's *webrtc.TrackRemote. This stub returns no chunks;
// replace with your real source.
func playbackSource() <-chan []int16 {
	ch := make(chan []int16)
	close(ch)
	return ch
}

// renderToSpeakers stands in for your audio output path. Replace
// with malgo, oto, or whatever playback library your application
// uses. The slice is 48 kHz mono int16.
func renderToSpeakers(chunk []int16) {
	_ = chunk
}

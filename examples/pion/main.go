// Command pion is a minimal example showing the webrtc-apm integration
// with pion/mediadevices. Every line maps to one step in the project
// README's "With pion/mediadevices" section. Nothing else.
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
	// 1. Build the APM bridge. AGC on; AEC / NS / rnnoise off (see
	//    README Tuning notes for when to enable them).
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

	track := stream.GetAudioTracks()[0].(*mediadevices.AudioTrack)
	defer track.Close()

	// 3. Attach the APM transform. This is the integration point.
	track.Transform(bridge.Transform())

	// The track now passes through webrtc-apm before anything
	// downstream sees it. Real applications would now:
	//   - bind the track to a *webrtc.PeerConnection via
	//     pc.AddTransceiverFromTrack(track, ...)
	//   - or encode it with Opus via
	//     track.NewEncodedReader("audio/opus")
	//   - or read raw processed samples via track.NewReader(false)
	// See https://github.com/pion/mediadevices/tree/master/examples
	// for those patterns.
	log.Println("webrtc-apm transform attached; running for 5 seconds")
	time.Sleep(5 * time.Second)
}

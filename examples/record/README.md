# examples/record

Capture + APM demo. Opens the default microphone via malgo (miniaudio),
runs every 10 ms frame through `apm.Processor`, and writes two WAV
files for A/B listening:

- `record-in.wav`  — raw microphone input
- `record-out.wav` — after rnnoise + APM (AEC / NS / AGC)

Both files are 48 kHz mono 16-bit PCM.

## Run

From the project root, after `make deps`:

```sh
go run ./examples/record --duration 8s
```

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `--duration` | `5s` | max record time; Ctrl+C also stops |
| `--out-dir` | `.` | directory for the two WAV files |
| `--rnn` | `true` | toggle rnnoise pre-stage |
| `--aec` | `true` | toggle echo cancellation |
| `--ns` | `true` | toggle noise suppression |
| `--ns-level` | `high` | `low\|moderate\|high\|very-high` |
| `--agc` | `true` | toggle automatic gain control |

## Why malgo here

`webrtc-apm` itself is capture-agnostic — it takes raw `[]int16` frames
and returns processed ones. This demo uses
[`gen2brain/malgo`](https://github.com/gen2brain/malgo) (a Go binding to
miniaudio) because that's what most standalone Go audio applications
use, and it's also the backend `pion/mediadevices` uses internally.

For a `pion/mediadevices` application the more idiomatic path is to
attach `webrtc-apm/mediadevicesx` as an audio transform on the
`mediadevices` `AudioTrack` — see the top-level README for that snippet.
This demo deliberately stays one layer lower so you can see the data
flow plainly.

## A/B experiments worth running

```sh
# rnnoise off vs on — isolates the DNN pre-stage
go run ./examples/record --rnn=false --out-dir=/tmp/no-rnn  --duration 5s
go run ./examples/record --rnn=true  --out-dir=/tmp/with-rnn --duration 5s

# NS aggressiveness
go run ./examples/record --rnn=false --ns-level=low       --duration 5s
go run ./examples/record --rnn=false --ns-level=very-high --duration 5s

# Everything off — should be near-identical to raw mic input
go run ./examples/record --rnn=false --aec=false --ns=false --agc=false --duration 5s
```

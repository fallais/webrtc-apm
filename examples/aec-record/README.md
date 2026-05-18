# examples/aec-record

The proper AEC test: plays a known far-end speech sample through your
loudspeakers, captures from the microphone (which now picks up your
voice **plus** the leaked echo of the playback), and feeds the played
audio to APM's `ProcessReverseStream` as the AEC reference signal.

After the recording you have three WAV files in the output directory:

- `farend.wav`     — the reference that was played through the speakers.
- `record-in.wav`  — what the mic actually heard (your voice + echo).
- `record-out.wav` — APM output: ideally your voice only.

Listen to `record-in.wav` vs `record-out.wav`. The played speech should
be largely absent from the output; your own voice should be preserved.

## Run

From the project root (after `make deps`):

```sh
go run ./examples/aec-record --duration 8s
```

On first run the demo downloads a ~540 KB English speech sample from
the [Open Speech Repository](https://www.voiptroubleshooter.com/open_speech/)
to `examples/aec-record/testdata/farend.wav` and reuses it from then on.

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `--out-dir` | `.` | directory for the three WAV files |
| `--duration` | `0` | max record time; 0 = play far-end through to the end |
| `--far-end` | (empty) | use this WAV instead of the auto-downloaded one |
| `--cache-dir` | `examples/aec-record/testdata` | where to cache the auto-downloaded sample |
| `--aec` | `true` | enable echo cancellation |
| `--ns` | `false` | enable noise suppression (off here to isolate AEC) |
| `--agc` | `false` | enable AGC (off here to isolate AEC) |
| `--rnn` | `false` | enable rnnoise (off here to isolate AEC) |

## How to interpret what you hear

The most informative thing to do during the run is **alternate**:

1. Stay silent for the first 2–3 s of playback. `record-out.wav` for
   that segment should be near-silent (far-end-only intervals are
   where AEC has the easiest job).
2. Then talk over the playback for the next 2–3 s. This is the
   *double-talk* condition. AEC should preserve your voice while
   continuing to suppress the played speech. Some bleed-through is
   normal — even Chrome's AEC struggles with hard double-talk.

## What can go wrong (and how to tell)

- **No echo to begin with.** If you're wearing headphones, the
  microphone never picks up the played audio so AEC has nothing to
  cancel. `record-in.wav` will already sound clean. Switch to
  loudspeakers for a meaningful test.
- **Volume too low.** Same effect — if the speakers are quiet relative
  to your voice, the echo is below the noise floor. Bump the volume to
  ~50 % and re-run.
- **Duplex device init failure.** miniaudio sometimes can't open a
  duplex device on Linux if PipeWire/PulseAudio aren't routing properly.
  The error will say so. Restarting PipeWire (`systemctl --user restart pipewire`)
  usually fixes it.

## A/B with AEC off

```sh
go run ./examples/aec-record --duration 8s --out-dir runs/aec-off --aec=false
go run ./examples/aec-record --duration 8s --out-dir runs/aec-on
```

`runs/aec-off/record-out.wav` should be near-identical to its
`record-in.wav` (APM is a near-passthrough with everything disabled).
`runs/aec-on/record-out.wav` should have the played speech largely
suppressed.

#!/usr/bin/env python3
"""Convert a music track into the format the game ships.

    python resources/gen/convert_track.py <source.wav> <resources/audio/name.wav> [--rate 22050] [--stereo]

The game's wav decoder (ebiten audio/wav) accepts 8- or 16-bit PCM ONLY, so a
24-bit source does not merely sound wrong, it fails to decode and the game runs
silent. Everything the track is embedded in carries its bytes verbatim, so the
format is also what decides the size of the browser download: mono at 22050 Hz
is a quarter of stereo at 44100 for the same music, and is what the project has
shipped.

Reports the loop join as well, because audio.NewInfiniteLoop repeats the whole
stream with no crossfade: a track that fades out and restarts at full level has
an audible seam every pass.
"""

import argparse
import audioop
import wave


def describe(label, data, rate, width=2, channels=1):
    seconds = len(data) / float(rate * width * channels)
    peak = audioop.max(data, width)
    full = float(1 << (8 * width - 1))
    # Half a second at each end, which is what a seam is heard in.
    edge = int(rate * 0.5) * width * channels
    head, tail = audioop.max(data[:edge], width), audioop.max(data[-edge:], width)
    print(
        "%-10s %8.1f s  %7.2f MB  peak %5.1f%%  first 0.5s %5.1f%%  last 0.5s %5.1f%%"
        % (
            label,
            seconds,
            len(data) / 1e6,
            100 * peak / full,
            100 * head / full,
            100 * tail / full,
        )
    )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("source")
    ap.add_argument("dest")
    ap.add_argument("--rate", type=int, default=22050)
    ap.add_argument("--stereo", action="store_true")
    args = ap.parse_args()

    with wave.open(args.source, "rb") as src:
        channels, width, rate = src.getnchannels(), src.getsampwidth(), src.getframerate()
        frames = src.readframes(src.getnframes())
    print("source     %d ch, %d-bit, %d Hz" % (channels, width * 8, rate))
    describe("source", frames, rate, width, channels)

    if width != 2:
        frames = audioop.lin2lin(frames, width, 2)
    if channels == 2 and not args.stereo:
        frames = audioop.tomono(frames, 2, 0.5, 0.5)
        channels = 1
    elif channels == 1 and args.stereo:
        frames = audioop.tostereo(frames, 2, 1, 1)
        channels = 2
    if rate != args.rate:
        # ratecv filters as it resamples, so halving the rate does not alias.
        frames, _ = audioop.ratecv(frames, 2, channels, rate, args.rate, None)

    with wave.open(args.dest, "wb") as dst:
        dst.setnchannels(channels)
        dst.setsampwidth(2)
        dst.setframerate(args.rate)
        dst.writeframes(frames)

    print("dest       %d ch, 16-bit, %d Hz" % (channels, args.rate))
    describe("dest", frames, args.rate, 2, channels)


if __name__ == "__main__":
    main()

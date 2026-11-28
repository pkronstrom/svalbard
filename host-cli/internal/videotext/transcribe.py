"""Transcribe one local media file in bounded windows at original speed."""
import array
import json
import os
import subprocess
import sys

import sherpa_onnx

RATE = 16000
WINDOW_BYTES = 30 * RATE * 2
audio = subprocess.run(
    ["ffprobe", "-v", "error", "-select_streams", "a:0", "-show_entries",
     "stream=index", "-of", "csv=p=0", sys.argv[1]],
    check=True,
    stdout=subprocess.PIPE,
)
if not audio.stdout.strip():
    sys.exit(0)

model = "/model"
recognizer = sherpa_onnx.OfflineRecognizer.from_transducer(
    encoder=f"{model}/encoder.int8.onnx",
    decoder=f"{model}/decoder.int8.onnx",
    joiner=f"{model}/joiner.int8.onnx",
    tokens=f"{model}/tokens.txt",
    num_threads=min(4, os.cpu_count() or 1),
    model_type="nemo_transducer",
    provider="cpu",
)
# FFmpeg writes bounded raw PCM through a pipe instead of a full-size WAV.
with subprocess.Popen(
    ["ffmpeg", "-v", "error", "-nostdin", "-i", sys.argv[1], "-map", "0:a:0?",
     "-vn", "-ac", "1", "-ar", str(RATE), "-f", "s16le", "pipe:1"],
    stdout=subprocess.PIPE,
) as ffmpeg:
    offset_ms = 0
    while frames := ffmpeg.stdout.read(WINDOW_BYTES):
        samples = array.array("h", frames)
        stream = recognizer.create_stream()
        stream.accept_waveform(RATE, [sample / 32768 for sample in samples])
        recognizer.decode_stream(stream)
        result = stream.result
        if result.text.strip() and result.timestamps:
            position = offset_ms + round(1000 * max(0, min(result.timestamps)))
            print(json.dumps({"start_ms": position, "text": result.text.strip()}), flush=True)
        offset_ms += round(1000 * len(samples) / RATE)
    if ffmpeg.wait():
        sys.exit("FFmpeg could not decode the audio")

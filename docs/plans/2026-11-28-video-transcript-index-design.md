# Searchable video transcripts

Video speech becomes searchable in the portable menu and vault tools. The host
prepares transcripts as an optional indexing pass. The stick carries timed
text and its search index. Playback uses the original video in its ZIM.

## Feasibility on the 256 GB stick

The installed Canadian Prepper archive contains 52 WebM videos, a native ZIM
index and 22 chapter tracks. Keyword search found video pages by title. The
WebVTT tracks we inspected contain chapter labels. The archive listing showed
no separate speech-caption tracks.

We extracted a 600-second video from the archive, decoded mono 16 kHz audio
with FFmpeg and transcribed it in 30-second chunks on this Apple Silicon Mac.
Parakeet TDT 0.6B v3 int8 took 14.0 seconds to decode and 15.5 seconds including
model load and writing results. Peak process memory was 2.7 GiB. The output
was 11 KB of timed text before indexing. The model download was 465 MiB. A
short synthetic Finnish sample returned recognizable text with two spelling
errors. A one-thread, 15-second-window run on the English sample took
3.9 seconds per audio minute and used about 2.4 GiB peak RAM.

A scratch FTS5 index found a phrase from the 60-second excerpt. Kiwix served
the original WebM over byte ranges and headless Chrome sought to the result
timestamp at 30.16 seconds. These are measurements from one host and video.
Measure accuracy on natural Finnish speech and performance on a modest host
before setting platform defaults.

### Audio tempo

We kept the original 30-second windows fixed and ran the same 10-minute
English video and synthetic Finnish sentence at six tempos. Parakeet used
four CPU threads and a warm model. Times include applying FFmpeg's tempo
filter to the English audio. The English edit rate compares each result to
the 1× transcript. Finnish edits use the known 18-word script.

| Audio tempo | English time | English words | English edits vs 1× | Finnish edits |
| --- | ---: | ---: | ---: | ---: |
| 1× | 14.07 s | 1,924 | 0.0% | 2 |
| 1.25× | 12.37 s | 1,973 | 4.9% | 1 |
| 1.5× | 10.39 s | 1,966 | 6.5% | 3 |
| 2× | 8.19 s | 1,773 | 15.1% | 7 |
| 3× | 5.77 s | 895 | 75.1% | 17 |
| 4× | 4.50 s | 275 | 94.5% | 18 |

Keep original speed as the default. Test 1.25× against human-checked
English and natural Finnish before offering a faster option. At 3× and
4× the transcript skipped most of the English speech. The failed 4×
quality check ended the speed sweep.

If an accelerated mode is added, calculate the original video time as
the chunk start plus the recognizer timestamp multiplied by the tempo.
Matching English tokens showed a median 40 ms shift relative to the 1×
transcript after this conversion at both 1.25× and 1.5×. Record tempo
in the cache identity. For tests above 2×, chain FFmpeg tempo filters so
each stage stays at or below 2×.

## Apply and index workflow

1. Scan the selected archive for video entries and their existing subtitle or
   chapter tracks. Import usable captions directly. Preserve chapter labels
   as timestamped headings and mark which videos still lack speech text.
2. Run opt-in recognition at original speed in a dedicated host-side Docker
   image. FFmpeg decodes bounded audio windows and Parakeet processes one
   video at a time. Cache the model weights on the host. Use a video count
   to limit a run. Native recognition was used for the feasibility test.
3. Publish each completed video's timed passages as a source in the vault's
   existing FTS index under the index writer lock. The source records the
   parent archive identity and whether its captions or speech are complete.
   Reapply with the same flag to resume. Semantic coverage can be added
   through the existing selective embedding pass.
4. Open each hit at its start time in the original video. Use the existing
   loopback Kiwix viewer and a media fragment on the asset URL. Keep exact
   archive, video path and timestamp available to tools and archive filters.

Start with one YouTube-style archive. Test a chapter hit, a speech-only hit,
cancel and resume, an archive refresh, and playback at the matched timestamp.
Keep the ordinary apply path free of model downloads and transcription work.

References

- [NVIDIA Parakeet v3 model card](https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3)
- [Sherpa-ONNX quantized Parakeet and CPU example](https://k2-fsa.github.io/sherpa/onnx/pretrained_models/offline-transducer/nemo-transducer-models.html)
- [FFmpeg tempo filter](https://ffmpeg.org/ffmpeg-filters.html#atempo)
- [Whisper.cpp model sizes](https://github.com/ggml-org/whisper.cpp/blob/master/models/README.md)

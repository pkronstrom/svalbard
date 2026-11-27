# Tone.js

Sound in the page: synthesizers, effects, sequences and timing. Browsers start audio only after a click, so start it from a button. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import * as Tone from "tone";
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Tone</title></head>
<body>
<button id="play">Play</button>
<script type="module">
import * as Tone from "tone";
document.getElementById("play").onclick = async () => {
  await Tone.start();
  const synth = new Tone.PolySynth(Tone.Synth).toDestination();
  const now = Tone.now();
  ["C4", "E4", "G4", "C5"].forEach((note, i) => synth.triggerAttackRelease(note, "8n", now + i * 0.25));
};
</script>
</body></html>
```

## Notes

- Effects chain: `new Tone.Synth().connect(new Tone.Reverb(2).toDestination())`.
- Loops: `new Tone.Loop(time => synth.triggerAttackRelease("C4", "16n", time), "4n").start(0); Tone.getTransport().start();`.

## Reference

`tone.reference.md` (`skill_read`, file `tone.reference.md`): the README and the type definitions of every class.

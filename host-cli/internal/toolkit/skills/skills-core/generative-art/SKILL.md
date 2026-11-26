---
name: generative-art
description: Make seeded generative art with p5.js (flow fields, particles, noise, geometric patterns) with sliders for its parameters and a PNG download, in one HTML page that opens offline. Same seed, same picture.
requires: [js-media]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned (keep the quotes
around it in commands: a drive's path can hold spaces). Work in the user's
working folder, never inside `<folder>`.

1. Copy the template:

   ```sh
   cp "<folder>"/templates/sketch.html sketch.html
   ```

2. Decide the idea in a sentence (for example "ink rivers following
   noise", "circles packing into a grid that breaks apart"), then edit the
   marked regions of `sketch.html`:
   - `EDIT: parameters`: the title, the sliders (`value`, `min`, `max`,
     `step`), the palette and the canvas size. Keep `seed`.
   - `EDIT: drawing`: the function `art(s, v)`. `s` is the p5 sketch, so
     every p5 call is `s.circle`, `s.line`, `s.noise`, `s.random`;
     `v` holds the slider values (`v.particles`). Use `s.random` and
     `s.noise` only: the seed makes them repeat. Draw the whole picture in
     one call (the sketch does not animate).
3. Pack it into one file:

   ```sh
   svalbard-drive html pack sketch.html -o sketch-packed.html
   ```

4. Check your result. It must print `ok`:

   ```sh
   sh "<folder>"/examples/check.sh sketch-packed.html
   ```

5. Tell the user the packed file's full path. Opened by double-click, the
   sliders redraw the picture, "New seed" makes a new one, "Save PNG"
   downloads it.

## Ideas and p5 notes

- This drive has p5.js 1.x: `s.preload`, `s.loadImage` and `s.noise` work
  as in most p5 examples; write them as `s.` methods (instance mode).
- Flow fields: angle from `s.noise(x * scale, y * scale)`; particles: many
  short lines with low alpha (`c.setAlpha(40)`); patterns: nested loops
  over a grid with `s.random` breaking the order.
- Motion instead of a still: remove `s.noLoop()` and draw in `s.draw`.
- The use case follows Anthropic's algorithmic-art skill (Apache-2.0); the
  template is written for this drive, with its own p5.js and no CDN or web
  fonts.

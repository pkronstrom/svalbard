---
name: parametric-cad
description: Make a parametric 3D-printable part (a spur gear, plates, spacers) in millimetres with JSCAD, preview it in 3D with Three.js and download it as an STL file, all in one HTML page that works offline. Starts from a tested gear template.
requires: [js-3d-cad]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned (keep the quotes
around it in commands: a drive's path can hold spaces). Work in the user's
working folder, never inside `<folder>`.

1. Copy the template:

   ```sh
   cp -n "<folder>"/templates/gear.html gear.html
   ```

   `-n` keeps a file of that name already there: then pick another name.

2. For a spur gear, edit only the region `EDIT: parameters`: the title and
   each parameter's `value` (module, teeth, thickness, bore, pressure angle,
   backlash, all in mm or degrees). Each parameter becomes an input in the
   page, so the user can change it later. Keep the geometry below as it
   is: it is tested to give a watertight mesh.
3. For another part, keep the page and change `buildGear` to build your
   solid from JSCAD primitives, and `warnings` to your checks. Cut holes in
   2D before `extrudeLinear` (see `references/printability.md`). The JSCAD
   quickstart (`skill_search` "jscad" with `kind` library) lists the
   functions.
4. Pack it into one file:

   ```sh
   svalbard-drive html pack gear.html -o gear-packed.html
   ```

5. Check your result. It must print `ok`:

   ```sh
   sh "<folder>"/examples/check.sh gear-packed.html
   ```

6. Tell the user the packed file's full path. Opened by double-click it
   shows the part in 3D; "Download STL" saves the mesh for a slicer.

## Gear notes

- Pitch diameter = module × teeth; outside diameter = module × (teeth + 2).
- Gears that mesh need the same module and pressure angle; centre distance
  = module × (teeth1 + teeth2) / 2.
- The page warns about thin walls around the bore, thin tooth tips, a
  module under 0.8 mm and fewer than 13 teeth.
- `sh "<folder>"/examples/check.sh part.stl` validates an STL file itself
  (watertight, positive volume).

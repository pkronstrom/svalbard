# Printability of parametric parts

Rules of thumb for FDM printers with a 0.4 mm nozzle and 0.2 mm layers.
The gear template checks the first four and shows a warning; for other
parts, compute them yourself before telling the user a part is ready.

## Walls and features

- Walls at least 0.8 mm (two lines); 1.2 mm or more where the part takes
  load. Between a bore and the nearest outside edge keep 2 mm.
- Pins and ribs at least 1 mm across. Text embossed at least 0.6 mm high
  and 0.5 mm wide strokes.
- Overhangs up to 45° print without support; bridges up to about 10 mm.

## Holes and fits

- Small holes print undersize by about 0.1 to 0.2 mm. For a shaft of
  diameter d: a free (sliding) fit is d + 0.3 mm, a press fit d + 0.1 mm.
- Holes along the print direction (vertical) are round; horizontal holes
  sag at the top: make them teardrop-shaped or add 0.2 mm.
- Clearance between moving parts printed apart: 0.3 to 0.5 mm per side.

## Gears

- Module 1 mm is about the finest that prints cleanly; 1.5 to 2 mm is
  robust. Two gears mesh only when they have the same module and pressure
  angle; their centre distance is m × (z1 + z2) / 2.
- Fewer than 13 teeth at 20° pressure angle undercuts the roots (weaker
  teeth). Backlash 0.1 to 0.2 mm keeps printed gears from binding.
- Tooth tips thinner than 0.8 mm print poorly; use a larger module.
- Face width (thickness) about 6 to 10 × module.

## Checking an exported file

`sh "<folder>"/examples/check.sh part.stl` checks the header, the triangle
count, that the mesh is watertight (every edge joins exactly two triangles)
and that it encloses a positive volume. Build holes from the 2D outline
(`booleans.subtract` on 2D shapes, then `extrudeLinear`) where you can: a
3D subtract can leave edges that meet mid-edge, which this check, and
slicers, call not watertight.

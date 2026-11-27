"""Validate an STL file for 3D printing, with the standard library.

    python3 validate_stl.py FILE.stl [--min-triangles N]

Binary or text STL. Checks the header and triangle count, that the mesh is
watertight (every edge belongs to exactly two triangles) and that it
encloses a positive volume. Prints the counts and "ok", or why not.
"""
import struct
import sys


def triangles(data):
    if len(data) >= 84:
        (count,) = struct.unpack_from("<I", data, 80)
        if 84 + 50 * count == len(data):
            for i in range(count):
                values = struct.unpack_from("<12f", data, 84 + 50 * i)
                yield values[3:6], values[6:9], values[9:12]
            return
    text = data.decode("ascii", "replace")
    if not text.lstrip().startswith("solid"):
        raise SystemExit("not an STL file: neither binary (80-byte header, count, 50 bytes a triangle) nor text")
    verts = [tuple(float(v) for v in line.split()[1:4]) for line in text.splitlines() if line.strip().startswith("vertex")]
    for i in range(0, len(verts) - 2, 3):
        yield verts[i], verts[i + 1], verts[i + 2]


def main(argv):
    if not argv:
        raise SystemExit(__doc__)
    path = argv[0]
    need = int(argv[argv.index("--min-triangles") + 1]) if "--min-triangles" in argv else 1
    data = open(path, "rb").read()
    tris = list(triangles(data))
    if len(tris) < need:
        raise SystemExit("%s has %d triangles, want at least %d" % (path, len(tris), need))
    key = lambda p: tuple(round(c, 4) for c in p)
    edges = {}
    volume = 0.0
    for a, b, c in tris:
        ka, kb, kc = key(a), key(b), key(c)
        for e in ((ka, kb), (kb, kc), (kc, ka)):
            k = tuple(sorted(e))
            edges[k] = edges.get(k, 0) + 1
        volume += (a[0] * (b[1] * c[2] - b[2] * c[1]) - a[1] * (b[0] * c[2] - b[2] * c[0]) + a[2] * (b[0] * c[1] - b[1] * c[0])) / 6
    open_edges = sum(1 for n in edges.values() if n != 2)
    if open_edges:
        raise SystemExit("%s is not watertight: %d of %d edges do not join exactly two triangles" % (path, open_edges, len(edges)))
    if volume <= 0:
        raise SystemExit("%s encloses no positive volume (%.1f mm3): the triangles may face inward" % (path, volume))
    print("%s: %d triangles, watertight, volume %.0f mm3: ok" % (path, len(tris), volume))


if __name__ == "__main__":
    main(sys.argv[1:])

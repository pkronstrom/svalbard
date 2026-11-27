#!/usr/bin/env py-docs
"""Export a layer of a GeoPackage to GeoJSON, with the Python standard library.

    py-docs gpkg_to_geojson.py GPKG --list
    py-docs gpkg_to_geojson.py GPKG LAYER OUT.geojson [--columns A,B] [--where SQL] [--precision N]

GPKG is a path, or the name of a GeoPackage in the drive's data/ folder
(natural_earth_vector.gpkg). --list prints the layers with their feature
counts and columns. --columns keeps those attribute columns (default: all),
--where filters rows with an SQL condition ("POP_EST > 0"), --precision
rounds coordinates to N decimals (default 4).

Geometries: Point, LineString, Polygon and their Multi kinds, in either byte
order, with Z or M values dropped. Anything else is counted and reported as
unsupported, never dropped silently.
"""
import json
import math
import os
import sqlite3
import struct
import sys
import urllib.parse

USAGE = __doc__.split("\n\n")[1]


class Unsupported(Exception):
    pass


class EmptyPoint(Exception):
    """A point written as NaN NaN, the WKB form of an empty point."""


def find_gpkg(name):
    """A path, or a GeoPackage in the data/ folder of the drive."""
    if os.path.exists(name):
        return name
    here = os.path.dirname(os.path.abspath(__file__))
    candidates = []
    for root in (os.environ.get("DRIVE_ROOT"), os.environ.get("DRIVE"), os.path.join(here, "..", "..", "..", "..")):
        if root:
            candidates.append(os.path.join(root, "data", os.path.basename(name)))
    for path in candidates:
        if os.path.exists(path):
            return os.path.normpath(path)
    sys.exit("no GeoPackage %s here or in the drive's data/ folder" % name)


def header_size(blob):
    """Length of the GeoPackage binary header before the WKB."""
    if len(blob) < 8 or blob[0:2] != b"GP":
        raise Unsupported("not a GeoPackage geometry (no GP header)")
    flags = blob[3]
    envelope = (flags >> 1) & 0x07
    sizes = {0: 0, 1: 32, 2: 48, 3: 48, 4: 64}
    if envelope not in sizes:
        raise Unsupported("unknown envelope type %d" % envelope)
    return 8 + sizes[envelope], bool(flags & 0x10)


class WKB:
    def __init__(self, data, precision):
        self.data, self.pos, self.precision = data, 0, precision

    def read(self, fmt):
        value = struct.unpack_from(fmt, self.data, self.pos)
        self.pos += struct.calcsize(fmt)
        return value

    def geometry(self):
        order = "<" if self.read("B")[0] == 1 else ">"
        (code,) = self.read(order + "I")
        dims = 2
        if code & 0x80000000 or code & 0x40000000:  # EWKB Z / M flags
            dims += bool(code & 0x80000000) + bool(code & 0x40000000)
            code &= 0x0FFFFFFF
        if code & 0x20000000:  # EWKB SRID
            self.read(order + "I")
            code &= ~0x20000000
        if 1000 <= code < 4000:
            dims = {1: 3, 2: 3, 3: 4}[code // 1000]
            code %= 1000
        if code == 1:
            return {"type": "Point", "coordinates": self.point(order, dims)}
        if code == 2:
            return {"type": "LineString", "coordinates": self.points(order, dims)}
        if code == 3:
            return {"type": "Polygon", "coordinates": self.rings(order, dims)}
        if code in (4, 5, 6):
            (count,) = self.read(order + "I")
            parts = [self.geometry() for _ in range(count)]
            kind = {4: "MultiPoint", 5: "MultiLineString", 6: "MultiPolygon"}[code]
            return {"type": kind, "coordinates": [p["coordinates"] for p in parts]}
        raise Unsupported("WKB geometry type %d" % code)

    def point(self, order, dims):
        values = self.read(order + "d" * dims)
        if math.isnan(values[0]) or math.isnan(values[1]):
            raise EmptyPoint()
        return [round(values[0], self.precision), round(values[1], self.precision)]

    def points(self, order, dims):
        (count,) = self.read(order + "I")
        return [self.point(order, dims) for _ in range(count)]

    def rings(self, order, dims):
        (count,) = self.read(order + "I")
        return [self.points(order, dims) for _ in range(count)]


def to_geometry(blob, precision):
    if blob is None:
        return None
    start, empty = header_size(blob)
    if empty:
        return None
    try:
        return WKB(blob[start:], precision).geometry()
    except EmptyPoint:
        return None
    except struct.error:
        raise Unsupported("truncated geometry")


def open_db(path):
    # Quoted, so a "#" or "?" in the path cannot cut off mode=ro.
    return sqlite3.connect("file:%s?mode=ro" % urllib.parse.quote(os.path.abspath(path)), uri=True)


def layers(db):
    return [row[0] for row in db.execute("SELECT table_name FROM gpkg_contents ORDER BY table_name")]


def geometry_column(db, layer):
    try:
        row = db.execute("SELECT column_name FROM gpkg_geometry_columns WHERE table_name = ?", (layer,)).fetchone()
        if row:
            return row[0]
    except sqlite3.Error:
        pass
    return "geom"


def main(argv):
    if len(argv) >= 2 and argv[1] == "--list":
        db = open_db(find_gpkg(argv[0]))
        for layer in layers(db):
            columns = [r[1] for r in db.execute('PRAGMA table_info("%s")' % layer.replace('"', '""'))]
            count = db.execute('SELECT count(*) FROM "%s"' % layer.replace('"', '""')).fetchone()[0]
            print("%s  %d features  columns: %s" % (layer, count, ", ".join(columns)))
        return 0
    if len(argv) < 3:
        sys.exit("usage:\n" + USAGE)
    gpkg, layer, out = argv[0], argv[1], argv[2]
    columns, where, precision = None, None, 4
    rest = argv[3:]
    while rest:
        flag = rest.pop(0)
        if flag == "--columns" and rest:
            columns = [c.strip() for c in rest.pop(0).split(",") if c.strip()]
        elif flag == "--where" and rest:
            where = rest.pop(0)
        elif flag == "--precision" and rest:
            precision = int(rest.pop(0))
        else:
            sys.exit("unknown option %s\nusage:\n%s" % (flag, USAGE))
    db = open_db(find_gpkg(gpkg))
    if layer not in layers(db):
        sys.exit("no layer %s; --list shows the layers" % layer)
    geom = geometry_column(db, layer)
    quoted = '"%s"' % layer.replace('"', '""')
    all_columns = [r[1] for r in db.execute("PRAGMA table_info(%s)" % quoted)]
    keep = columns or [c for c in all_columns if c != geom and c != "fid"]
    missing = [c for c in keep if c not in all_columns]
    if missing:
        sys.exit("no column %s in %s; it has: %s" % (", ".join(missing), layer, ", ".join(all_columns)))
    sql = "SELECT %s, %s FROM %s" % ('"%s"' % geom, ", ".join('"%s"' % c for c in keep) or "1", quoted)
    if where:
        sql += " WHERE " + where
    features, unsupported = [], {}
    minx = miny = math.inf
    maxx = maxy = -math.inf
    for row in db.execute(sql):
        try:
            geometry = to_geometry(row[0], precision)
        except Unsupported as error:
            unsupported[str(error)] = unsupported.get(str(error), 0) + 1
            continue
        props = dict(zip(keep, row[1:])) if keep else {}
        features.append({"type": "Feature", "properties": props, "geometry": geometry})
        for x, y in coordinates(geometry):
            minx, miny, maxx, maxy = min(minx, x), min(miny, y), max(maxx, x), max(maxy, y)
    with open(out, "w", encoding="utf-8") as f:
        json.dump({"type": "FeatureCollection", "features": features}, f, ensure_ascii=False, separators=(",", ":"))
    print("wrote %s: %d features from %s, bbox %s" % (out, len(features), layer, [round(v, 2) for v in (minx, miny, maxx, maxy)] if features else []))
    for reason, count in unsupported.items():
        print("unsupported: %d features skipped (%s)" % (count, reason))
    return 0


def coordinates(geometry):
    if not geometry:
        return
    stack = [geometry["coordinates"]]
    while stack:
        item = stack.pop()
        if item and isinstance(item[0], (int, float)):
            yield item[0], item[1]
        else:
            stack.extend(item)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

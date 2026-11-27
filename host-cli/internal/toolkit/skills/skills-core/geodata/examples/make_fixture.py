"""Write a small GeoPackage that exercises the exporter: both byte orders,
each envelope size, a Z value, an empty geometry and an unsupported type.

    py-docs make_fixture.py OUT.gpkg
"""
import sqlite3
import struct
import sys


def gp(wkb, order="<", envelope=0, empty=False, srs=4326):
    sizes = {0: 0, 1: 4, 2: 6, 3: 6, 4: 8}
    flags = (1 if order == "<" else 0) | (envelope << 1) | (0x10 if empty else 0)
    head = b"GP" + bytes([0, flags]) + struct.pack(order + "i", srs)
    return head + struct.pack(order + "d" * sizes[envelope], *([0.0] * sizes[envelope])) + wkb


def point(order, x, y):
    return bytes([1 if order == "<" else 0]) + struct.pack(order + "Idd", 1, x, y)


def polygon(order, ring, code=3, z=False):
    body = bytes([1 if order == "<" else 0]) + struct.pack(order + "II", code, 1) + struct.pack(order + "I", len(ring))
    for x, y in ring:
        body += struct.pack(order + ("ddd" if z else "dd"), *((x, y, 5.0) if z else (x, y)))
    return body


square = [(20.6, 59.8), (31.6, 59.8), (31.6, 70.1), (20.6, 70.1), (20.6, 59.8)]
db = sqlite3.connect(sys.argv[1])
db.executescript("""
CREATE TABLE gpkg_contents (table_name TEXT PRIMARY KEY, data_type TEXT);
CREATE TABLE gpkg_geometry_columns (table_name TEXT, column_name TEXT);
CREATE TABLE places (fid INTEGER PRIMARY KEY, shape BLOB, NAME TEXT, VALUE REAL);
INSERT INTO gpkg_contents VALUES ('places', 'features');
INSERT INTO gpkg_geometry_columns VALUES ('places', 'shape');
""")
multi_z = b"\x01" + struct.pack("<II", 1006, 1) + polygon("<", square, code=1003, z=True)
collection = b"\x01" + struct.pack("<II", 7, 1) + point("<", 1, 2)
rows = [
    (gp(point("<", 24.94, 60.17)), "point-le", 1),
    (gp(polygon(">", square), order=">", envelope=1), "polygon-be", 2),
    (gp(multi_z, envelope=2), "multipolygon-z", 3),
    (gp(b"", empty=True), "empty", 4),
    (gp(collection), "collection", 5),
    (gp(point("<", float("nan"), float("nan"))), "empty-point", 6),
    (gp(point("<", 1, 2)[:12]), "truncated", 7),
]
db.executemany("INSERT INTO places (shape, NAME, VALUE) VALUES (?, ?, ?)", rows)
db.commit()

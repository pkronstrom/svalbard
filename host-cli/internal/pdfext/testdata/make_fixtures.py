"""Write the PDF fixtures of this directory: python3 make_fixtures.py .

text.pdf   three text pages (the second in Finnish), a Title and an outline
scan.pdf   two pages that are only an image, like a scan without OCR
mixed.pdf  text pages alternating with image-only pages
hyphens.pdf  one page whose lines break inside words and at real hyphens

enc-user.pdf and enc-owner.pdf are text.pdf encrypted with AES-256 by pdfcpu
(api.EncryptFile with model.NewAESConfiguration): the first with the user
password "secret", the second with an owner password only, so it opens
without a password.
"""
import sys, zlib

def build(pages, title=None, outline=None):
    """pages: list of ("text", [lines]) or ("image",) ; outline: [(title, page_index)]"""
    objs = {}
    def add(num, body):
        objs[num] = body
    n_pages = len(pages)
    # object numbers: 1 catalog, 2 pages, 3 font, 4 image, then per page: page, content
    page_nums = [5 + 2 * i for i in range(n_pages)]
    kids = " ".join(f"{n} 0 R" for n in page_nums)
    add(2, f"<< /Type /Pages /Kids [{kids}] /Count {n_pages} >>".encode())
    add(3, b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
    img = bytes([200, 60, 60, 200])  # 2x2 gray
    add(4, b"<< /Type /XObject /Subtype /Image /Width 2 /Height 2 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length %d >>\nstream\n" % len(img) + img + b"\nendstream")
    for i, page in enumerate(pages):
        if page[0] == "text":
            ops = ["BT", "/F1 12 Tf", "14 TL", "72 720 Td"]
            for line in page[1]:
                esc = "".join("\\%03o" % b if b > 126 or chr(b) in "()\\" else chr(b) for b in line.encode("cp1252"))
                ops.append(f"({esc}) Tj T*")
            ops.append("ET")
            content = "\n".join(ops).encode()
            res = b"<< /Font << /F1 3 0 R >> >>"
        else:
            content = b"q 468 0 0 648 72 72 cm /Im1 Do Q"
            res = b"<< /XObject << /Im1 4 0 R >> >>"
        add(page_nums[i], b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources " + res + b" /Contents %d 0 R >>" % (page_nums[i] + 1))
        add(page_nums[i] + 1, b"<< /Length %d >>\nstream\n" % len(content) + content + b"\nendstream")
    nxt = 5 + 2 * n_pages
    catalog = "<< /Type /Catalog /Pages 2 0 R"
    if outline:
        root = nxt
        items = [root + 1 + i for i in range(len(outline))]
        add(root, f"<< /Type /Outlines /First {items[0]} 0 R /Last {items[-1]} 0 R /Count {len(items)} >>".encode())
        for i, (text, page_index) in enumerate(outline):
            d = f"<< /Title ({text}) /Parent {root} 0 R /Dest [{page_nums[page_index]} 0 R /Fit]"
            if i > 0: d += f" /Prev {items[i-1]} 0 R"
            if i < len(items) - 1: d += f" /Next {items[i+1]} 0 R"
            add(items[i], (d + " >>").encode())
        catalog += f" /Outlines {root} 0 R"
        nxt = items[-1] + 1
    add(1, (catalog + " >>").encode())
    info = None
    if title:
        info = nxt
        add(info, f"<< /Title ({title}) >>".encode())
    out = bytearray(b"%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
    offsets = {}
    for num in sorted(objs):
        offsets[num] = len(out)
        out += b"%d 0 obj\n" % num + objs[num] + b"\nendobj\n"
    xref = len(out)
    size = max(objs) + 1
    out += b"xref\n0 %d\n" % size + b"0000000000 65535 f \n"
    for num in range(1, size):
        out += b"%010d 00000 n \n" % offsets[num]
    trailer = f"<< /Size {size} /Root 1 0 R" + (f" /Info {info} 0 R" if info else "") + " >>"
    out += b"trailer\n" + trailer.encode() + b"\nstartxref\n%d\n%%%%EOF\n" % xref
    return bytes(out)

dest = sys.argv[1]
text_pages = [
    ("text", ["Cooling system", "Coolant capacity incl. heater: 9.5 litres.", "Use only type C coolant and check the level", "when the engine is cold."]),
    ("text", ["Jäähdytysjärjestelmä", "Jäähdytysnesteen määrä lämmityslaitteineen on 9,5 litraa.", "Tarkista nestemäärä moottorin ollessa kylmä, öljy erikseen."]),
    ("text", ["Brakes", "Brake fluid: DOT 4. Replace every two years", "or after 40 000 km, whichever comes first."]),
]
open(dest + "/text.pdf", "wb").write(build(text_pages, title="Fixture Owner's Manual", outline=[("Cooling system", 0), ("Brakes", 2)]))
open(dest + "/scan.pdf", "wb").write(build([("image",), ("image",)]))
open(dest + "/mixed.pdf", "wb").write(build([("text", ["Wiring diagrams", "The following pages are scanned drawings of the harness."]), ("image",), ("text", ["Fuse 7 feeds the cooling fan relay and the horn."]), ("image",)]))
hyphen_lines = [
    "Many opioid medications are longer-",
    "acting and may need more doses. Repeat the medica-",
    "tions as needed and monitor the patient for respira-",
    "tory depression. Give medications by mouth where",
    "possible. A longer course of acting on symptoms is not",
    "advised; use a long-",
    "acting agent, as with other long-acting drugs. The Anglo-",
    "Saxon chart lists doses of 10-",
    "20 mg.",
]
open(dest + "/hyphens.pdf", "wb").write(build([("text", hyphen_lines)]))

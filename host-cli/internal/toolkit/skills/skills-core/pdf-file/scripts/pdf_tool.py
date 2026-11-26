"""Make, combine, take apart, read and fill PDF files. Run with py-docs:

  py-docs pdf_tool.py create IN.md OUT.pdf [--size A4|letter]
  py-docs pdf_tool.py merge OUT.pdf IN1.pdf IN2.pdf ...
  py-docs pdf_tool.py pages IN.pdf OUT.pdf 1-3,5
  py-docs pdf_tool.py split IN.pdf OUTDIR
  py-docs pdf_tool.py text IN.pdf [--pages 1-3]
  py-docs pdf_tool.py fields IN.pdf
  py-docs pdf_tool.py fill IN.pdf VALUES.json OUT.pdf
  py-docs pdf_tool.py info IN.pdf

create reads simple markdown: "# Title", "## Heading", "### Subheading",
paragraphs separated by blank lines, "- item" bullets, "1. item" numbered
lists, "| a | b |" tables (the first row is the header), "![caption](image.png)"
images, **bold** and *italic*, and a line "<<<pagebreak>>>" for a new page.
Page numbers are 1-based.
"""
import argparse
import json
import re
import sys
from pathlib import Path
from xml.sax.saxutils import escape

from pypdf import PdfReader, PdfWriter
from pypdf.generic import NameObject, BooleanObject

PAGEBREAK = "<<<pagebreak>>>"


def inline(text):
    """Markdown bold and italic to reportlab paragraph markup."""
    text = escape(text)
    text = re.sub(r"\*\*(.+?)\*\*", r"<b>\1</b>", text)
    text = re.sub(r"(?<![\w*])\*(?!\s)(.+?)(?<!\s)\*(?![\w*])", r"<i>\1</i>", text)
    text = re.sub(r"`(.+?)`", r"<font face='Courier'>\1</font>", text)
    return text


def table_cells(line):
    return [c.strip() for c in line.strip().strip("|").split("|")]


def parse_markdown(text):
    """Return a list of (kind, value) blocks."""
    blocks, para = [], []
    lines = text.splitlines()

    def flush():
        if para:
            blocks.append(("p", " ".join(para)))
            para.clear()

    i = 0
    while i < len(lines):
        line = lines[i].rstrip()
        s = line.strip()
        if not s:
            flush()
        elif s == PAGEBREAK:
            flush()
            blocks.append(("break", None))
        elif m := re.match(r"(#{1,3})\s+(.*)", s):
            flush()
            blocks.append((f"h{len(m.group(1))}", m.group(2)))
        elif m := re.match(r"!\[(.*?)\]\((.+?)\)", s):
            flush()
            blocks.append(("img", (m.group(2), m.group(1))))
        elif re.match(r"[-*]\s+", s) or re.match(r"\d+[.)]\s+", s):
            flush()
            numbered = bool(re.match(r"\d+[.)]\s+", s))
            items = []
            while i < len(lines) and (m := re.match(r"\s*(?:[-*]|\d+[.)])\s+(.*)", lines[i])):
                items.append(m.group(1))
                i += 1
            blocks.append(("ol" if numbered else "ul", items))
            continue
        elif s.startswith("|"):
            flush()
            rows = []
            while i < len(lines) and lines[i].strip().startswith("|"):
                cells = table_cells(lines[i])
                if not all(re.fullmatch(r":?-{2,}:?", c) for c in cells if c):
                    rows.append(cells)
                i += 1
            blocks.append(("table", rows))
            continue
        else:
            para.append(s)
        i += 1
    flush()
    return blocks


def create(args):
    from reportlab.lib import colors
    from reportlab.lib.pagesizes import A4, letter
    from reportlab.lib.styles import getSampleStyleSheet
    from reportlab.lib.units import mm
    from reportlab.platypus import (Image, ListFlowable, ListItem, PageBreak, Paragraph,
                                    SimpleDocTemplate, Spacer, Table, TableStyle)

    source = Path(args.source)
    styles = getSampleStyleSheet()
    size = letter if args.size.lower() == "letter" else A4
    doc = SimpleDocTemplate(args.out, pagesize=size, leftMargin=20 * mm, rightMargin=20 * mm,
                            topMargin=20 * mm, bottomMargin=20 * mm)
    title = None
    story = []
    for kind, value in parse_markdown(source.read_text(encoding="utf-8")):
        if kind == "h1":
            title = title or value
            story.append(Paragraph(inline(value), styles["Title"]))
        elif kind in ("h2", "h3"):
            story.append(Paragraph(inline(value), styles["Heading2" if kind == "h2" else "Heading3"]))
        elif kind == "p":
            story.append(Paragraph(inline(value), styles["BodyText"]))
        elif kind in ("ul", "ol"):
            items = [ListItem(Paragraph(inline(v), styles["BodyText"])) for v in value]
            story.append(ListFlowable(items, bulletType="1" if kind == "ol" else "bullet", start="1" if kind == "ol" else None))
        elif kind == "table":
            width = max(len(r) for r in value)
            rows = [[Paragraph(inline(c), styles["BodyText"]) for c in r + [""] * (width - len(r))] for r in value]
            table = Table(rows, repeatRows=1, hAlign="LEFT", spaceBefore=2 * mm)
            table.setStyle(TableStyle([
                ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#DDE4EE")),
                ("GRID", (0, 0), (-1, -1), 0.5, colors.grey),
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
            ]))
            story.append(table)
            story.append(Spacer(1, 4 * mm))
        elif kind == "img":
            path, caption = value
            image_path = (source.parent / path) if not Path(path).is_absolute() else Path(path)
            if not image_path.exists():
                sys.exit(f"image {image_path} not found")
            img = Image(str(image_path))
            scale = min(1.0, doc.width / img.drawWidth, (doc.height * 0.6) / img.drawHeight)
            img.drawWidth, img.drawHeight = img.drawWidth * scale, img.drawHeight * scale
            story.append(img)
            if caption:
                story.append(Paragraph(inline(caption), styles["Italic"]))
        elif kind == "break":
            story.append(PageBreak())
    if title:
        doc.title = title

    def number_pages(canvas, document):
        canvas.setFont("Helvetica", 8)
        canvas.drawRightString(size[0] - 20 * mm, 10 * mm, str(document.page))

    doc.build(story, onFirstPage=number_pages, onLaterPages=number_pages)
    print(f"wrote {args.out}: {len(PdfReader(args.out).pages)} pages")


def page_numbers(spec, count):
    """'1-3,5' to zero-based indexes, checked against count."""
    if not spec:
        return list(range(count))
    out = []
    for part in spec.split(","):
        part = part.strip()
        if "-" in part:
            a, b = part.split("-", 1)
            first, last = int(a or 1), int(b or count)
        else:
            first = last = int(part)
        if not (1 <= first <= last <= count):
            sys.exit(f"pages {part} outside 1-{count}")
        out.extend(range(first - 1, last))
    return out


def merge(args):
    writer = PdfWriter()
    for path in args.inputs:
        writer.append(path)
    writer.write(args.out)
    print(f"wrote {args.out}: {len(writer.pages)} pages from {len(args.inputs)} files")


def pages(args):
    reader = PdfReader(args.pdf)
    writer = PdfWriter()
    for index in page_numbers(args.spec, len(reader.pages)):
        writer.add_page(reader.pages[index])
    writer.write(args.out)
    print(f"wrote {args.out}: {len(writer.pages)} pages")


def split(args):
    reader = PdfReader(args.pdf)
    out = Path(args.outdir)
    out.mkdir(parents=True, exist_ok=True)
    stem = Path(args.pdf).stem
    for index, page in enumerate(reader.pages, start=1):
        writer = PdfWriter()
        writer.add_page(page)
        writer.write(out / f"{stem}-{index:03d}.pdf")
    print(f"wrote {len(reader.pages)} files to {out}")


def text(args):
    reader = PdfReader(args.pdf)
    for index in page_numbers(args.pages, len(reader.pages)):
        print(f"--- page {index + 1} ---")
        print(reader.pages[index].extract_text() or "")


def fields(args):
    found = PdfReader(args.pdf).get_fields() or {}
    if not found:
        print("no form fields")
    for name, field in found.items():
        print(f"{name}\t{field.get('/V', '')}\t{field.get('/FT', '')}")


def fill(args):
    values = json.loads(Path(args.values).read_text(encoding="utf-8"))
    reader = PdfReader(args.pdf)
    known = reader.get_fields() or {}
    missing = [k for k in values if k not in known]
    if missing:
        sys.exit(f"no such fields: {', '.join(missing)}; the form has: {', '.join(known) or 'none'}")
    writer = PdfWriter(clone_from=reader)
    for page in writer.pages:
        writer.update_page_form_field_values(page, {k: str(v) for k, v in values.items()}, auto_regenerate=False)
    writer._root_object["/AcroForm"][NameObject("/NeedAppearances")] = BooleanObject(True)
    writer.write(args.out)
    print(f"wrote {args.out}: filled {len(values)} fields")


def info(args):
    reader = PdfReader(args.pdf)
    meta = reader.metadata or {}
    print(f"pages: {len(reader.pages)}")
    for key in ("/Title", "/Author", "/Subject", "/Producer"):
        if meta.get(key):
            print(f"{key[1:].lower()}: {meta[key]}")
    box = reader.pages[0].mediabox if reader.pages else None
    if box:
        print(f"page size: {float(box.width):.0f} x {float(box.height):.0f} points")
    print(f"form fields: {len(reader.get_fields() or {})}")
    print(f"encrypted: {reader.is_encrypted}")


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)
    c = sub.add_parser("create")
    c.add_argument("source")
    c.add_argument("out")
    c.add_argument("--size", default="A4")
    c.set_defaults(fn=create)
    m = sub.add_parser("merge")
    m.add_argument("out")
    m.add_argument("inputs", nargs="+")
    m.set_defaults(fn=merge)
    g = sub.add_parser("pages")
    g.add_argument("pdf")
    g.add_argument("out")
    g.add_argument("spec")
    g.set_defaults(fn=pages)
    s = sub.add_parser("split")
    s.add_argument("pdf")
    s.add_argument("outdir")
    s.set_defaults(fn=split)
    t = sub.add_parser("text")
    t.add_argument("pdf")
    t.add_argument("--pages")
    t.set_defaults(fn=text)
    f = sub.add_parser("fields")
    f.add_argument("pdf")
    f.set_defaults(fn=fields)
    fl = sub.add_parser("fill")
    fl.add_argument("pdf")
    fl.add_argument("values")
    fl.add_argument("out")
    fl.set_defaults(fn=fill)
    i = sub.add_parser("info")
    i.add_argument("pdf")
    i.set_defaults(fn=info)
    args = p.parse_args()
    args.fn(args)


if __name__ == "__main__":
    main()

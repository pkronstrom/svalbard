"""Make, read and fill Word documents (.docx) with python-docx. Run with py-docs:

  py-docs docx_tool.py create IN.md OUT.docx
  py-docs docx_tool.py read IN.docx
  py-docs docx_tool.py fill TEMPLATE.docx VALUES.json OUT.docx

create reads simple markdown: "# Title", "## Heading", "### Subheading",
paragraphs separated by blank lines, "- item" bullets, "1. item" numbered
lists, "| a | b |" tables (the first row is the header, in bold),
"![caption](image.png)" images, **bold** and *italic*, and a line
"<<<pagebreak>>>" for a new page.

read prints the document in the same markdown, in order.

fill replaces each {{name}} in the template's paragraphs, tables, headers
and footers with the value of "name" in VALUES.json.
"""
import argparse
import json
import re
import sys
from pathlib import Path

import docx
from docx.enum.text import WD_BREAK
from docx.shared import Inches

PAGEBREAK = "<<<pagebreak>>>"
INLINE = re.compile(r"(\*\*.+?\*\*|(?<![\w*])\*(?!\s).+?(?<!\s)\*(?![\w*]))")


def add_runs(paragraph, text):
    for part in INLINE.split(text):
        if not part:
            continue
        if part.startswith("**") and part.endswith("**") and len(part) > 4:
            paragraph.add_run(part[2:-2]).bold = True
        elif part.startswith("*") and part.endswith("*") and len(part) > 2:
            paragraph.add_run(part[1:-1]).italic = True
        else:
            paragraph.add_run(part)


def table_cells(line):
    return [c.strip() for c in line.strip().strip("|").split("|")]


def create(args):
    source = Path(args.source)
    lines = source.read_text(encoding="utf-8").splitlines()
    d = docx.Document()
    para = []

    def flush():
        if para:
            add_runs(d.add_paragraph(), " ".join(para))
            para.clear()

    i = 0
    while i < len(lines):
        s = lines[i].strip()
        if not s:
            flush()
        elif s == PAGEBREAK:
            flush()
            d.add_paragraph().add_run().add_break(WD_BREAK.PAGE)
        elif m := re.match(r"(#{1,3})\s+(.*)", s):
            flush()
            d.add_heading(m.group(2), level=len(m.group(1)) - 1)
        elif m := re.match(r"!\[(.*?)\]\((.+?)\)", s):
            flush()
            path = Path(m.group(2))
            path = path if path.is_absolute() else source.parent / path
            if not path.exists():
                sys.exit(f"image {path} not found")
            d.add_picture(str(path), width=Inches(6))
            if m.group(1):
                d.add_paragraph(m.group(1), style="Caption")
        elif re.match(r"([-*]|\d+[.)])\s+", s):
            flush()
            while i < len(lines) and (m := re.match(r"\s*([-*]|\d+[.)])\s+(.*)", lines[i])):
                style = "List Bullet" if m.group(1) in "-*" else "List Number"
                add_runs(d.add_paragraph(style=style), m.group(2))
                i += 1
            continue
        elif s.startswith("|"):
            flush()
            rows = []
            while i < len(lines) and lines[i].strip().startswith("|"):
                cells = table_cells(lines[i])
                if not all(re.fullmatch(r":?-{2,}:?", c) for c in cells if c):
                    rows.append(cells)
                i += 1
            width = max(len(r) for r in rows)
            table = d.add_table(rows=len(rows), cols=width)
            table.style = "Table Grid"
            for r, row in enumerate(rows):
                for c, text in enumerate(row):
                    cell = table.cell(r, c)
                    add_runs(cell.paragraphs[0], text)
                    if r == 0:
                        for run in cell.paragraphs[0].runs:
                            run.bold = True
            continue
        else:
            para.append(s)
        i += 1
    flush()
    d.save(args.out)
    print(f"wrote {args.out}: {len(d.paragraphs)} paragraphs, {len(d.tables)} tables")


def paragraph_markdown(p, number):
    style = p.style.name if p.style is not None else ""
    text = p.text
    if style == "Title":
        return "# " + text
    if m := re.match(r"Heading (\d)", style):
        return "#" * (int(m.group(1)) + 1) + " " + text
    if style.startswith("List Bullet"):
        return "- " + text
    if style.startswith("List Number"):
        return f"{number}. " + text
    return text


def read(args):
    d = docx.Document(args.docx)
    previous_blank = True
    number = 0
    for block in d.iter_inner_content():
        is_numbered = not isinstance(block, docx.table.Table) and block.style is not None and block.style.name.startswith("List Number")
        number = number + 1 if is_numbered else 0
        if isinstance(block, docx.table.Table):
            for r, row in enumerate(block.rows):
                print("| " + " | ".join(c.text.strip() for c in row.cells) + " |")
                if r == 0:
                    print("|" + "---|" * len(row.cells))
            print()
            previous_blank = True
            continue
        line = paragraph_markdown(block, number)
        if not line.strip():
            if not previous_blank:
                print()
            previous_blank = True
            continue
        print(line)
        previous_blank = False


def replace_in_paragraph(p, values):
    if "{{" not in p.text:
        return 0
    text = p.text
    new = re.sub(r"\{\{\s*([^}]+?)\s*\}\}", lambda m: str(values.get(m.group(1), m.group(0))), text)
    if new == text:
        return 0
    # Placeholders may be split across runs: put the text in the first run
    # (keeping its formatting) and empty the rest.
    runs = p.runs
    runs[0].text = new
    for run in runs[1:]:
        run.text = ""
    return 1


def all_paragraphs(d):
    def walk(container):
        yield from container.paragraphs
        for table in container.tables:
            for row in table.rows:
                for cell in row.cells:
                    yield from walk(cell)
    yield from walk(d)
    for section in d.sections:
        for part in (section.header, section.footer):
            yield from walk(part)


def fill(args):
    values = json.loads(Path(args.values).read_text(encoding="utf-8"))
    d = docx.Document(args.template)
    changed = sum(replace_in_paragraph(p, values) for p in all_paragraphs(d))
    left = sorted({m for p in all_paragraphs(d) for m in re.findall(r"\{\{\s*([^}]+?)\s*\}\}", p.text)})
    d.save(args.out)
    print(f"wrote {args.out}: {changed} paragraphs changed" + (f"; no value for: {', '.join(left)}" if left else ""))


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)
    c = sub.add_parser("create")
    c.add_argument("source")
    c.add_argument("out")
    c.set_defaults(fn=create)
    r = sub.add_parser("read")
    r.add_argument("docx")
    r.set_defaults(fn=read)
    f = sub.add_parser("fill")
    f.add_argument("template")
    f.add_argument("values")
    f.add_argument("out")
    f.set_defaults(fn=fill)
    args = p.parse_args()
    args.fn(args)


if __name__ == "__main__":
    main()

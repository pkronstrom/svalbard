"""Validate a .docx: it opens, has at least one heading and one paragraph
of text. --heading S wants a heading S, --cell S a table cell S."""
import argparse
import sys

import docx


def main():
    p = argparse.ArgumentParser()
    p.add_argument("docx")
    p.add_argument("--heading")
    p.add_argument("--cell")
    a = p.parse_args()
    try:
        d = docx.Document(a.docx)
    except Exception as e:
        sys.exit(f"{a.docx} does not open as a Word document: {e}")
    headings = [x.text for x in d.paragraphs if x.style.name.startswith(("Heading", "Title")) and x.text.strip()]
    body = [x.text for x in d.paragraphs if not x.style.name.startswith(("Heading", "Title")) and x.text.strip()]
    cells = [c.text for t in d.tables for row in t.rows for c in row.cells]
    if not headings:
        sys.exit(f"{a.docx}: no heading")
    if not body and not cells:
        sys.exit(f"{a.docx}: no text besides headings")
    if a.heading and a.heading not in headings:
        sys.exit(f"{a.docx}: no heading {a.heading!r}; headings: {headings}")
    if a.cell and a.cell not in cells:
        sys.exit(f"{a.docx}: no table cell {a.cell!r}")
    print(f"{a.docx}: ok ({len(headings)} headings, {len(body)} paragraphs, {len(d.tables)} tables)")


if __name__ == "__main__":
    main()

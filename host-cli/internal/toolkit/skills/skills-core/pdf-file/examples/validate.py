"""Validate a PDF: it opens, has at least one page and some extractable
text. --pages N wants exactly N pages, --text S wants S in the text."""
import argparse
import sys

from pypdf import PdfReader


def main():
    p = argparse.ArgumentParser()
    p.add_argument("pdf")
    p.add_argument("--pages", type=int)
    p.add_argument("--text")
    a = p.parse_args()
    try:
        reader = PdfReader(a.pdf)
        pages = len(reader.pages)
        text = "\n".join(page.extract_text() or "" for page in reader.pages)
    except Exception as e:
        sys.exit(f"{a.pdf} does not open as a PDF: {e}")
    if pages == 0:
        sys.exit(f"{a.pdf}: no pages")
    if a.pages is not None and pages != a.pages:
        sys.exit(f"{a.pdf}: {pages} pages, want {a.pages}")
    if not text.strip():
        sys.exit(f"{a.pdf}: no extractable text (scanned or empty?)")
    if a.text and a.text not in text:
        sys.exit(f"{a.pdf}: text does not contain {a.text!r}")
    print(f"{a.pdf}: ok ({pages} pages, {len(text)} characters of text)")


if __name__ == "__main__":
    main()

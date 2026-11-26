#!/usr/bin/env py-docs
"""Make and read PowerPoint presentations (.pptx) with python-pptx. Run with
py-docs:

  py-docs pptx_tool.py create OUTLINE.json|OUTLINE.md OUT.pptx [--4x3]
  py-docs pptx_tool.py read IN.pptx

The JSON outline:
  {"title": "Deck title", "subtitle": "optional",
   "slides": [{"title": "Slide", "bullets": ["point", ["sub-point"]],
               "image": "picture.png", "notes": "speaker notes"}]}
A list inside "bullets" holds the sub-points of the bullet before it.

The markdown outline: "# Deck title", then an optional subtitle line; each
"## Slide title" starts a slide; "- point" and "  - sub-point" are bullets;
"![](picture.png)" adds a picture; a line "Notes: ..." adds speaker notes.

Images are found relative to the outline. A slide with bullets and an image
puts the image on the right. Slides are 16:9 unless --4x3 is given.
"""
import argparse
import json
import re
import sys
from pathlib import Path

try:
    from pptx import Presentation
    from pptx.util import Emu, Inches, Pt
except ImportError:
    sys.exit("This script needs the drive's document libraries: run it as py-docs " + sys.argv[0] + " ...")

TITLE_LAYOUT, CONTENT_LAYOUT, TITLE_ONLY_LAYOUT = 0, 1, 5


def parse_markdown(text):
    deck = {"title": "", "subtitle": "", "slides": []}
    slide = None
    for line in text.splitlines():
        s = line.strip()
        if not s:
            continue
        if m := re.match(r"#\s+(.*)", s):
            deck["title"] = m.group(1)
        elif m := re.match(r"##\s+(.*)", s):
            slide = {"title": m.group(1), "bullets": []}
            deck["slides"].append(slide)
        elif slide is None:
            deck["subtitle"] = (deck["subtitle"] + " " + s).strip()
        elif m := re.match(r"!\[.*?\]\((.+?)\)", s):
            slide["image"] = m.group(1)
        elif m := re.match(r"(?i)notes:\s*(.*)", s):
            slide["notes"] = m.group(1)
        elif m := re.match(r"(\s*)[-*]\s+(.*)", line):
            if len(m.group(1).expandtabs(4)) >= 2 and slide["bullets"]:
                if not isinstance(slide["bullets"][-1], list):
                    slide["bullets"].append([])
                slide["bullets"][-1].append(m.group(2))
            else:
                slide["bullets"].append(m.group(2))
        else:
            slide["bullets"].append(s)
    return deck


def flatten(bullets):
    """Yield (level, text) from the outline's bullets."""
    for item in bullets or []:
        if isinstance(item, list):
            for sub in item:
                yield 1, str(sub)
        else:
            yield 0, str(item)


def widen(slide, factor):
    """Stretch a 4:3 layout's placeholders to the wider slide."""
    for shape in slide.placeholders:
        # Read the inherited position first: setting one coordinate on a
        # placeholder zeroes the others.
        left, top, width, height = shape.left, shape.top, shape.width, shape.height
        shape.left, shape.top = Emu(int(left * factor)), top
        shape.width, shape.height = Emu(int(width * factor)), height


def add_picture(slide, path, left, top, max_width, max_height):
    picture = slide.shapes.add_picture(str(path), left, top)
    scale = min(max_width / picture.width, max_height / picture.height, 1.0)
    picture.width, picture.height = int(picture.width * scale), int(picture.height * scale)
    picture.left = int(left + (max_width - picture.width) / 2)
    picture.top = int(top + (max_height - picture.height) / 2)


def create(args):
    source = Path(args.outline)
    text = source.read_text(encoding="utf-8")
    deck = json.loads(text) if source.suffix.lower() == ".json" else parse_markdown(text)
    prs = Presentation()
    factor = 1.0
    if not args.four_three:
        factor = 13.333 / 10
        prs.slide_width, prs.slide_height = Inches(13.333), Inches(7.5)
    width, height = prs.slide_width, prs.slide_height

    if deck.get("title"):
        slide = prs.slides.add_slide(prs.slide_layouts[TITLE_LAYOUT])
        widen(slide, factor)
        slide.shapes.title.text = deck["title"]
        subtitle = slide.placeholders[1]
        if deck.get("subtitle"):
            subtitle.text = deck["subtitle"]
        else:
            subtitle._element.getparent().remove(subtitle._element)

    for spec in deck.get("slides", []):
        bullets = list(flatten(spec.get("bullets")))
        image = spec.get("image")
        if image:
            image = Path(image) if Path(image).is_absolute() else source.parent / image
            if not image.exists():
                sys.exit(f"image {image} not found")
        layout = CONTENT_LAYOUT if bullets else TITLE_ONLY_LAYOUT
        slide = prs.slides.add_slide(prs.slide_layouts[layout])
        widen(slide, factor)
        slide.shapes.title.text = spec.get("title", "")
        top = Inches(1.6)
        if bullets:
            body = slide.placeholders[1]
            if image:
                body.width = int((width - 2 * body.left) * 0.55)
            frame = body.text_frame
            for i, (level, line) in enumerate(bullets):
                para = frame.paragraphs[0] if i == 0 else frame.add_paragraph()
                para.text = line
                para.level = level
            if len(bullets) > 7:
                for para in frame.paragraphs:
                    for run in para.runs:
                        run.font.size = Pt(18)
            top = body.top
        if image:
            margin = Inches(0.5)
            if bullets:
                left = body.left + body.width + margin
                add_picture(slide, image, left, top, width - left - margin, height - top - margin)
            else:
                add_picture(slide, image, margin, top, width - 2 * margin, height - top - margin)
        if spec.get("notes"):
            slide.notes_slide.notes_text_frame.text = spec["notes"]
    prs.save(args.out)
    print(f"wrote {args.out}: {len(prs.slides)} slides")


def read(args):
    prs = Presentation(args.pptx)
    for number, slide in enumerate(prs.slides, start=1):
        title_shape = slide.shapes.title
        title = title_shape.text_frame.text if title_shape is not None else ""
        print(f"## {number}. {title}")
        for shape in slide.shapes:
            if shape == title_shape:
                continue
            if shape.shape_type == 13:  # picture
                print(f"[picture {shape.width / 914400:.1f} x {shape.height / 914400:.1f} in]")
            elif shape.has_text_frame:
                for para in shape.text_frame.paragraphs:
                    if para.text.strip():
                        print("  " * para.level + "- " + para.text)
            elif shape.has_table:
                for row in shape.table.rows:
                    print("| " + " | ".join(c.text for c in row.cells) + " |")
        if slide.has_notes_slide and slide.notes_slide.notes_text_frame.text.strip():
            print("Notes: " + slide.notes_slide.notes_text_frame.text.strip())
        print()


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)
    c = sub.add_parser("create")
    c.add_argument("outline")
    c.add_argument("out")
    c.add_argument("--4x3", dest="four_three", action="store_true")
    c.set_defaults(fn=create)
    r = sub.add_parser("read")
    r.add_argument("pptx")
    r.set_defaults(fn=read)
    args = p.parse_args()
    args.fn(args)


if __name__ == "__main__":
    main()

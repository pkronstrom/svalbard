"""Validate a .pptx: it opens, has at least one slide, and the first slide
has a title. --slides N wants N slides, --title S the first slide titled S,
--pictures N at least N pictures."""
import argparse
import sys

from pptx import Presentation
from pptx.enum.shapes import MSO_SHAPE_TYPE


def main():
    p = argparse.ArgumentParser()
    p.add_argument("pptx")
    p.add_argument("--slides", type=int)
    p.add_argument("--title")
    p.add_argument("--pictures", type=int)
    a = p.parse_args()
    try:
        prs = Presentation(a.pptx)
    except Exception as e:
        sys.exit(f"{a.pptx} does not open as a presentation: {e}")
    slides = list(prs.slides)
    if not slides:
        sys.exit(f"{a.pptx}: no slides")
    first = slides[0].shapes.title
    title = first.text_frame.text.strip() if first is not None else ""
    if not title:
        sys.exit(f"{a.pptx}: the first slide has no title")
    if a.slides is not None and len(slides) != a.slides:
        sys.exit(f"{a.pptx}: {len(slides)} slides, want {a.slides}")
    if a.title and title != a.title:
        sys.exit(f"{a.pptx}: first title {title!r}, want {a.title!r}")
    for number, slide in enumerate(slides, start=1):
        for shape in slide.shapes:
            if shape.width is not None and (shape.width <= 0 or shape.height <= 0):
                sys.exit(f"{a.pptx}: slide {number} shape {shape.name!r} has no size")
            if shape.left is not None and (shape.left + shape.width > prs.slide_width * 1.01 or shape.top + shape.height > prs.slide_height * 1.01):
                sys.exit(f"{a.pptx}: slide {number} shape {shape.name!r} runs off the slide")
    pictures = sum(1 for s in slides for sh in s.shapes if sh.shape_type == MSO_SHAPE_TYPE.PICTURE)
    if a.pictures is not None and pictures < a.pictures:
        sys.exit(f"{a.pptx}: {pictures} pictures, want at least {a.pictures}")
    print(f"{a.pptx}: ok ({len(slides)} slides, first title {title!r}, {pictures} pictures)")


if __name__ == "__main__":
    main()

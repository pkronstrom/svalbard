"""Write the ZIM and EPUB fixtures of this directory:

    uv run --with 'libzim>=3,<4' python make_fixture.py .

book.epub      "Fixture Book": a cover with only an image, chapter 1 "Beginnings" (short) and
               chapter 2 "The Long Road" (about 49,000 characters of numbered lines L0001 ...,
               four parts of at most 16,000 characters).
drm.epub       a valid EPUB whose META-INF/encryption.xml encrypts its chapter, as DRM does.
documents.zim  "Fixture Library", main page home, laid out like an openZIM nautilus ZIM:
                 home                    text/html   the rendered page
                 home.html               text/html   an unrendered template, <title>{{ title }}</title>
                 about                   text/html   an ordinary article
                 Node.js                 text/html   an article whose path ends like a script
                 Escaping_braces         text/html   an article whose title holds "{{"
                 vendors/x.css.map       text/plain  a source map (junk)
                 display_rows.handlebars text/plain  a template (junk)
                 vendors/app.js          application/javascript (junk)
                 database.js             text/javascript, the nautilus catalogue: guide.pdf is
                                         'Part 2' / 'Fruit and Fruit Products', water.pdf is
                                         'Water purification' / 'Quick Guides'
                 files/guide.pdf         ../../pdfext/testdata/text.pdf
                 files/water.pdf         ../../pdfext/testdata/text.pdf
                 files/scan (1).pdf      ../../pdfext/testdata/scan.pdf (a space and parentheses)
                 files/book.epub         book.epub
                 Novel.123 and Novel.123.epub, a Gutenberg-style HTML book with its EPUB copy
empty.zim      only vendors/app.js: nothing to index.
long-url.zim   an auxiliary JSON URL beyond gozim's directory-entry buffer,
               a readable article in the title index and an embedded PDF.
"""
import os
import sys
import zipfile

from libzim.writer import Creator, Hint, Item, StringProvider

HERE = os.path.dirname(os.path.abspath(__file__))
PDFS = os.path.join(HERE, "..", "..", "pdfext", "testdata")


def epub(path, chapters, title, drm=False):
    """chapters: list of (file name, xhtml body)."""
    with zipfile.ZipFile(path, "w") as z:
        z.writestr(zipfile.ZipInfo("mimetype"), "application/epub+zip")
        z.writestr("META-INF/container.xml",
                   '<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">'
                   '<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>')
        if drm:
            z.writestr("META-INF/encryption.xml",
                       '<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"'
                       ' xmlns:enc="http://www.w3.org/2001/04/xmlenc#"><enc:EncryptedData><enc:CipherData>'
                       '<enc:CipherReference URI="OEBPS/text/ch1.xhtml"/></enc:CipherData></enc:EncryptedData></encryption>')
        manifest = "".join(f'<item id="i{n}" href="text/{name}" media-type="application/xhtml+xml"/>' for n, (name, _) in enumerate(chapters))
        spine = "".join(f'<itemref idref="i{n}"/>' for n in range(len(chapters)))
        z.writestr("OEBPS/content.opf",
                   '<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0">'
                   f'<metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>{title}</dc:title></metadata>'
                   f'<manifest>{manifest}</manifest><spine>{spine}</spine></package>')
        for name, body in chapters:
            z.writestr(f"OEBPS/text/{name}",
                       f'<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>{title}</title></head><body>{body}</body></html>')


def long_road():
    lines = [f"<p>L{n:04d} the road goes ever on and on.</p>" for n in range(1, 1300)]
    return "<h1>The Long Road</h1>" + "".join(lines)


class Entry(Item):
    def __init__(self, path, mime, content, title=""):
        super().__init__()
        self.path, self.mime, self.content, self.title = path, mime, content, title

    def get_path(self):
        return self.path

    def get_title(self):
        return self.title or self.path

    def get_mimetype(self):
        return self.mime

    def get_contentprovider(self):
        return StringProvider(self.content)

    def get_hints(self):
        return {Hint.FRONT_ARTICLE: self.mime == "text/html"}


def zim(path, title, main, entries):
    if os.path.exists(path):
        os.remove(path)
    with Creator(path).config_indexing(False, "eng") as creator:
        if main:
            creator.set_mainpath(main)
        creator.add_metadata("Title", title)
        creator.add_metadata("Language", "eng")
        for entry in entries:
            creator.add_item(entry)


def read(path):
    with open(path, "rb") as f:
        return f.read()


def main(out):
    book = os.path.join(out, "book.epub")
    epub(book, [
        ("cover.xhtml", '<img src="cover.jpg" alt=""/>'),
        ("ch1.xhtml", "<h1>Beginnings</h1><p>It began with a kettle on the stove.</p>"),
        ("ch2.xhtml", long_road()),
    ], "Fixture Book")
    epub(os.path.join(out, "drm.epub"), [("ch1.xhtml", "<h1>Locked</h1><p>Secret text.</p>")], "Locked Book", drm=True)

    page = lambda t, body: f"<html><head><title>{t}</title></head><body>{body}</body></html>"
    catalogue = ("var DATABASE = [\n"
                 "{'_id': '00000', 'ti': 'Part 2', 'dsc': 'Fruit and Fruit Products', 'aut': '', 'fp': ['guide.pdf']},\n"
                 "{'_id': '00001', 'ti': 'Water purification', 'dsc': 'Quick Guides', 'aut': 'Various', 'fp': ['water.pdf']},\n"
                 "];\n")
    zim(os.path.join(out, "documents.zim"), "Fixture Library", "home", [
        Entry("home", "text/html", page("Fixture Library", "<p>A library of guides.</p>"), "Fixture Library"),
        Entry("home.html", "text/html", page("{{ title }}", "<p>{{ description }}</p>"), "home.html"),
        Entry("about", "text/html", page("About", "<p>About the guides and their authors.</p>"), "About"),
        Entry("Node.js", "text/html", page("Node.js", "<p>Node.js is a JavaScript runtime.</p>"), "Node.js"),
        Entry("Escaping_braces", "text/html", page("How to escape {{ in Jinja", "<p>Use a raw block to write literal braces.</p>"), "How to escape {{ in Jinja"),
        Entry("vendors/x.css.map", "text/plain", '{"version":3,"sources":["x.scss"]}'),
        Entry("display_rows.handlebars", "text/plain", "{{#each rows}}<li>{{title}}</li>{{/each}}"),
        Entry("vendors/app.js", "application/javascript", "console.log('app');"),
        Entry("database.js", "text/javascript", catalogue),
        Entry("files/guide.pdf", "application/pdf", read(os.path.join(PDFS, "text.pdf"))),
        Entry("files/water.pdf", "application/pdf", read(os.path.join(PDFS, "text.pdf"))),
        Entry("files/scan (1).pdf", "application/pdf", read(os.path.join(PDFS, "scan.pdf"))),
        Entry("files/book.epub", "application/epub+zip", read(book)),
        Entry("Novel.123", "text/html", page("Novel", "<p>Call me Fixture.</p>"), "Novel"),
        Entry("Novel.123.epub", "application/epub+zip", read(book)),
    ])
    zim(os.path.join(out, "empty.zim"), "Empty", None, [
        Entry("vendors/app.js", "application/javascript", "console.log('app');"),
    ])
    zim(os.path.join(out, "long-url.zim"), "Long URL", "home", [
        Entry("api/data.json?query=" + "x" * 4100, "application/json", "{}", "Data"),
        Entry("guide.pdf", "application/pdf", read(os.path.join(PDFS, "text.pdf")), "Guide"),
        Entry("home", "text/html", page("Long URL", "<p>A page that remains searchable.</p>"), "Long URL"),
    ])


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else ".")

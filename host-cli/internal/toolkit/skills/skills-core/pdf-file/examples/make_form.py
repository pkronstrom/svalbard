"""Make a one-page PDF form with two text fields, name and litres, to
test filling."""
import sys

from reportlab.lib.pagesizes import A4
from reportlab.pdfgen import canvas

c = canvas.Canvas(sys.argv[1], pagesize=A4)
c.drawString(72, 760, "Water reading")
c.drawString(72, 720, "Name:")
c.acroForm.textfield(name="name", x=140, y=712, width=240, height=20)
c.drawString(72, 680, "Litres:")
c.acroForm.textfield(name="litres", x=140, y=672, width=120, height=20)
c.save()

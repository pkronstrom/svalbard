# pdf-lib 1.17.1 reference

This is the project's own documentation, joined at the version on this drive. Here nothing is installed or downloaded: import the package by name in a module script and pack the page with `svalbard-drive html pack`. Code blocks marked `not-for-this-stick` and lines starting "Not for this drive:" show package managers, CDN links or downloads; this drive does not use them.

## README

<a href="https://pdf-lib.js.org">
<h1 align="center">
Not for this drive: <img alt="pdf-lib" height="300" src="https://raw.githubusercontent.com/Hopding/pdf-lib-docs/master/assets/logo-full.svg?sanitize=true">
</h1>
</a>

<div align="center">
  <strong>Create and modify PDF documents in any JavaScript environment.</strong>
</div>
<div align="center">
  Designed to work in any modern JavaScript runtime. Tested in Node, Browser, Deno, and React Native environments.
</div>

<br />

<div align="center">
  
  <a href="https://www.npmjs.com/package/pdf-lib">
    <img
Not for this drive:       src="https://img.shields.io/npm/v/pdf-lib.svg?style=flat-square"
      alt="NPM Version"
    />
  </a>
  
  <a href="https://circleci.com/gh/Hopding/pdf-lib">
    <img
Not for this drive:       src="https://img.shields.io/circleci/project/github/Hopding/pdf-lib/master.svg?style=flat-square&label=CircleCI"
      alt="CircleCI Build Status"
    />
  </a>
  
  <a href="https://prettier.io/">
    <img
Not for this drive:       src="https://img.shields.io/badge/code_style-prettier-ff69b4.svg?style=flat-square"
      alt="Prettier Badge"
    />
  </a>
  
  <a href="https://discord.gg/Y7uuVMc">
    <img
Not for this drive:       src="https://img.shields.io/static/v1?label=discord&message=pdf-lib&color=566fbb&style=flat-square"
      alt="Discord Badge"
    />
  </a>
</div>

<br />

> **Learn more at [pdf-lib.js.org](https://pdf-lib.js.org)**

## README: Table of Contents

- [Features](#features)
- [Motivation](#motivation)
- [Usage Examples](#usage-examples)
  - [Create Document](#create-document)
  - [Modify Document](#modify-document)
  - [Create Form](#create-form)
  - [Fill Form](#fill-form)
  - [Flatten Form](#flatten-form)
  - [Copy Pages](#copy-pages)
  - [Embed PNG and JPEG Images](#embed-png-and-jpeg-images)
  - [Embed PDF Pages](#embed-pdf-pages)
  - [Embed Font and Measure Text](#embed-font-and-measure-text)
  - [Add Attachments](#add-attachments)
  - [Set Document Metadata](#set-document-metadata)
  - [Read Document Metadata](#read-document-metadata)
  - [Set Viewer Preferences](#set-viewer-preferences)
  - [Read Viewer Preferences](#read-viewer-preferences)
  - [Draw SVG Paths](#draw-svg-paths)
- [Deno Usage](#deno-usage)
- [Complete Examples](#complete-examples)
- [Installation](#installation)
- [Documentation](#documentation)
- [Fonts and Unicode](#fonts-and-unicode)
- [Creating and Filling Forms](#creating-and-filling-forms)
- [Limitations](#limitations)
- [Help and Discussion](#help-and-discussion)
- [Encryption Handling](#encryption-handling)
- [Migrating to v1.0.0](docs/MIGRATION.md)
- [Contributing](#contributing)
- [Maintainership](#maintainership)
- [Tutorials and Cool Stuff](#tutorials-and-cool-stuff)
- [Prior Art](#prior-art)
- [Git History Rewrite](#git-history-rewrite)
- [License](#license)

## README: Features

- Create new PDFs
- Modify existing PDFs
- Create forms
- Fill forms
- Flatten forms
- Add Pages
- Insert Pages
- Remove Pages
- Copy pages between PDFs
- Draw Text
- Draw Images
- Draw PDF Pages
- Draw Vector Graphics
- Draw SVG Paths
- Measure width and height of text
- Embed Fonts (supports UTF-8 and UTF-16 character sets)
- Set document metadata
- Read document metadata
- Set viewer preferences
- Read viewer preferences
- Add attachments

## README: Motivation

`pdf-lib` was created to address the JavaScript ecosystem's lack of robust support for PDF manipulation (especially for PDF _modification_).

Two of `pdf-lib`'s distinguishing features are:

1. Supporting modification (editing) of existing documents.
2. Working in all JavaScript environments - not just in Node or the Browser.

There are [other](#prior-art) good open source JavaScript PDF libraries available. However, most of them can only _create_ documents, they cannot _modify_ existing ones. And many of them only work in particular environments.

## README: Usage Examples

### Create Document

_This example produces [this PDF](assets/pdfs/examples/create_document.pdf)._

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/rxwsc8f5/13/)


```js
import { PDFDocument, StandardFonts, rgb } from 'pdf-lib'

// Create a new PDFDocument
const pdfDoc = await PDFDocument.create()

// Embed the Times Roman font
const timesRomanFont = await pdfDoc.embedFont(StandardFonts.TimesRoman)

// Add a blank page to the document
const page = pdfDoc.addPage()

// Get the width and height of the page
const { width, height } = page.getSize()

// Draw a string of text toward the top of the page
const fontSize = 30
page.drawText('Creating PDFs in JavaScript is awesome!', {
  x: 50,
  y: height - 4 * fontSize,
  size: fontSize,
  font: timesRomanFont,
  color: rgb(0, 0.53, 0.71),
})

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Modify Document

_This example produces [this PDF](assets/pdfs/examples/modify_document.pdf)_ (when [this PDF](assets/pdfs/with_update_sections.pdf) is used for the `existingPdfBytes` variable).

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/64zajhge/1/)


```js
import { degrees, PDFDocument, rgb, StandardFonts } from 'pdf-lib';

// This should be a Uint8Array or ArrayBuffer
// This data can be obtained in a number of different ways
// If your running in a Node environment, you could use fs.readFile()
// In the browser, you could make a fetch() call and use res.arrayBuffer()
const existingPdfBytes = ...

// Load a PDFDocument from the existing PDF bytes
const pdfDoc = await PDFDocument.load(existingPdfBytes)

// Embed the Helvetica font
const helveticaFont = await pdfDoc.embedFont(StandardFonts.Helvetica)

// Get the first page of the document
const pages = pdfDoc.getPages()
const firstPage = pages[0]

// Get the width and height of the first page
const { width, height } = firstPage.getSize()

// Draw a string of text diagonally across the first page
firstPage.drawText('This text was added with JavaScript!', {
  x: 5,
  y: height / 2 + 300,
  size: 50,
  font: helveticaFont,
  color: rgb(0.95, 0.1, 0.1),
  rotate: degrees(-45),
})


// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Create Form

_This example produces [this PDF](assets/pdfs/examples/create_form.pdf)._

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/bct7vngL/4/)

> See also [Creating and Filling Forms](#creating-and-filling-forms)


```js
import { PDFDocument } from 'pdf-lib'

// Create a new PDFDocument
const pdfDoc = await PDFDocument.create()

// Add a blank page to the document
const page = pdfDoc.addPage([550, 750])

// Get the form so we can add fields to it
const form = pdfDoc.getForm()

// Add the superhero text field and description
page.drawText('Enter your favorite superhero:', { x: 50, y: 700, size: 20 })

const superheroField = form.createTextField('favorite.superhero')
superheroField.setText('One Punch Man')
superheroField.addToPage(page, { x: 55, y: 640 })

// Add the rocket radio group, labels, and description
page.drawText('Select your favorite rocket:', { x: 50, y: 600, size: 20 })

page.drawText('Falcon Heavy', { x: 120, y: 560, size: 18 })
page.drawText('Saturn IV', { x: 120, y: 500, size: 18 })
page.drawText('Delta IV Heavy', { x: 340, y: 560, size: 18 })
page.drawText('Space Launch System', { x: 340, y: 500, size: 18 })

const rocketField = form.createRadioGroup('favorite.rocket')
rocketField.addOptionToPage('Falcon Heavy', page, { x: 55, y: 540 })
rocketField.addOptionToPage('Saturn IV', page, { x: 55, y: 480 })
rocketField.addOptionToPage('Delta IV Heavy', page, { x: 275, y: 540 })
rocketField.addOptionToPage('Space Launch System', page, { x: 275, y: 480 })
rocketField.select('Saturn IV')

// Add the gundam check boxes, labels, and description
page.drawText('Select your favorite gundams:', { x: 50, y: 440, size: 20 })

page.drawText('Exia', { x: 120, y: 400, size: 18 })
page.drawText('Kyrios', { x: 120, y: 340, size: 18 })
page.drawText('Virtue', { x: 340, y: 400, size: 18 })
page.drawText('Dynames', { x: 340, y: 340, size: 18 })

const exiaField = form.createCheckBox('gundam.exia')
const kyriosField = form.createCheckBox('gundam.kyrios')
const virtueField = form.createCheckBox('gundam.virtue')
const dynamesField = form.createCheckBox('gundam.dynames')

exiaField.addToPage(page, { x: 55, y: 380 })
kyriosField.addToPage(page, { x: 55, y: 320 })
virtueField.addToPage(page, { x: 275, y: 380 })
dynamesField.addToPage(page, { x: 275, y: 320 })

exiaField.check()
dynamesField.check()

// Add the planet dropdown and description
page.drawText('Select your favorite planet*:', { x: 50, y: 280, size: 20 })

const planetsField = form.createDropdown('favorite.planet')
planetsField.addOptions(['Venus', 'Earth', 'Mars', 'Pluto'])
planetsField.select('Pluto')
planetsField.addToPage(page, { x: 55, y: 220 })

// Add the person option list and description
page.drawText('Select your favorite person:', { x: 50, y: 180, size: 18 })

const personField = form.createOptionList('favorite.person')
personField.addOptions([
  'Julius Caesar',
  'Ada Lovelace',
  'Cleopatra',
  'Aaron Burr',
  'Mark Antony',
])
personField.select('Ada Lovelace')
personField.addToPage(page, { x: 55, y: 70 })

// Just saying...
page.drawText(`* Pluto should be a planet too!`, { x: 15, y: 15, size: 15 })

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Fill Form

_This example produces [this PDF](assets/pdfs/examples/fill_form.pdf)_ (when [this PDF](assets/pdfs/dod_character.pdf) is used for the `formPdfBytes` variable, [this image](assets/images/small_mario.png) is used for the `marioImageBytes` variable, and [this image](assets/images/mario_emblem.png) is used for the `emblemImageBytes` variable).

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/0mwfqkv6/3/)

> See also [Creating and Filling Forms](#creating-and-filling-forms)


```js
import { PDFDocument } from 'pdf-lib'

// These should be Uint8Arrays or ArrayBuffers
// This data can be obtained in a number of different ways
// If your running in a Node environment, you could use fs.readFile()
// In the browser, you could make a fetch() call and use res.arrayBuffer()
const formPdfBytes = ...
const marioImageBytes = ...
const emblemImageBytes = ...

// Load a PDF with form fields
const pdfDoc = await PDFDocument.load(formPdfBytes)

// Embed the Mario and emblem images
const marioImage = await pdfDoc.embedPng(marioImageBytes)
const emblemImage = await pdfDoc.embedPng(emblemImageBytes)

// Get the form containing all the fields
const form = pdfDoc.getForm()

// Get all fields in the PDF by their names
const nameField = form.getTextField('CharacterName 2')
const ageField = form.getTextField('Age')
const heightField = form.getTextField('Height')
const weightField = form.getTextField('Weight')
const eyesField = form.getTextField('Eyes')
const skinField = form.getTextField('Skin')
const hairField = form.getTextField('Hair')

const alliesField = form.getTextField('Allies')
const factionField = form.getTextField('FactionName')
const backstoryField = form.getTextField('Backstory')
const traitsField = form.getTextField('Feat+Traits')
const treasureField = form.getTextField('Treasure')

const characterImageField = form.getButton('CHARACTER IMAGE')
const factionImageField = form.getTextField('Faction Symbol Image')

// Fill in the basic info fields
nameField.setText('Mario')
ageField.setText('24 years')
heightField.setText(`5' 1"`)
weightField.setText('196 lbs')
eyesField.setText('blue')
skinField.setText('white')
hairField.setText('brown')

// Fill the character image field with our Mario image
characterImageField.setImage(marioImage)

// Fill in the allies field
alliesField.setText(
  [
    `Allies:`,
    `  • Princess Daisy`,
    `  • Princess Peach`,
    `  • Rosalina`,
    `  • Geno`,
    `  • Luigi`,
    `  • Donkey Kong`,
    `  • Yoshi`,
    `  • Diddy Kong`,
    ``,
    `Organizations:`,
    `  • Italian Plumbers Association`,
  ].join('\n'),
)

// Fill in the faction name field
factionField.setText(`Mario's Emblem`)

// Fill the faction image field with our emblem image
factionImageField.setImage(emblemImage)

// Fill in the backstory field
backstoryField.setText(
  `Mario is a fictional character in the Mario video game franchise, owned by Nintendo and created by Japanese video game designer Shigeru Miyamoto. Serving as the company's mascot and the eponymous protagonist of the series, Mario has appeared in over 200 video games since his creation. Depicted as a short, pudgy, Italian plumber who resides in the Mushroom Kingdom, his adventures generally center upon rescuing Princess Peach from the Koopa villain Bowser. His younger brother and sidekick is Luigi.`,
)

// Fill in the traits field
traitsField.setText(
  [
    `Mario can use three basic three power-ups:`,
    `  • the Super Mushroom, which causes Mario to grow larger`,
    `  • the Fire Flower, which allows Mario to throw fireballs`,
    `  • the Starman, which gives Mario temporary invincibility`,
  ].join('\n'),
)

// Fill in the treasure field
treasureField.setText(['• Gold coins', '• Treasure chests'].join('\n'))

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Flatten Form

_This example produces [this PDF](assets/pdfs/examples/flatten_form.pdf)_ (when [this PDF](assets/pdfs/form_to_flatten.pdf) is used for the `formPdfBytes` variable).

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/skevywdz/2/)


```js
import { PDFDocument } from 'pdf-lib'

// This should be a Uint8Array or ArrayBuffer
// This data can be obtained in a number of different ways
// If your running in a Node environment, you could use fs.readFile()
// In the browser, you could make a fetch() call and use res.arrayBuffer()
const formPdfBytes = ...

// Load a PDF with form fields
const pdfDoc = await PDFDocument.load(formPdfBytes)

// Get the form containing all the fields
const form = pdfDoc.getForm()

// Fill the form's fields
form.getTextField('Text1').setText('Some Text');

form.getRadioGroup('Group2').select('Choice1');
form.getRadioGroup('Group3').select('Choice3');
form.getRadioGroup('Group4').select('Choice1');

form.getCheckBox('Check Box3').check();
form.getCheckBox('Check Box4').uncheck();

form.getDropdown('Dropdown7').select('Infinity');

form.getOptionList('List Box6').select('Honda');

// Flatten the form's fields
form.flatten();

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Copy Pages

_This example produces [this PDF](assets/pdfs/examples/copy_pages.pdf)_ (when [this PDF](assets/pdfs/with_update_sections.pdf) is used for the `firstDonorPdfBytes` variable and [this PDF](assets/pdfs/with_large_page_count.pdf) is used for the `secondDonorPdfBytes` variable).

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/ybank8s9/2/)


```js
import { PDFDocument } from 'pdf-lib'

// Create a new PDFDocument
const pdfDoc = await PDFDocument.create()

// These should be Uint8Arrays or ArrayBuffers
// This data can be obtained in a number of different ways
// If your running in a Node environment, you could use fs.readFile()
// In the browser, you could make a fetch() call and use res.arrayBuffer()
const firstDonorPdfBytes = ...
const secondDonorPdfBytes = ...

// Load a PDFDocument from each of the existing PDFs
const firstDonorPdfDoc = await PDFDocument.load(firstDonorPdfBytes)
const secondDonorPdfDoc = await PDFDocument.load(secondDonorPdfBytes)

// Copy the 1st page from the first donor document, and
// the 743rd page from the second donor document
const [firstDonorPage] = await pdfDoc.copyPages(firstDonorPdfDoc, [0])
const [secondDonorPage] = await pdfDoc.copyPages(secondDonorPdfDoc, [742])

// Add the first copied page
pdfDoc.addPage(firstDonorPage)

// Insert the second copied page to index 0, so it will be the
// first page in `pdfDoc`
pdfDoc.insertPage(0, secondDonorPage)

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Embed PNG and JPEG Images

_This example produces [this PDF](assets/pdfs/examples/embed_png_and_jpeg_images.pdf)_ (when [this image](assets/images/cat_riding_unicorn.jpg) is used for the `jpgImageBytes` variable and [this image](assets/images/minions_banana_alpha.png) is used for the `pngImageBytes` variable).

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/bcya43ju/5/)


```js
import { PDFDocument } from 'pdf-lib'

// These should be Uint8Arrays or ArrayBuffers
// This data can be obtained in a number of different ways
// If your running in a Node environment, you could use fs.readFile()
// In the browser, you could make a fetch() call and use res.arrayBuffer()
const jpgImageBytes = ...
const pngImageBytes = ...

// Create a new PDFDocument
const pdfDoc = await PDFDocument.create()

// Embed the JPG image bytes and PNG image bytes
const jpgImage = await pdfDoc.embedJpg(jpgImageBytes)
const pngImage = await pdfDoc.embedPng(pngImageBytes)

// Get the width/height of the JPG image scaled down to 25% of its original size
const jpgDims = jpgImage.scale(0.25)

// Get the width/height of the PNG image scaled down to 50% of its original size
const pngDims = pngImage.scale(0.5)

// Add a blank page to the document
const page = pdfDoc.addPage()

// Draw the JPG image in the center of the page
page.drawImage(jpgImage, {
  x: page.getWidth() / 2 - jpgDims.width / 2,
  y: page.getHeight() / 2 - jpgDims.height / 2,
  width: jpgDims.width,
  height: jpgDims.height,
})

// Draw the PNG image near the lower right corner of the JPG image
page.drawImage(pngImage, {
  x: page.getWidth() / 2 - pngDims.width / 2 + 75,
  y: page.getHeight() / 2 - pngDims.height,
  width: pngDims.width,
  height: pngDims.height,
})

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Embed PDF Pages

_This example produces [this PDF](assets/pdfs/examples/embed_pdf_pages.pdf)_ (when [this PDF](assets/pdfs/american_flag.pdf) is used for the `americanFlagPdfBytes` variable and [this PDF](assets/pdfs/us_constitution.pdf) is used for the `usConstitutionPdfBytes` variable).

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/Lyb16ocj/13/)


```js
import { PDFDocument } from 'pdf-lib'

// These should be Uint8Arrays or ArrayBuffers
// This data can be obtained in a number of different ways
// If your running in a Node environment, you could use fs.readFile()
// In the browser, you could make a fetch() call and use res.arrayBuffer()
const americanFlagPdfBytes = ...
const usConstitutionPdfBytes = ...

// Create a new PDFDocument
const pdfDoc = await PDFDocument.create()

// Embed the American flag PDF bytes
const [americanFlag] = await pdfDoc.embedPdf(americanFlagPdfBytes)

// Load the U.S. constitution PDF bytes
const usConstitutionPdf = await PDFDocument.load(usConstitutionPdfBytes)

// Embed the second page of the constitution and clip the preamble
const preamble = await pdfDoc.embedPage(usConstitutionPdf.getPages()[1], {
  left: 55,
  bottom: 485,
  right: 300,
  top: 575,
})

// Get the width/height of the American flag PDF scaled down to 30% of
// its original size
const americanFlagDims = americanFlag.scale(0.3)

// Get the width/height of the preamble clipping scaled up to 225% of
// its original size
const preambleDims = preamble.scale(2.25)

// Add a blank page to the document
const page = pdfDoc.addPage()

// Draw the American flag image in the center top of the page
page.drawPage(americanFlag, {
  ...americanFlagDims,
  x: page.getWidth() / 2 - americanFlagDims.width / 2,
  y: page.getHeight() - americanFlagDims.height - 150,
})

// Draw the preamble clipping in the center bottom of the page
page.drawPage(preamble, {
  ...preambleDims,
  x: page.getWidth() / 2 - preambleDims.width / 2,
  y: page.getHeight() / 2 - preambleDims.height / 2 - 50,
})

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Embed Font and Measure Text

`pdf-lib` relies on a sister module to support embedding custom fonts: [`@pdf-lib/fontkit`](https://www.npmjs.com/package/@pdf-lib/fontkit). You must add the `@pdf-lib/fontkit` module to your project and register it using `pdfDoc.registerFontkit(...)` before embedding custom fonts.

> **[See below for detailed installation instructions on installing `@pdf-lib/fontkit` as a UMD or NPM module.](#fontkit-installation)**

_This example produces [this PDF](assets/pdfs/examples/embed_font_and_measure_text.pdf)_ (when [this font](assets/fonts/ubuntu/Ubuntu-R.ttf) is used for the `fontBytes` variable).

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/rgu6ca59/2/)


```js
import { PDFDocument, rgb } from 'pdf-lib'
import fontkit from '@pdf-lib/fontkit'

// This should be a Uint8Array or ArrayBuffer
// This data can be obtained in a number of different ways
// If you're running in a Node environment, you could use fs.readFile()
// In the browser, you could make a fetch() call and use res.arrayBuffer()
const fontBytes = ...

// Create a new PDFDocument
const pdfDoc = await PDFDocument.create()

// Register the `fontkit` instance
pdfDoc.registerFontkit(fontkit)

// Embed our custom font in the document
const customFont = await pdfDoc.embedFont(fontBytes)

// Add a blank page to the document
const page = pdfDoc.addPage()

// Create a string of text and measure its width and height in our custom font
const text = 'This is text in an embedded font!'
const textSize = 35
const textWidth = customFont.widthOfTextAtSize(text, textSize)
const textHeight = customFont.heightAtSize(textSize)

// Draw the string of text on the page
page.drawText(text, {
  x: 40,
  y: 450,
  size: textSize,
  font: customFont,
  color: rgb(0, 0.53, 0.71),
})

// Draw a box around the string of text
page.drawRectangle({
  x: 40,
  y: 450,
  width: textWidth,
  height: textHeight,
  borderColor: rgb(1, 0, 0),
  borderWidth: 1.5,
})

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Add Attachments

_This example produces [this PDF](assets/pdfs/examples/add_attachments.pdf)_ (when [this image](assets/images/cat_riding_unicorn.jpg) is used for the `jpgAttachmentBytes` variable and [this PDF](assets/pdfs/us_constitution.pdf) is used for the `pdfAttachmentBytes` variable).

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/9snL63wj/5/)


```js
import { PDFDocument } from 'pdf-lib'

// These should be Uint8Arrays or ArrayBuffers
// This data can be obtained in a number of different ways
// If your running in a Node environment, you could use fs.readFile()
// In the browser, you could make a fetch() call and use res.arrayBuffer()
const jpgAttachmentBytes = ...
const pdfAttachmentBytes = ...

// Create a new PDFDocument
const pdfDoc = await PDFDocument.create()

// Add the JPG attachment
await pdfDoc.attach(jpgAttachmentBytes, 'cat_riding_unicorn.jpg', {
  mimeType: 'image/jpeg',
  description: 'Cool cat riding a unicorn! 🦄🐈🕶️',
  creationDate: new Date('2019/12/01'),
  modificationDate: new Date('2020/04/19'),
})

// Add the PDF attachment
await pdfDoc.attach(pdfAttachmentBytes, 'us_constitution.pdf', {
  mimeType: 'application/pdf',
  description: 'Constitution of the United States 🇺🇸🦅',
  creationDate: new Date('1787/09/17'),
  modificationDate: new Date('1992/05/07'),
})

// Add a page with some text
const page = pdfDoc.addPage();
page.drawText('This PDF has two attachments', { x: 135, y: 415 })

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Set Document Metadata

_This example produces [this PDF](assets/pdfs/examples/set_document_metadata.pdf)_.

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/vcwmfnbe/2/)


```js
import { PDFDocument, StandardFonts } from 'pdf-lib'

// Create a new PDFDocument
const pdfDoc = await PDFDocument.create()

// Embed the Times Roman font
const timesRomanFont = await pdfDoc.embedFont(StandardFonts.TimesRoman)

// Add a page and draw some text on it
const page = pdfDoc.addPage([500, 600])
page.setFont(timesRomanFont)
page.drawText('The Life of an Egg', { x: 60, y: 500, size: 50 })
page.drawText('An Epic Tale of Woe', { x: 125, y: 460, size: 25 })

// Set all available metadata fields on the PDFDocument. Note that these fields
// are visible in the "Document Properties" section of most PDF readers.
pdfDoc.setTitle('🥚 The Life of an Egg 🍳')
pdfDoc.setAuthor('Humpty Dumpty')
pdfDoc.setSubject('📘 An Epic Tale of Woe 📖')
pdfDoc.setKeywords(['eggs', 'wall', 'fall', 'king', 'horses', 'men'])
pdfDoc.setProducer('PDF App 9000 🤖')
pdfDoc.setCreator('pdf-lib (https://github.com/Hopding/pdf-lib)')
pdfDoc.setCreationDate(new Date('2018-06-24T01:58:37.228Z'))
pdfDoc.setModificationDate(new Date('2019-12-21T07:00:11.000Z'))

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Read Document Metadata

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/eg8rfz3k/16/)


```js
import { PDFDocument } from 'pdf-lib'

// This should be a Uint8Array or ArrayBuffer
// This data can be obtained in a number of different ways
// If your running in a Node environment, you could use fs.readFile()
// In the browser, you could make a fetch() call and use res.arrayBuffer()
const existingPdfBytes = ...

// Load a PDFDocument without updating its existing metadata
const pdfDoc = await PDFDocument.load(existingPdfBytes, {
  updateMetadata: false
})

// Print all available metadata fields
console.log('Title:', pdfDoc.getTitle())
console.log('Author:', pdfDoc.getAuthor())
console.log('Subject:', pdfDoc.getSubject())
console.log('Creator:', pdfDoc.getCreator())
console.log('Keywords:', pdfDoc.getKeywords())
console.log('Producer:', pdfDoc.getProducer())
console.log('Creation Date:', pdfDoc.getCreationDate())
console.log('Modification Date:', pdfDoc.getModificationDate())
```

This script outputs the following (_when [this PDF](assets/pdfs/with_cropbox.pdf) is used for the `existingPdfBytes` variable_):

```
Title: Microsoft Word - Basic Curriculum Vitae example.doc
Author: Administrator
Subject: undefined
Creator: PScript5.dll Version 5.2
Keywords: undefined
Producer: Acrobat Distiller 8.1.0 (Windows)
Creation Date: 2010-07-29T14:26:00.000Z
Modification Date: 2010-07-29T14:26:00.000Z
```

### Set Viewer Preferences


```js
import {
  PDFDocument,
  StandardFonts,
  NonFullScreenPageMode,
  ReadingDirection,
  PrintScaling,
  Duplex,
  PDFName,
} from 'pdf-lib'

// Create a new PDFDocument
const pdfDoc = await PDFDocument.create()

// Embed the Times Roman font
const timesRomanFont = await pdfDoc.embedFont(StandardFonts.TimesRoman)

// Add a page and draw some text on it
const page = pdfDoc.addPage([500, 600])
page.setFont(timesRomanFont)
page.drawText('The Life of an Egg', { x: 60, y: 500, size: 50 })
page.drawText('An Epic Tale of Woe', { x: 125, y: 460, size: 25 })

// Set all available viewer preferences on the PDFDocument:
const viewerPrefs = pdfDoc.catalog.getOrCreateViewerPreferences()
viewerPrefs.setHideToolbar(true)
viewerPrefs.setHideMenubar(true)
viewerPrefs.setHideWindowUI(true)
viewerPrefs.setFitWindow(true)
viewerPrefs.setCenterWindow(true)
viewerPrefs.setDisplayDocTitle(true)

// Set the PageMode (otherwise setting NonFullScreenPageMode has no meaning)
pdfDoc.catalog.set(PDFName.of('PageMode'), PDFName.of('FullScreen'))

// Set what happens when fullScreen is closed
viewerPrefs.setNonFullScreenPageMode(NonFullScreenPageMode.UseOutlines)

viewerPrefs.setReadingDirection(ReadingDirection.L2R)
viewerPrefs.setPrintScaling(PrintScaling.None)
viewerPrefs.setDuplex(Duplex.DuplexFlipLongEdge)
viewerPrefs.setPickTrayByPDFSize(true)

// We can set the default print range to only the first page
viewerPrefs.setPrintPageRange({ start: 0, end: 0 })

// Or we can supply noncontiguous ranges (e.g. pages 1, 3, and 5-7)
viewerPrefs.setPrintPageRange([
  { start: 0, end: 0 },
  { start: 2, end: 2 },
  { start: 4, end: 6 },
])

viewerPrefs.setNumCopies(2)

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

### Read Viewer Preferences


```js
import { PDFDocument } from 'pdf-lib'

// This should be a Uint8Array or ArrayBuffer
// This data can be obtained in a number of different ways
// If your running in a Node environment, you could use fs.readFile()
// In the browser, you could make a fetch() call and use res.arrayBuffer()
const existingPdfBytes = ...

// Load a PDFDocument without updating its existing metadata
const pdfDoc = await PDFDocument.load(existingPdfBytes)
const viewerPrefs = pdfDoc.catalog.getOrCreateViewerPreferences()

// Print all available viewer preference fields
console.log('HideToolbar:', viewerPrefs.getHideToolbar())
console.log('HideMenubar:', viewerPrefs.getHideMenubar())
console.log('HideWindowUI:', viewerPrefs.getHideWindowUI())
console.log('FitWindow:', viewerPrefs.getFitWindow())
console.log('CenterWindow:', viewerPrefs.getCenterWindow())
console.log('DisplayDocTitle:', viewerPrefs.getDisplayDocTitle())
console.log('NonFullScreenPageMode:', viewerPrefs.getNonFullScreenPageMode())
console.log('ReadingDirection:', viewerPrefs.getReadingDirection())
console.log('PrintScaling:', viewerPrefs.getPrintScaling())
console.log('Duplex:', viewerPrefs.getDuplex())
console.log('PickTrayByPDFSize:', viewerPrefs.getPickTrayByPDFSize())
console.log('PrintPageRange:', viewerPrefs.getPrintPageRange())
console.log('NumCopies:', viewerPrefs.getNumCopies())
```

This script outputs the following (_when [this PDF](assets/pdfs/with_viewer_prefs.pdf) is used for the `existingPdfBytes` variable_):

```
HideToolbar: true
HideMenubar: true
HideWindowUI: false
FitWindow: true
CenterWindow: true
DisplayDocTitle: true
NonFullScreenPageMode: UseNone
ReadingDirection: R2L
PrintScaling: None
Duplex: DuplexFlipLongEdge
PickTrayByPDFSize: true
PrintPageRange: [ { start: 1, end: 1 }, { start: 3, end: 4 } ]
NumCopies: 2
```

### Draw SVG Paths

_This example produces [this PDF](assets/pdfs/examples/draw_svg_paths.pdf)_.

[Try the JSFiddle demo](https://jsfiddle.net/Hopding/bwaomr9h/2/)


```js
import { PDFDocument, rgb } from 'pdf-lib'

// SVG path for a wavy line
const svgPath =
  'M 0,20 L 100,160 Q 130,200 150,120 C 190,-40 200,200 300,150 L 400,90'

// Create a new PDFDocument
const pdfDoc = await PDFDocument.create()

// Add a blank page to the document
const page = pdfDoc.addPage()
page.moveTo(100, page.getHeight() - 5)

// Draw the SVG path as a black line
page.moveDown(25)
page.drawSvgPath(svgPath)

// Draw the SVG path as a thick green line
page.moveDown(200)
page.drawSvgPath(svgPath, { borderColor: rgb(0, 1, 0), borderWidth: 5 })

// Draw the SVG path and fill it with red
page.moveDown(200)
page.drawSvgPath(svgPath, { color: rgb(1, 0, 0) })

// Draw the SVG path at 50% of its original size
page.moveDown(200)
page.drawSvgPath(svgPath, { scale: 0.5 })

// Serialize the PDFDocument to bytes (a Uint8Array)
const pdfBytes = await pdfDoc.save()

// For example, `pdfBytes` can be:
//   • Written to a file in Node
//   • Downloaded from the browser
//   • Rendered in an <iframe>
```

## README: Deno Usage

`pdf-lib` fully supports the exciting new [Deno](https://deno.land/) runtime! All of the [usage examples](#usage-examples) work in Deno. The only thing you need to do is change the imports for `pdf-lib` and `@pdf-lib/fontkit` to use the [Skypack](https://www.skypack.dev/) CDN, because Deno requires all modules to be referenced via URLs.

> **See also [How to Create and Modify PDF Files in Deno With pdf-lib](https://medium.com/swlh/how-to-create-and-modify-pdf-files-in-deno-ffaad7099b0?source=friends_link&sk=3da183bb776d059df428eaea52102f19)**

### Creating a Document with Deno

Below is the [**create document**](#create-document) example modified for Deno:

```js
import {
  PDFDocument,
  StandardFonts,
  rgb,
} from 'https://cdn.skypack.dev/pdf-lib@^1.11.1?dts';

const pdfDoc = await PDFDocument.create();
const timesRomanFont = await pdfDoc.embedFont(StandardFonts.TimesRoman);

const page = pdfDoc.addPage();
const { width, height } = page.getSize();
const fontSize = 30;
page.drawText('Creating PDFs in JavaScript is awesome!', {
  x: 50,
  y: height - 4 * fontSize,
  size: fontSize,
  font: timesRomanFont,
  color: rgb(0, 0.53, 0.71),
});

const pdfBytes = await pdfDoc.save();

await Deno.writeFile('out.pdf', pdfBytes);
```

If you save this script as `create-document.ts`, you can execute it using Deno with the following command:

```
deno run --allow-write create-document.ts
```

The resulting `out.pdf` file will look like [this PDF](assets/pdfs/examples/create_document.pdf).

### Embedding a Font with Deno

Here's a slightly more complicated example demonstrating how to embed a font and measure text in Deno:

```not-for-this-stick
import {
  degrees,
  PDFDocument,
  rgb,
  StandardFonts,
} from 'https://cdn.skypack.dev/pdf-lib@^1.11.1?dts';
import fontkit from 'https://cdn.skypack.dev/@pdf-lib/fontkit@^1.0.0?dts';

const url = 'https://pdf-lib.js.org/assets/ubuntu/Ubuntu-R.ttf';
const fontBytes = await fetch(url).then((res) => res.arrayBuffer());

const pdfDoc = await PDFDocument.create();

pdfDoc.registerFontkit(fontkit);
const customFont = await pdfDoc.embedFont(fontBytes);

const page = pdfDoc.addPage();

const text = 'This is text in an embedded font!';
const textSize = 35;
const textWidth = customFont.widthOfTextAtSize(text, textSize);
const textHeight = customFont.heightAtSize(textSize);

page.drawText(text, {
  x: 40,
  y: 450,
  size: textSize,
  font: customFont,
  color: rgb(0, 0.53, 0.71),
});
page.drawRectangle({
  x: 40,
  y: 450,
  width: textWidth,
  height: textHeight,
  borderColor: rgb(1, 0, 0),
  borderWidth: 1.5,
});

const pdfBytes = await pdfDoc.save();

await Deno.writeFile('out.pdf', pdfBytes);
```

If you save this script as `custom-font.ts`, you can execute it with the following command:

```
deno run --allow-write --allow-net custom-font.ts
```

The resulting `out.pdf` file will look like [this PDF](assets/pdfs/examples/embed_font_and_measure_text.pdf).

## README: Complete Examples

The [usage examples](#usage-examples) provide code that is brief and to the point, demonstrating the different features of `pdf-lib`. You can find complete working examples in the [`apps/`](apps/) directory. These apps are used to do manual testing of `pdf-lib` before every release (in addition to the [automated tests](tests/)).

There are currently four apps:

- [**`node`**](apps/node/) - contains [tests](apps/node/tests/) for `pdf-lib` in Node environments. These tests are a handy reference when trying to save/load PDFs, fonts, or images with `pdf-lib` from the filesystem. They also allow you to quickly open your PDFs in different viewers (Acrobat, Preview, Foxit, Chrome, Firefox, etc...) to ensure compatibility.
- [**`web`**](apps/web/) - contains [tests](apps/web/) for `pdf-lib` in browser environments. These tests are a handy reference when trying to save/load PDFs, fonts, or images with `pdf-lib` in a browser environment.
- [**`rn`**](apps/rn) - contains [tests](apps/rn/src/tests/) for `pdf-lib` in React Native environments. These tests are a handy reference when trying to save/load PDFs, fonts, or images with `pdf-lib` in a React Native environment.
- [**`deno`**](apps/deno) - contains [tests](apps/deno/tests/) for `pdf-lib` in Deno environments. These tests are a handy reference when trying to save/load PDFs, fonts, or images with `pdf-lib` from the filesystem.

## README: Installation

### NPM Module

To install the latest stable version:

```not-for-this-stick
# With npm
npm install --save pdf-lib

# With yarn
yarn add pdf-lib
```

Not for this drive: This assumes you're using [npm](https://www.npmjs.com/) or [yarn](https://yarnpkg.com/lang/en/) as your package manager.

### UMD Module

You can also download `pdf-lib` as a UMD module from [unpkg](https://unpkg.com/#/) or [jsDelivr](https://www.jsdelivr.com/). The UMD builds have been compiled to ES5, so they should work [in any modern browser](https://caniuse.com/#feat=es5). UMD builds are useful if you aren't using a package manager or module bundler. For example, you can use them directly in the `<script>` tag of an HTML page.

The following builds are available:

- https://unpkg.com/pdf-lib/dist/pdf-lib.js
- https://unpkg.com/pdf-lib/dist/pdf-lib.min.js
Not for this drive: - https://cdn.jsdelivr.net/npm/pdf-lib/dist/pdf-lib.js
Not for this drive: - https://cdn.jsdelivr.net/npm/pdf-lib/dist/pdf-lib.min.js

> **NOTE:** if you are using the CDN scripts in production, you should include a specific version number in the URL, for example:
>
> - https://unpkg.com/pdf-lib@1.4.0/dist/pdf-lib.min.js
Not for this drive: > - https://cdn.jsdelivr.net/npm/pdf-lib@1.4.0/dist/pdf-lib.min.js

When using a UMD build, you will have access to a global `window.PDFLib` variable. This variable contains all of the classes and functions exported by `pdf-lib`. For example:

```javascript
// NPM module
import { PDFDocument, rgb } from 'pdf-lib';

// UMD module
var PDFDocument = PDFLib.PDFDocument;
var rgb = PDFLib.rgb;
```

## README: Fontkit Installation

`pdf-lib` relies upon a sister module to support embedding custom fonts: [`@pdf-lib/fontkit`](https://www.npmjs.com/package/@pdf-lib/fontkit). You must add the `@pdf-lib/fontkit` module to your project and register it using `pdfDoc.registerFontkit(...)` before embedding custom fonts (see the [font embedding example](#embed-font-and-measure-text)). This module is not included by default because not all users need it, and it increases bundle size.

Not for this drive: Installing this module is easy. Just like `pdf-lib` itself, `@pdf-lib/fontkit` can be installed with `npm`/`yarn` or as a UMD module.

### Fontkit NPM Module

```not-for-this-stick
# With npm
npm install --save @pdf-lib/fontkit

# With yarn
yarn add @pdf-lib/fontkit
```

To register the `fontkit` instance:


```js
import { PDFDocument } from 'pdf-lib'
import fontkit from '@pdf-lib/fontkit'

const pdfDoc = await PDFDocument.create()
pdfDoc.registerFontkit(fontkit)
```

### Fontkit UMD Module

The following builds are available:

- https://unpkg.com/@pdf-lib/fontkit/dist/fontkit.umd.js
- https://unpkg.com/@pdf-lib/fontkit/dist/fontkit.umd.min.js
Not for this drive: - https://cdn.jsdelivr.net/npm/@pdf-lib/fontkit/dist/fontkit.umd.js
Not for this drive: - https://cdn.jsdelivr.net/npm/@pdf-lib/fontkit/dist/fontkit.umd.min.js

> **NOTE:** if you are using the CDN scripts in production, you should include a specific version number in the URL, for example:
>
> - https://unpkg.com/@pdf-lib/fontkit@0.0.4/dist/fontkit.umd.min.js
Not for this drive: > - https://cdn.jsdelivr.net/npm/@pdf-lib/fontkit@0.0.4/dist/fontkit.umd.min.js

When using a UMD build, you will have access to a global `window.fontkit` variable. To register the `fontkit` instance:


```js
var pdfDoc = await PDFLib.PDFDocument.create()
pdfDoc.registerFontkit(fontkit)
```

## README: Documentation

API documentation is available on the project site at https://pdf-lib.js.org/docs/api/.

The repo for the project site (and generated documentation files) is
located here: https://github.com/Hopding/pdf-lib-docs.

## README: Fonts and Unicode

When working with PDFs, you will frequently come across the terms "character encoding" and "font". If you have experience in web development, you may wonder why these are so prevalent. Aren't they just annoying details that you shouldn't need to worry about? Shouldn't PDF libraries and readers be able to handle all of this for you like web browsers can? Unfortunately, this is not the case. The nature of the PDF file format makes it very difficult to avoid thinking about character encodings and fonts when working with PDFs.

`pdf-lib` does its best to simplify things for you. But it can't perform magic. This means you should be aware of the following:

- **There are 14 standard fonts** defined in the PDF specification. They are as follows: _Times Roman_ (normal, bold, and italic), _Helvetica_ (normal, bold, and italic), _Courier_ (normal, bold, and italic), _ZapfDingbats_ (normal), and _Symbol_ (normal). These 14 fonts are guaranteed to be available in PDF readers. As such, you do not need to embed any font data if you wish to use one of these fonts. You can use a standard font like so:
  
  ```js
  import { PDFDocument, StandardFonts } from 'pdf-lib'
  const pdfDoc = await PDFDocument.create()
  const courierFont = await pdfDoc.embedFont(StandardFonts.Courier)
  const page = pdfDoc.addPage()
  page.drawText('Some boring latin text in the Courier font', {
    font: courierFont,
  })
  ```
- **The standard fonts do not support all characters** available in Unicode. The Times Roman, Helvetica, and Courier fonts use WinAnsi encoding (aka [Windows-1252](https://en.wikipedia.org/wiki/Windows-1252)). The WinAnsi character set only supports 218 characters in the Latin alphabet. For this reason, many users will find the standard fonts insufficient for their use case. This is unfortunate, but there's nothing that PDF libraries can do to change this. This is a result of the PDF specification and its age. Note that the [ZapfDingbats](https://en.wikipedia.org/wiki/Zapf_Dingbats) and [Symbol](<https://en.wikipedia.org/wiki/Symbol_(typeface)>) fonts use their own specialized encodings that support 203 and 194 characters, respectively. However, the characters they support are not useful for most use cases. See [here](assets/pdfs/standard_fonts_demo.pdf) for an example of all 14 standard fonts.
- **You can use characters outside the Latin alphabet** by embedding your own fonts. Embedding your own font requires to you load the font data (from a file or via a network request, for example) and pass it to the `embedFont` method. When you embed your own font, you can use any Unicode characters that it supports. This capability frees you from the limitations imposed by the standard fonts. Most PDF files use embedded fonts. You can embed and use a custom font like so ([see also](#embed-font-and-measure-text)):
  
  ```js
  import { PDFDocument } from 'pdf-lib'
  import fontkit from '@pdf-lib/fontkit'

  const url = 'https://pdf-lib.js.org/assets/ubuntu/Ubuntu-R.ttf'
  const fontBytes = await fetch(url).then((res) => res.arrayBuffer())

  const pdfDoc = await PDFDocument.create()

  pdfDoc.registerFontkit(fontkit)
  const ubuntuFont = await pdfDoc.embedFont(fontBytes)

  const page = pdfDoc.addPage()
  page.drawText('Some fancy Unicode text in the ŪЬȕǹƚü font', {
    font: ubuntuFont,
  })
  ```

Note that encoding errors will be thrown if you try to use a character with a font that does not support it. For example, `Ω` is not in the WinAnsi character set. So trying to draw it on a page with the standard Helvetica font will throw the following error:

```
Error: WinAnsi cannot encode "Ω" (0x03a9)
    at Encoding.encodeUnicodeCodePoint
```

### Font Subsetting

Embedding a font in a PDF document will typically increase the file's size. You can reduce the amount a file's size is increased by subsetting the font so that only the necessary characters are embedded. You can subset a font by setting the [`subset` option](https://pdf-lib.js.org/docs/api/interfaces/embedfontoptions#optional-subset) to `true`. For example:

```js
const font = await pdfDoc.embedFont(fontBytes, { subset: true });
```

Note that subsetting does not work for all fonts. See https://github.com/Hopding/pdf-lib/issues/207#issuecomment-537210471 for additional details.

## README: Creating and Filling Forms

`pdf-lib` can create, fill, and read PDF form fields. The following field types are supported:

- [Buttons](https://pdf-lib.js.org/docs/api/classes/pdfbutton)
- [Check Boxes](https://pdf-lib.js.org/docs/api/classes/pdfcheckbox)
- [Dropdowns](https://pdf-lib.js.org/docs/api/classes/pdfdropdown)
- [Option Lists](https://pdf-lib.js.org/docs/api/classes/pdfoptionlist)
- [Radio Groups](https://pdf-lib.js.org/docs/api/classes/pdfradiogroup)
- [Text Fields](https://pdf-lib.js.org/docs/api/classes/pdftextfield)

See the [form creation](#create-form) and [form filling](#fill-form) usage examples for code samples. Tests 1, 14, 15, 16, and 17 in the [complete examples](#complete-examples) contain working example code for form creation and filling in a variety of different JS environments.

**IMPORTANT:** The default font used to display text in buttons, dropdowns, option lists, and text fields is the standard Helvetica font. This font only supports characters in the latin alphabet (see [Fonts and Unicode](#fonts-and-unicode) for details). This means that if any of these field types are created or modified to contain text outside the latin alphabet (as is often the case), you will need to embed and use a custom font to update the field appearances. Otherwise an error will be thrown (likely when you save the `PDFDocument`).

You can use an embedded font when filling form fields as follows:

```js
import { PDFDocument } from 'pdf-lib';
import fontkit from '@pdf-lib/fontkit';

// Fetch the PDF with form fields
const formUrl = 'https://pdf-lib.js.org/assets/dod_character.pdf';
const formBytes = await fetch(formUrl).then((res) => res.arrayBuffer());

// Fetch the Ubuntu font
const fontUrl = 'https://pdf-lib.js.org/assets/ubuntu/Ubuntu-R.ttf';
const fontBytes = await fetch(fontUrl).then((res) => res.arrayBuffer());

// Load the PDF with form fields
const pdfDoc = await PDFDocument.load(formBytes);

// Embed the Ubuntu font
pdfDoc.registerFontkit(fontkit);
const ubuntuFont = await pdfDoc.embedFont(fontBytes);

// Get two text fields from the form
const form = pdfDoc.getForm();
const nameField = form.getTextField('CharacterName 2');
const ageField = form.getTextField('Age');

// Fill the text fields with some fancy Unicode characters (outside
// the WinAnsi latin character set)
nameField.setText('Ӎӑȑїõ');
ageField.setText('24 ŷȇȁŗš');

// **Key Step:** Update the field appearances with the Ubuntu font
form.updateFieldAppearances(ubuntuFont);

// Save the PDF with filled form fields
const pdfBytes = await pdfDoc.save();
```

### Handy Methods for Filling, Creating, and Reading Form Fields

Existing form fields can be accessed with the following methods of [`PDFForm`](https://pdf-lib.js.org/docs/api/classes/pdfform):

- [`PDFForm.getButton`](https://pdf-lib.js.org/docs/api/classes/pdfform#getbutton)
- [`PDFForm.getCheckBox`](https://pdf-lib.js.org/docs/api/classes/pdfform#getcheckbox)
- [`PDFForm.getDropdown`](https://pdf-lib.js.org/docs/api/classes/pdfform#getdropdown)
- [`PDFForm.getOptionList`](https://pdf-lib.js.org/docs/api/classes/pdfform#getoptionlist)
- [`PDFForm.getRadioGroup`](https://pdf-lib.js.org/docs/api/classes/pdfform#getradiogroup)
- [`PDFForm.getTextField`](https://pdf-lib.js.org/docs/api/classes/pdfform#gettextfield)

New form fields can be created with the following methods of [`PDFForm`](https://pdf-lib.js.org/docs/api/classes/pdfform):

- [`PDFForm.createButton`](https://pdf-lib.js.org/docs/api/classes/pdfform#createbutton)
- [`PDFForm.createCheckBox`](https://pdf-lib.js.org/docs/api/classes/pdfform#createcheckbox)
- [`PDFForm.createDropdown`](https://pdf-lib.js.org/docs/api/classes/pdfform#createdropdown)
- [`PDFForm.createOptionList`](https://pdf-lib.js.org/docs/api/classes/pdfform#createoptionlist)
- [`PDFForm.createRadioGroup`](https://pdf-lib.js.org/docs/api/classes/pdfform#createradiogroup)
- [`PDFForm.createTextField`](https://pdf-lib.js.org/docs/api/classes/pdfform#createtextfield)

Below are some of the most commonly used methods for reading and filling the aforementioned subclasses of [`PDFField`](https://pdf-lib.js.org/docs/api/classes/pdffield):

- [`PDFCheckBox.check`](https://pdf-lib.js.org/docs/api/classes/pdfcheckbox#check)
- [`PDFCheckBox.uncheck`](https://pdf-lib.js.org/docs/api/classes/pdfcheckbox#uncheck)
- [`PDFCheckBox.isChecked`](https://pdf-lib.js.org/docs/api/classes/pdfcheckbox#ischecked)

---

- [`PDFDropdown.select`](https://pdf-lib.js.org/docs/api/classes/pdfdropdown#select)
- [`PDFDropdown.clear`](https://pdf-lib.js.org/docs/api/classes/pdfdropdown#clear)
- [`PDFDropdown.getSelected`](https://pdf-lib.js.org/docs/api/classes/pdfdropdown#getselected)
- [`PDFDropdown.getOptions`](https://pdf-lib.js.org/docs/api/classes/pdfdropdown#getoptions)
- [`PDFDropdown.addOptions`](https://pdf-lib.js.org/docs/api/classes/pdfdropdown#addoptions)

---

- [`PDFOptionList.select`](https://pdf-lib.js.org/docs/api/classes/pdfoptionlist#select)
- [`PDFOptionList.clear`](https://pdf-lib.js.org/docs/api/classes/pdfoptionlist#clear)
- [`PDFOptionList.getSelected`](https://pdf-lib.js.org/docs/api/classes/pdfoptionlist#getselected)
- [`PDFOptionList.getOptions`](https://pdf-lib.js.org/docs/api/classes/pdfoptionlist#getoptions)
- [`PDFOptionList.addOptions`](https://pdf-lib.js.org/docs/api/classes/pdfoptionlist#addoptions)

---

- [`PDFRadioGroup.select`](https://pdf-lib.js.org/docs/api/classes/pdfradiogroup#select)
- [`PDFRadioGroup.clear`](https://pdf-lib.js.org/docs/api/classes/pdfradiogroup#clear)
- [`PDFRadioGroup.getSelected`](https://pdf-lib.js.org/docs/api/classes/pdfradiogroup#getselected)
- [`PDFRadioGroup.getOptions`](https://pdf-lib.js.org/docs/api/classes/pdfradiogroup#getoptions)
- [`PDFRadioGroup.addOptionToPage`](https://pdf-lib.js.org/docs/api/classes/pdfradiogroup#addoptiontopage)

---

- [`PDFTextField.setText`](https://pdf-lib.js.org/docs/api/classes/pdftextfield#settext)
- [`PDFTextField.getText`](https://pdf-lib.js.org/docs/api/classes/pdftextfield#gettext)
- [`PDFTextField.setMaxLength`](https://pdf-lib.js.org/docs/api/classes/pdftextfield#setmaxlength)
- [`PDFTextField.getMaxLength`](https://pdf-lib.js.org/docs/api/classes/pdftextfield#getmaxlength)
- [`PDFTextField.removeMaxLength`](https://pdf-lib.js.org/docs/api/classes/pdftextfield#removemaxlength)

## README: Limitations

- `pdf-lib` **can** extract the content of text fields (see [`PDFTextField.getText`](https://pdf-lib.js.org/docs/api/classes/pdftextfield#gettext)), but it **cannot** extract plain text on a page outside of a form field. This is a difficult feature to implement, but it is within the scope of this library and may be added to `pdf-lib` in the future. See
  [#93](https://github.com/Hopding/pdf-lib/issues/93),
  [#137](https://github.com/Hopding/pdf-lib/issues/137),
  [#177](https://github.com/Hopding/pdf-lib/issues/177),
  [#329](https://github.com/Hopding/pdf-lib/issues/329), and
  [#380](https://github.com/Hopding/pdf-lib/issues/380).
- `pdf-lib` **can** remove and edit the content of text fields (see [`PDFTextField.setText`](https://pdf-lib.js.org/docs/api/classes/pdftextfield#settext)), but it does **not** provide APIs for removing or editing text on a page outside of a form field. This is also a difficult feature to implement, but is within the scope of `pdf-lib` and may be added in the future. See
  [#93](https://github.com/Hopding/pdf-lib/issues/93),
  [#137](https://github.com/Hopding/pdf-lib/issues/137),
  [#177](https://github.com/Hopding/pdf-lib/issues/177),
  [#329](https://github.com/Hopding/pdf-lib/issues/329), and
  [#380](https://github.com/Hopding/pdf-lib/issues/380).
- `pdf-lib` does **not** support the use of HTML or CSS when adding content to a PDF. Similarly, `pdf-lib` **cannot** embed HTML/CSS content into PDFs. As convenient as such a feature might be, it would be extremely difficult to implement and is far beyond the scope of this library. If this capability is something you need, consider using [Puppeteer](https://github.com/puppeteer/puppeteer).

## README: Help and Discussion

[Discussions](https://github.com/Hopding/pdf-lib/discussions) is the best place to chat with us, ask questions, and learn more about pdf-lib!

See also [MAINTAINERSHIP.md#communication](docs/MAINTAINERSHIP.md#communication) and [MAINTAINERSHIP.md#discord](docs/MAINTAINERSHIP.md#discord).

## README: Encryption Handling

**`pdf-lib` does not currently support encrypted documents.** You should not use `pdf-lib` with encrypted documents. However, this is a feature that could be added to `pdf-lib`. Please [create an issue](https://github.com/Hopding/pdf-lib/issues/new) if you would find this feature helpful!

When an encrypted document is passed to `PDFDocument.load(...)`, an error will be thrown:


```js
import { PDFDocument, EncryptedPDFError } from 'pdf-lib'

const encryptedPdfBytes = ...

// Assignment fails. Throws an `EncryptedPDFError`.
const pdfDoc = PDFDocument.load(encryptedPdfBytes)
```

This default behavior is usually what you want. It allows you to easily detect if a given document is encrypted, and it prevents you from trying to modify it. However, if you really want to load the document, you can use the `{ ignoreEncryption: true }` option:

```js
import { PDFDocument } from 'pdf-lib'

const encryptedPdfBytes = ...

// Assignment succeeds. Does not throw an error.
const pdfDoc = PDFDocument.load(encryptedPdfBytes, { ignoreEncryption: true })
```

Note that **using this option does not decrypt the document**. This means that any modifications you attempt to make on the returned `PDFDocument` may fail, or have unexpected results.

**You should not use this option.** It only exists for backwards compatibility reasons.

## README: Contributing

We welcome contributions from the open source community! If you are interested in contributing to `pdf-lib`, please take a look at the [CONTRIBUTING.md](docs/CONTRIBUTING.md) file. It contains information to help you get `pdf-lib` setup and running on your machine. (We try to make this as simple and fast as possible! :rocket:)

## README: Maintainership

Check out [MAINTAINERSHIP.md](docs/MAINTAINERSHIP.md) for details on how this repo is maintained and how we use [issues](docs/MAINTAINERSHIP.md#issues), [PRs](docs/MAINTAINERSHIP.md#pull-requests), and [discussions](docs/MAINTAINERSHIP.md#discussions).

## README: Tutorials and Cool Stuff

- [labelmake](https://github.com/hand-dot/labelmake) - a library for declarative PDF generation created by @hand-dot
- [Möbius Printing helper](https://shreevatsa.net/mobius-print/) - a tool created by @shreevatsa
- [Extract PDF pages](https://shreevatsa.net/pdf-pages/) - a tool created by @shreevatsa
- [Travel certificate generator](https://github.com/LAB-MI/deplacement-covid-19) - a tool that creates travel certificates for French citizens under quarantine due to COVID-19
- [How to use pdf-lib in AWS Lambdas](https://medium.com/swlh/create-pdf-using-pdf-lib-on-serverless-aws-lambda-e9506246dc88) - a tutorial written by Crespo Wang
- [Working With PDFs in Node.js Using pdf-lib](http://thecodebarbarian.com/working-with-pdfs-in-node-js.html) - a tutorial by Valeri Karpov
- [Electron app for resizing PDFs](https://github.com/vegarringdal/simple-pdf-resizer) - a tool created by @vegarringdal
- [PDF Shelter](https://pdfshelter.com) - online PDF manipulation tools by Lucas Morais

## README: Prior Art

- [`pdfkit`](https://github.com/devongovett/pdfkit) is a PDF generation library for Node and the Browser. This library was immensely helpful as a reference and existence proof when creating `pdf-lib`. `pdfkit`'s code for [font embedding](src/core/embedders/CustomFontEmbedder.ts#L17-L21), [PNG embedding](src/core/embedders/PngEmbedder.ts#L7-L11), and [JPG embedding](src/core/embedders/JpegEmbedder.ts#L25-L29) was especially useful.
- [`pdf.js`](https://github.com/mozilla/pdf.js) is a PDF rendering library for the Browser. This library was helpful as a reference when writing `pdf-lib`'s parser. Some of the code for stream decoding was [ported directly to TypeScript](src/core/streams) for use in `pdf-lib`.
- [`pdfbox`](https://pdfbox.apache.org/) is a PDF generation and modification library written in Java. This library was an invaluable reference when implementing form creation and filling APIs for `pdf-lib`.
- [`jspdf`](https://github.com/MrRio/jsPDF) is a PDF generation library for the browser.
- [`pdfmake`](https://github.com/bpampuch/pdfmake) is a PDF generation library for the browser.
- [`hummus`](https://github.com/galkahana/HummusJS) is a PDF generation and modification library for Node environments. `hummus` is a Node wrapper around a [C++ library](https://github.com/galkahana/PDF-Writer), so it doesn't work in many JavaScript environments - like the Browser or React Native.
- [`react-native-pdf-lib`](https://github.com/Hopding/react-native-pdf-lib) is a PDF generation and modification library for React Native environments. `react-native-pdf-lib` is a wrapper around [C++](https://github.com/galkahana/PDF-Writer) and [Java](https://github.com/TomRoush/PdfBox-Android) libraries.
- [`pdfassembler`](https://github.com/DevelopingMagic/pdfassembler) is a PDF generation and modification library for Node and the browser. It requires some knowledge about the logical structure of PDF documents to use.

## README: Git History Rewrite

This repo used to contain a file called `pdf_specification.pdf` in the root directory. This was a copy of the [PDF 1.7 specification](https://www.adobe.com/content/dam/acom/en/devnet/pdf/pdfs/PDF32000_2008.pdf), which is made freely available by Adobe. On 8/30/2021, we received a DMCA complaint requiring us to remove the file from this repo. Simply removing the file via a new commit to `master` was insufficient to satisfy the complaint. The file needed to be completely removed from the repo's git history. Unfortunately, the file was added over two years ago, this meant we had to rewrite the repo's git history and force push to `master` 😔.

### Steps We Took

We removed the file and rewrote the repo's history using [BFG Repo-Cleaner](https://rtyley.github.io/bfg-repo-cleaner/) as outlined [here](https://docs.github.com/en/github/authenticating-to-github/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository). For full transparency, here are the exact commands we ran:

```not-for-this-stick
$ git clone git@github.com:Hopding/pdf-lib.git
$ cd pdf-lib
$ rm pdf_specification.pdf
$ git commit -am 'Remove pdf_specification.pdf'
$ bfg --delete-files pdf_specification.pdf
$ git reflog expire --expire=now --all && git gc --prune=now --aggressive
$ git push --force
```

### Why Should I Care?

If you're a user of `pdf-lib`, you shouldn't care! Just keep on using `pdf-lib` like normal 😃 ✨!

If you are a `pdf-lib` developer (meaning you've forked `pdf-lib` and/or have an open PR) then this does impact you. If you forked or cloned the repo prior to 8/30/2021 then your fork's git history is out of sync with this repo's `master` branch. Unfortunately, this will likely be a headache for you to deal with. Sorry! We didn't want to rewrite the history, but there really was no alternative.

It's important to note that pdf-lib's _source code_ has not changed at all. It's exactly the same as it was before the git history rewrite. The repo still has the exact same number of commits (and even the same commit contents, except for the commit that added `pdf_specification.pdf`). What has changed are the SHAs of those commits.

The simplest way to deal with this fact is to:

1. Reclone pdf-lib
2. Manually copy any changes you've made from your old clone to the new one
3. Use your new clone going forward
4. Reopen your unmerged PRs using your new clone

See this [StackOverflow answer](https://stackoverflow.com/a/48268766) for a great, in depth explanation of what a git history rewrite entails.

## README: License

[MIT](LICENSE.md)


## Types

The package's type definitions, 36 files.

### es/api/Embeddable.d.ts

```ts
/**
 * A PDF entity, like images or fonts, which needs to be embedded into the
 * document before saving.
 */
export default interface Embeddable {
    embed: () => Promise<void>;
}
//# sourceMappingURL=Embeddable.d.ts.map
```

### es/api/PDFDocument.d.ts

```ts
import PDFEmbeddedPage from "./PDFEmbeddedPage";
import PDFFont from "./PDFFont";
import PDFImage from "./PDFImage";
import PDFPage from "./PDFPage";
import PDFForm from "./form/PDFForm";
import { StandardFonts } from "./StandardFonts";
import { PageBoundingBox, PDFCatalog, PDFContext } from "../core";
import { AttachmentOptions, SaveOptions, Base64SaveOptions, LoadOptions, CreateOptions, EmbedFontOptions, SetTitleOptions } from "./PDFDocumentOptions";
import PDFRef from "../core/objects/PDFRef";
import { Fontkit } from "../types/fontkit";
import { TransformationMatrix } from "../types/matrix";
/**
 * Represents a PDF document.
 */
export default class PDFDocument {
    /**
     * Load an existing [[PDFDocument]]. The input data can be provided in
     * multiple formats:
     *
     * | Type          | Contents                                               |
     * | ------------- | ------------------------------------------------------ |
     * | `string`      | A base64 encoded string (or data URI) containing a PDF |
     * | `Uint8Array`  | The raw bytes of a PDF                                 |
     * | `ArrayBuffer` | The raw bytes of a PDF                                 |
     *
     * For example:
     * ```js
     * import { PDFDocument } from 'pdf-lib'
     *
     * // pdf=string
     * const base64 =
     *  'JVBERi0xLjcKJYGBgYEKCjUgMCBvYmoKPDwKL0ZpbHRlciAvRmxhdGVEZWNvZGUKL0xlbm' +
     *  'd0aCAxMDQKPj4Kc3RyZWFtCniccwrhMlAAwaJ0Ln2P1Jyy1JLM5ERdc0MjCwUjE4WQNC4Q' +
     *  '6cNlCFZkqGCqYGSqEJLLZWNuYGZiZmbkYuZsZmlmZGRgZmluDCQNzc3NTM2NzdzMXMxMjQ' +
     *  'ztFEKyuEK0uFxDuAAOERdVCmVuZHN0cmVhbQplbmRvYmoKCjYgMCBvYmoKPDwKL0ZpbHRl' +
     *  'ciAvRmxhdGVEZWNvZGUKL1R5cGUgL09ialN0bQovTiA0Ci9GaXJzdCAyMAovTGVuZ3RoID' +
     *  'IxNQo+PgpzdHJlYW0KeJxVj9GqwjAMhu/zFHkBzTo3nCCCiiKIHPEICuJF3cKoSCu2E8/b' +
     *  '20wPIr1p8v9/8kVhgilmGfawX2CGaVrgcAi0/bsy0lrX7IGWpvJ4iJYEN3gEmrrGBlQwGs' +
     *  'HHO9VBX1wNrxAqMX87RBD5xpJuddqwd82tjAHxzV1U5LPgy52DKXWnr1Lheg+j/c/pzGVr' +
     *  'iqV0VlwZPXGPCJjElw/ybkwUmeoWgxesDXGhHJC/D/iikp1Av80ptKU0FdBEe25pPihAM1' +
     *  'u6ytgaaWfs2Hrz35CJT1+EWmAKZW5kc3RyZWFtCmVuZG9iagoKNyAwIG9iago8PAovU2l6' +
     *  'ZSA4Ci9Sb290IDIgMCBSCi9GaWx0ZXIgL0ZsYXRlRGVjb2RlCi9UeXBlIC9YUmVmCi9MZW' +
     *  '5ndGggMzgKL1cgWyAxIDIgMiBdCi9JbmRleCBbIDAgOCBdCj4+CnN0cmVhbQp4nBXEwREA' +
     *  'EBAEsCwz3vrvRmOOyyOoGhZdutHN2MT55fIAVocD+AplbmRzdHJlYW0KZW5kb2JqCgpzdG' +
     *  'FydHhyZWYKNTEwCiUlRU9G'
     *
     * const dataUri = 'data:application/pdf;base64,' + base64
     *
     * const pdfDoc1 = await PDFDocument.load(base64)
     * const pdfDoc2 = await PDFDocument.load(dataUri)
     *
     * // pdf=Uint8Array
     * import fs from 'fs'
     * const uint8Array = fs.readFileSync('with_update_sections.pdf')
     * const pdfDoc3 = await PDFDocument.load(uint8Array)
     *
     * // pdf=ArrayBuffer
     * const url = 'https://pdf-lib.js.org/assets/with_update_sections.pdf'
     * const arrayBuffer = await fetch(url).then(res => res.arrayBuffer())
     * const pdfDoc4 = await PDFDocument.load(arrayBuffer)
     *
     * ```
     *
     * @param pdf The input data containing a PDF document.
     * @param options The options to be used when loading the document.
     * @returns Resolves with a document loaded from the input.
     */
    static load(pdf: string | Uint8Array | ArrayBuffer, options?: LoadOptions): Promise<PDFDocument>;
    /**
     * Create a new [[PDFDocument]].
     * @returns Resolves with the newly created document.
     */
    static create(options?: CreateOptions): Promise<PDFDocument>;
    /** The low-level context of this document. */
    readonly context: PDFContext;
    /** The catalog of this document. */
    readonly catalog: PDFCatalog;
    /** Whether or not this document is encrypted. */
    readonly isEncrypted: boolean;
    /** The default word breaks used in PDFPage.drawText */
    defaultWordBreaks: string[];
    private fontkit?;
    private pageCount;
    private readonly pageCache;
    private readonly pageMap;
    private readonly formCache;
    private readonly fonts;
    private readonly images;
    private readonly embeddedPages;
    private readonly embeddedFiles;
    private readonly javaScripts;
    private constructor();
    /**
     * Register a fontkit instance. This must be done before custom fonts can
     * be embedded. See [here](https://github.com/Hopding/pdf-lib/tree/master#fontkit-installation)
     * for instructions on how to install and register a fontkit instance.
     *
     * > You do **not** need to call this method to embed standard fonts.
     *
     * For example:
     * ```js
     * import { PDFDocument } from 'pdf-lib'
     * import fontkit from '@pdf-lib/fontkit'
     *
     * const pdfDoc = await PDFDocument.create()
     * pdfDoc.registerFontkit(fontkit)
     * ```
     *
     * @param fontkit The fontkit instance to be registered.
     */
    registerFontkit(fontkit: Fontkit): void;
    /**
     * Get the [[PDFForm]] containing all interactive fields for this document.
     * For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const fields = form.getFields()
     * fields.forEach(field => {
     *   const type = field.constructor.name
     *   const name = field.getName()
     *   console.log(`${type}: ${name}`)
     * })
     * ```
     * @returns The form for this document.
     */
    getForm(): PDFForm;
    /**
     * Get this document's title metadata. The title appears in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * const title = pdfDoc.getTitle()
     * ```
     * @returns A string containing the title of this document, if it has one.
     */
    getTitle(): string | undefined;
    /**
     * Get this document's author metadata. The author appears in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * const author = pdfDoc.getAuthor()
     * ```
     * @returns A string containing the author of this document, if it has one.
     */
    getAuthor(): string | undefined;
    /**
     * Get this document's subject metadata. The subject appears in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * const subject = pdfDoc.getSubject()
     * ```
     * @returns A string containing the subject of this document, if it has one.
     */
    getSubject(): string | undefined;
    /**
     * Get this document's keywords metadata. The keywords appear in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * const keywords = pdfDoc.getKeywords()
     * ```
     * @returns A string containing the keywords of this document, if it has any.
     */
    getKeywords(): string | undefined;
    /**
     * Get this document's creator metadata. The creator appears in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * const creator = pdfDoc.getCreator()
     * ```
     * @returns A string containing the creator of this document, if it has one.
     */
    getCreator(): string | undefined;
    /**
     * Get this document's producer metadata. The producer appears in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * const producer = pdfDoc.getProducer()
     * ```
     * @returns A string containing the producer of this document, if it has one.
     */
    getProducer(): string | undefined;
    /**
     * Get this document's creation date metadata. The creation date appears in
     * the "Document Properties" section of most PDF readers. For example:
     * ```js
     * const creationDate = pdfDoc.getCreationDate()
     * ```
     * @returns A Date containing the creation date of this document,
     *          if it has one.
     */
    getCreationDate(): Date | undefined;
    /**
     * Get this document's modification date metadata. The modification date
     * appears in the "Document Properties" section of most PDF readers.
     * For example:
     * ```js
     * const modification = pdfDoc.getModificationDate()
     * ```
     * @returns A Date containing the modification date of this document,
     *          if it has one.
     */
    getModificationDate(): Date | undefined;
    /**
     * Set this document's title metadata. The title will appear in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * pdfDoc.setTitle('🥚 The Life of an Egg 🍳')
     * ```
     *
     * To display the title in the window's title bar, set the
     * `showInWindowTitleBar` option to `true` (works for _most_ PDF readers).
     * For example:
     * ```js
     * pdfDoc.setTitle('🥚 The Life of an Egg 🍳', { showInWindowTitleBar: true })
     * ```
     *
     * @param title The title of this document.
     * @param options The options to be used when setting the title.
     */
    setTitle(title: string, options?: SetTitleOptions): void;
    /**
     * Set this document's author metadata. The author will appear in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * pdfDoc.setAuthor('Humpty Dumpty')
     * ```
     * @param author The author of this document.
     */
    setAuthor(author: string): void;
    /**
     * Set this document's subject metadata. The subject will appear in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * pdfDoc.setSubject('📘 An Epic Tale of Woe 📖')
     * ```
     * @param subject The subject of this document.
     */
    setSubject(subject: string): void;
    /**
     * Set this document's keyword metadata. These keywords will appear in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * pdfDoc.setKeywords(['eggs', 'wall', 'fall', 'king', 'horses', 'men'])
     * ```
     * @param keywords An array of keywords associated with this document.
     */
    setKeywords(keywords: string[]): void;
    /**
     * Set this document's creator metadata. The creator will appear in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * pdfDoc.setCreator('PDF App 9000 🤖')
     * ```
     * @param creator The creator of this document.
     */
    setCreator(creator: string): void;
    /**
     * Set this document's producer metadata. The producer will appear in the
     * "Document Properties" section of most PDF readers. For example:
     * ```js
     * pdfDoc.setProducer('PDF App 9000 🤖')
     * ```
     * @param producer The producer of this document.
     */
    setProducer(producer: string): void;
    /**
     * Set this document's language metadata. The language will appear in the
     * "Document Properties" section of some PDF readers. For example:
     * ```js
     * pdfDoc.setLanguage('en-us')
     * ```
     *
     * @param language An RFC 3066 _Language-Tag_ denoting the language of this
     *                 document, or an empty string if the language is unknown.
     */
    setLanguage(language: string): void;
    /**
     * Set this document's creation date metadata. The creation date will appear
     * in the "Document Properties" section of most PDF readers. For example:
     * ```js
     * pdfDoc.setCreationDate(new Date())
     * ```
     * @param creationDate The date this document was created.
     */
    setCreationDate(creationDate: Date): void;
    /**
     * Set this document's modification date metadata. The modification date will
     * appear in the "Document Properties" section of most PDF readers. For
     * example:
     * ```js
     * pdfDoc.setModificationDate(new Date())
     * ```
     * @param modificationDate The date this document was last modified.
     */
    setModificationDate(modificationDate: Date): void;
    /**
     * Get the number of pages contained in this document. For example:
     * ```js
     * const totalPages = pdfDoc.getPageCount()
     * ```
     * @returns The number of pages in this document.
     */
    getPageCount(): number;
    /**
     * Get an array of all the pages contained in this document. The pages are
     * stored in the array in the same order that they are rendered in the
     * document. For example:
     * ```js
     * const pages = pdfDoc.getPages()
     * pages[0]   // The first page of the document
     * pages[2]   // The third page of the document
     * pages[197] // The 198th page of the document
     * ```
     * @returns An array of all the pages contained in this document.
     */
    getPages(): PDFPage[];
    /**
     * Get the page rendered at a particular `index` of the document. For example:
     * ```js
     * pdfDoc.getPage(0)   // The first page of the document
     * pdfDoc.getPage(2)   // The third page of the document
     * pdfDoc.getPage(197) // The 198th page of the document
     * ```
     * @returns The [[PDFPage]] rendered at the given `index` of the document.
     */
    getPage(index: number): PDFPage;
    /**
     * Get an array of indices for all the pages contained in this document. The
     * array will contain a range of integers from
     * `0..pdfDoc.getPageCount() - 1`. For example:
     * ```js
     * const pdfDoc = await PDFDocument.create()
     * pdfDoc.addPage()
     * pdfDoc.addPage()
     * pdfDoc.addPage()
     *
     * const indices = pdfDoc.getPageIndices()
     * indices // => [0, 1, 2]
     * ```
     * @returns An array of indices for all pages contained in this document.
     */
    getPageIndices(): number[];
    /**
     * Remove the page at a given index from this document. For example:
     * ```js
     * pdfDoc.removePage(0)   // Remove the first page of the document
     * pdfDoc.removePage(2)   // Remove the third page of the document
     * pdfDoc.removePage(197) // Remove the 198th page of the document
     * ```
     * Once a page has been removed, it will no longer be rendered at that index
     * in the document.
     * @param index The index of the page to be removed.
     */
    removePage(index: number): void;
    /**
     * Add a page to the end of this document. This method accepts three
     * different value types for the `page` parameter:
     *
     * | Type               | Behavior                                                                            |
     * | ------------------ | ----------------------------------------------------------------------------------- |
     * | `undefined`        | Create a new page and add it to the end of this document                            |
     * | `[number, number]` | Create a new page with the given dimensions and add it to the end of this document  |
     * | `PDFPage`          | Add the existing page to the end of this document                                   |
     *
     * For example:
     * ```js
     * // page=undefined
     * const newPage = pdfDoc.addPage()
     *
     * // page=[number, number]
     * import { PageSizes } from 'pdf-lib'
     * const newPage1 = pdfDoc.addPage(PageSizes.A7)
     * const newPage2 = pdfDoc.addPage(PageSizes.Letter)
     * const newPage3 = pdfDoc.addPage([500, 750])
     *
     * // page=PDFPage
     * const pdfDoc1 = await PDFDocument.create()
     * const pdfDoc2 = await PDFDocument.load(...)
     * const [existingPage] = await pdfDoc1.copyPages(pdfDoc2, [0])
     * pdfDoc1.addPage(existingPage)
     * ```
     *
     * @param page Optionally, the desired dimensions or existing page.
     * @returns The newly created (or existing) page.
     */
    addPage(page?: PDFPage | [number, number]): PDFPage;
    /**
     * Insert a page at a given index within this document. This method accepts
     * three different value types for the `page` parameter:
     *
     * | Type               | Behavior                                                                       |
     * | ------------------ | ------------------------------------------------------------------------------ |
     * | `undefined`        | Create a new page and insert it into this document                             |
     * | `[number, number]` | Create a new page with the given dimensions and insert it into this document   |
     * | `PDFPage`          | Insert the existing page into this document                                    |
     *
     * For example:
     * ```js
     * // page=undefined
     * const newPage = pdfDoc.insertPage(2)
     *
     * // page=[number, number]
     * import { PageSizes } from 'pdf-lib'
     * const newPage1 = pdfDoc.insertPage(2, PageSizes.A7)
     * const newPage2 = pdfDoc.insertPage(0, PageSizes.Letter)
     * const newPage3 = pdfDoc.insertPage(198, [500, 750])
     *
     * // page=PDFPage
     * const pdfDoc1 = await PDFDocument.create()
     * const pdfDoc2 = await PDFDocument.load(...)
     * const [existingPage] = await pdfDoc1.copyPages(pdfDoc2, [0])
     * pdfDoc1.insertPage(0, existingPage)
     * ```
     *
     * @param index The index at which the page should be inserted (zero-based).
     * @param page Optionally, the desired dimensions or existing page.
     * @returns The newly created (or existing) page.
     */
    insertPage(index: number, page?: PDFPage | [number, number]): PDFPage;
    /**
     * Copy pages from a source document into this document. Allows pages to be
     * copied between different [[PDFDocument]] instances. For example:
     * ```js
     * const pdfDoc = await PDFDocument.create()
     * const srcDoc = await PDFDocument.load(...)
     *
     * const copiedPages = await pdfDoc.copyPages(srcDoc, [0, 3, 89])
     * const [firstPage, fourthPage, ninetiethPage] = copiedPages;
     *
     * pdfDoc.addPage(fourthPage)
     * pdfDoc.insertPage(0, ninetiethPage)
     * pdfDoc.addPage(firstPage)
     * ```
     * @param srcDoc The document from which pages should be copied.
     * @param indices The indices of the pages that should be copied.
     * @returns Resolves with an array of pages copied into this document.
     */
    copyPages(srcDoc: PDFDocument, indices: number[]): Promise<PDFPage[]>;
    /**
     * Get a copy of this document.
     *
     * For example:
     * ```js
     * const srcDoc = await PDFDocument.load(...)
     * const pdfDoc = await srcDoc.copy()
     * ```
     *
     * > **NOTE:**  This method won't copy all information over to the new
     * > document (acroforms, outlines, etc...).
     *
     * @returns Resolves with a copy this document.
     */
    copy(): Promise<PDFDocument>;
    /**
     * Add JavaScript to this document. The supplied `script` is executed when the
     * document is opened. The `script` can be used to perform some operation
     * when the document is opened (e.g. logging to the console), or it can be
     * used to define a function that can be referenced later in a JavaScript
     * action. For example:
     * ```js
     * // Show "Hello World!" in the console when the PDF is opened
     * pdfDoc.addJavaScript(
     *   'main',
     *   'console.show(); console.println("Hello World!");'
     * );
     *
     * // Define a function named "foo" that can be called in JavaScript Actions
     * pdfDoc.addJavaScript(
     *   'foo',
     *   'function foo() { return "foo"; }'
     * );
     * ```
     * See the [JavaScript for Acrobat API Reference](https://www.adobe.com/content/dam/acom/en/devnet/acrobat/pdfs/js_api_reference.pdf)
     * for details.
     * @param name The name of the script. Must be unique per document.
     * @param script The JavaScript to execute.
     */
    addJavaScript(name: string, script: string): void;
    /**
     * Add an attachment to this document. Attachments are visible in the
     * "Attachments" panel of Adobe Acrobat and some other PDF readers. Any
     * type of file can be added as an attachment. This includes, but is not
     * limited to, `.png`, `.jpg`, `.pdf`, `.csv`, `.docx`, and `.xlsx` files.
     *
     * The input data can be provided in multiple formats:
     *
     * | Type          | Contents                                                       |
     * | ------------- | -------------------------------------------------------------- |
     * | `string`      | A base64 encoded string (or data URI) containing an attachment |
     * | `Uint8Array`  | The raw bytes of an attachment                                 |
     * | `ArrayBuffer` | The raw bytes of an attachment                                 |
     *
     * For example:
     * ```js
     * // attachment=string
     * await pdfDoc.attach('/9j/4AAQSkZJRgABAQAAAQABAAD/2wBD...', 'cat_riding_unicorn.jpg', {
     *   mimeType: 'image/jpeg',
     *   description: 'Cool cat riding a unicorn! 🦄🐈🕶️',
     *   creationDate: new Date('2019/12/01'),
     *   modificationDate: new Date('2020/04/19'),
     * })
     * await pdfDoc.attach('data:image/jpeg;base64,/9j/4AAQ...', 'cat_riding_unicorn.jpg', {
     *   mimeType: 'image/jpeg',
     *   description: 'Cool cat riding a unicorn! 🦄🐈🕶️',
     *   creationDate: new Date('2019/12/01'),
     *   modificationDate: new Date('2020/04/19'),
     * })
     *
     * // attachment=Uint8Array
     * import fs from 'fs'
     * const uint8Array = fs.readFileSync('cat_riding_unicorn.jpg')
     * await pdfDoc.attach(uint8Array, 'cat_riding_unicorn.jpg', {
     *   mimeType: 'image/jpeg',
     *   description: 'Cool cat riding a unicorn! 🦄🐈🕶️',
     *   creationDate: new Date('2019/12/01'),
     *   modificationDate: new Date('2020/04/19'),
     * })
     *
     * // attachment=ArrayBuffer
     * const url = 'https://pdf-lib.js.org/assets/cat_riding_unicorn.jpg'
     * const arrayBuffer = await fetch(url).then(res => res.arrayBuffer())
     * await pdfDoc.attach(arrayBuffer, 'cat_riding_unicorn.jpg', {
     *   mimeType: 'image/jpeg',
     *   description: 'Cool cat riding a unicorn! 🦄🐈🕶️',
     *   creationDate: new Date('2019/12/01'),
     *   modificationDate: new Date('2020/04/19'),
     * })
     * ```
     *
     * @param attachment The input data containing the file to be attached.
     * @param name The name of the file to be attached.
     * @returns Resolves when the attachment is complete.
     */
    attach(attachment: string | Uint8Array | ArrayBuffer, name: string, options?: AttachmentOptions): Promise<void>;
    /**
     * Embed a font into this document. The input data can be provided in multiple
     * formats:
     *
     * | Type            | Contents                                                |
     * | --------------- | ------------------------------------------------------- |
     * | `StandardFonts` | One of the standard 14 fonts                            |
     * | `string`        | A base64 encoded string (or data URI) containing a font |
     * | `Uint8Array`    | The raw bytes of a font                                 |
     * | `ArrayBuffer`   | The raw bytes of a font                                 |
     *
     * For example:
     * ```js
     * // font=StandardFonts
     * import { StandardFonts } from 'pdf-lib'
     * const font1 = await pdfDoc.embedFont(StandardFonts.Helvetica)
     *
     * // font=string
     * const font2 = await pdfDoc.embedFont('AAEAAAAVAQAABABQRFNJRx/upe...')
     * const font3 = await pdfDoc.embedFont('data:font/opentype;base64,AAEAAA...')
     *
     * // font=Uint8Array
     * import fs from 'fs'
     * const font4 = await pdfDoc.embedFont(fs.readFileSync('Ubuntu-R.ttf'))
     *
     * // font=ArrayBuffer
     * const url = 'https://pdf-lib.js.org/assets/ubuntu/Ubuntu-R.ttf'
     * const ubuntuBytes = await fetch(url).then(res => res.arrayBuffer())
     * const font5 = await pdfDoc.embedFont(ubuntuBytes)
     * ```
     * See also: [[registerFontkit]]
     * @param font The input data for a font.
     * @param options The options to be used when embedding the font.
     * @returns Resolves with the embedded font.
     */
    embedFont(font: StandardFonts | string | Uint8Array | ArrayBuffer, options?: EmbedFontOptions): Promise<PDFFont>;
    /**
     * Embed a standard font into this document.
     * For example:
     * ```js
     * import { StandardFonts } from 'pdf-lib'
     * const helveticaFont = pdfDoc.embedFont(StandardFonts.Helvetica)
     * ```
     * @param font The standard font to be embedded.
     * @param customName The name to be used when embedding the font.
     * @returns The embedded font.
     */
    embedStandardFont(font: StandardFonts, customName?: string): PDFFont;
    /**
     * Embed a JPEG image into this document. The input data can be provided in
     * multiple formats:
     *
     * | Type          | Contents                                                      |
     * | ------------- | ------------------------------------------------------------- |
     * | `string`      | A base64 encoded string (or data URI) containing a JPEG image |
     * | `Uint8Array`  | The raw bytes of a JPEG image                                 |
     * | `ArrayBuffer` | The raw bytes of a JPEG image                                 |
     *
     * For example:
     * ```js
     * // jpg=string
     * const image1 = await pdfDoc.embedJpg('/9j/4AAQSkZJRgABAQAAAQABAAD/2wBD...')
     * const image2 = await pdfDoc.embedJpg('data:image/jpeg;base64,/9j/4AAQ...')
     *
     * // jpg=Uint8Array
     * import fs from 'fs'
     * const uint8Array = fs.readFileSync('cat_riding_unicorn.jpg')
     * const image3 = await pdfDoc.embedJpg(uint8Array)
     *
     * // jpg=ArrayBuffer
     * const url = 'https://pdf-lib.js.org/assets/cat_riding_unicorn.jpg'
     * const arrayBuffer = await fetch(url).then(res => res.arrayBuffer())
     * const image4 = await pdfDoc.embedJpg(arrayBuffer)
     * ```
     *
     * @param jpg The input data for a JPEG image.
     * @returns Resolves with the embedded image.
     */
    embedJpg(jpg: string | Uint8Array | ArrayBuffer): Promise<PDFImage>;
    /**
     * Embed a PNG image into this document. The input data can be provided in
     * multiple formats:
     *
     * | Type          | Contents                                                     |
     * | ------------- | ------------------------------------------------------------ |
     * | `string`      | A base64 encoded string (or data URI) containing a PNG image |
     * | `Uint8Array`  | The raw bytes of a PNG image                                 |
     * | `ArrayBuffer` | The raw bytes of a PNG image                                 |
     *
     * For example:
     * ```js
     * // png=string
     * const image1 = await pdfDoc.embedPng('iVBORw0KGgoAAAANSUhEUgAAAlgAAAF3...')
     * const image2 = await pdfDoc.embedPng('data:image/png;base64,iVBORw0KGg...')
     *
     * // png=Uint8Array
     * import fs from 'fs'
     * const uint8Array = fs.readFileSync('small_mario.png')
     * const image3 = await pdfDoc.embedPng(uint8Array)
     *
     * // png=ArrayBuffer
     * const url = 'https://pdf-lib.js.org/assets/small_mario.png'
     * const arrayBuffer = await fetch(url).then(res => res.arrayBuffer())
     * const image4 = await pdfDoc.embedPng(arrayBuffer)
     * ```
     *
     * @param png The input data for a PNG image.
     * @returns Resolves with the embedded image.
     */
    embedPng(png: string | Uint8Array | ArrayBuffer): Promise<PDFImage>;
    /**
     * Embed one or more PDF pages into this document.
     *
     * For example:
     * ```js
     * const pdfDoc = await PDFDocument.create()
     *
     * const sourcePdfUrl = 'https://pdf-lib.js.org/assets/with_large_page_count.pdf'
     * const sourcePdf = await fetch(sourcePdfUrl).then((res) => res.arrayBuffer())
     *
     * // Embed page 74 of `sourcePdf` into `pdfDoc`
     * const [embeddedPage] = await pdfDoc.embedPdf(sourcePdf, [73])
     * ```
     *
     * See [[PDFDocument.load]] for examples of the allowed input data formats.
     *
     * @param pdf The input data containing a PDF document.
     * @param indices The indices of the pages that should be embedded.
     * @returns Resolves with an array of the embedded pages.
     */
    embedPdf(pdf: string | Uint8Array | ArrayBuffer | PDFDocument, indices?: number[]): Promise<PDFEmbeddedPage[]>;
    /**
     * Embed a single PDF page into this document.
     *
     * For example:
     * ```js
     * const pdfDoc = await PDFDocument.create()
     *
     * const sourcePdfUrl = 'https://pdf-lib.js.org/assets/with_large_page_count.pdf'
     * const sourceBuffer = await fetch(sourcePdfUrl).then((res) => res.arrayBuffer())
     * const sourcePdfDoc = await PDFDocument.load(sourceBuffer)
     * const sourcePdfPage = sourcePdfDoc.getPages()[73]
     *
     * const embeddedPage = await pdfDoc.embedPage(
     *   sourcePdfPage,
     *
     *   // Clip a section of the source page so that we only embed part of it
     *   { left: 100, right: 450, bottom: 330, top: 570 },
     *
     *   // Translate all drawings of the embedded page by (10, 200) units
     *   [1, 0, 0, 1, 10, 200],
     * )
     * ```
     *
     * @param page The page to be embedded.
     * @param boundingBox
     * Optionally, an area of the source page that should be embedded
     * (defaults to entire page).
     * @param transformationMatrix
     * Optionally, a transformation matrix that is always applied to the embedded
     * page anywhere it is drawn.
     * @returns Resolves with the embedded pdf page.
     */
    embedPage(page: PDFPage, boundingBox?: PageBoundingBox, transformationMatrix?: TransformationMatrix): Promise<PDFEmbeddedPage>;
    /**
     * Embed one or more PDF pages into this document.
     *
     * For example:
     * ```js
     * const pdfDoc = await PDFDocument.create()
     *
     * const sourcePdfUrl = 'https://pdf-lib.js.org/assets/with_large_page_count.pdf'
     * const sourceBuffer = await fetch(sourcePdfUrl).then((res) => res.arrayBuffer())
     * const sourcePdfDoc = await PDFDocument.load(sourceBuffer)
     *
     * const page1 = sourcePdfDoc.getPages()[0]
     * const page2 = sourcePdfDoc.getPages()[52]
     * const page3 = sourcePdfDoc.getPages()[73]
     *
     * const embeddedPages = await pdfDoc.embedPages([page1, page2, page3])
     * ```
     *
     * @param page
     * The pages to be embedded (they must all share the same context).
     * @param boundingBoxes
     * Optionally, an array of clipping boundaries - one for each page
     * (defaults to entirety of each page).
     * @param transformationMatrices
     * Optionally, an array of transformation matrices - one for each page
     * (each page's transformation will apply anywhere it is drawn).
     * @returns Resolves with an array of the embedded pdf pages.
     */
    embedPages(pages: PDFPage[], boundingBoxes?: (PageBoundingBox | undefined)[], transformationMatrices?: (TransformationMatrix | undefined)[]): Promise<PDFEmbeddedPage[]>;
    /**
     * > **NOTE:** You shouldn't need to call this method directly. The [[save]]
     * > and [[saveAsBase64]] methods will automatically ensure that all embedded
     * > assets are flushed before serializing the document.
     *
     * Flush all embedded fonts, PDF pages, and images to this document's
     * [[context]].
     *
     * @returns Resolves when the flush is complete.
     */
    flush(): Promise<void>;
    /**
     * Serialize this document to an array of bytes making up a PDF file.
     * For example:
     * ```js
     * const pdfBytes = await pdfDoc.save()
     * ```
     *
     * There are a number of things you can do with the serialized document,
     * depending on the JavaScript environment you're running in:
     * * Write it to a file in Node or React Native
     * * Download it as a Blob in the browser
     * * Render it in an `iframe`
     *
     * @param options The options to be used when saving the document.
     * @returns Resolves with the bytes of the serialized document.
     */
    save(options?: SaveOptions): Promise<Uint8Array>;
    /**
     * Serialize this document to a base64 encoded string or data URI making up a
     * PDF file. For example:
     * ```js
     * const base64String = await pdfDoc.saveAsBase64()
     * base64String // => 'JVBERi0xLjcKJYGBgYEKC...'
     *
     * const base64DataUri = await pdfDoc.saveAsBase64({ dataUri: true })
     * base64DataUri // => 'data:application/pdf;base64,JVBERi0xLjcKJYGBgYEKC...'
     * ```
     *
     * @param options The options to be used when saving the document.
     * @returns Resolves with a base64 encoded string or data URI of the
     *          serialized document.
     */
    saveAsBase64(options?: Base64SaveOptions): Promise<string>;
    findPageForAnnotationRef(ref: PDFRef): PDFPage | undefined;
    private embedAll;
    private updateInfoDict;
    private getInfoDict;
    private assertFontkit;
    private computePages;
    private getOrCreateForm;
}
//# sourceMappingURL=PDFDocument.d.ts.map
```

### es/api/PDFDocumentOptions.d.ts

```ts
import { EmbeddedFileOptions } from "../core/embedders/FileEmbedder";
import { TypeFeatures } from "../types/fontkit";
export declare enum ParseSpeeds {
    Fastest = Infinity,
    Fast = 1500,
    Medium = 500,
    Slow = 100
}
export interface AttachmentOptions extends EmbeddedFileOptions {
}
export interface SaveOptions {
    useObjectStreams?: boolean;
    addDefaultPage?: boolean;
    objectsPerTick?: number;
    updateFieldAppearances?: boolean;
}
export interface Base64SaveOptions extends SaveOptions {
    dataUri?: boolean;
}
export interface LoadOptions {
    ignoreEncryption?: boolean;
    parseSpeed?: ParseSpeeds | number;
    throwOnInvalidObject?: boolean;
    updateMetadata?: boolean;
    capNumbers?: boolean;
}
export interface CreateOptions {
    updateMetadata?: boolean;
}
export interface EmbedFontOptions {
    subset?: boolean;
    customName?: string;
    features?: TypeFeatures;
}
export interface SetTitleOptions {
    showInWindowTitleBar: boolean;
}
//# sourceMappingURL=PDFDocumentOptions.d.ts.map
```

### es/api/PDFEmbeddedFile.d.ts

```ts
import Embeddable from "./Embeddable";
import PDFDocument from "./PDFDocument";
import FileEmbedder from "../core/embedders/FileEmbedder";
import { PDFRef } from "../core";
/**
 * Represents a file that has been embedded in a [[PDFDocument]].
 */
export default class PDFEmbeddedFile implements Embeddable {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFDocument.attach]] method, which will create
     * instances of [[PDFEmbeddedFile]] for you.
     *
     * Create an instance of [[PDFEmbeddedFile]] from an existing ref and embedder
     *
     * @param ref The unique reference for this file.
     * @param doc The document to which the file will belong.
     * @param embedder The embedder that will be used to embed the file.
     */
    static of: (ref: PDFRef, doc: PDFDocument, embedder: FileEmbedder) => PDFEmbeddedFile;
    /** The unique reference assigned to this embedded file within the document. */
    readonly ref: PDFRef;
    /** The document to which this embedded file belongs. */
    readonly doc: PDFDocument;
    private alreadyEmbedded;
    private readonly embedder;
    private constructor();
    /**
     * > **NOTE:** You probably don't need to call this method directly. The
     * > [[PDFDocument.save]] and [[PDFDocument.saveAsBase64]] methods will
     * > automatically ensure all embeddable files get embedded.
     *
     * Embed this embeddable file in its document.
     *
     * @returns Resolves when the embedding is complete.
     */
    embed(): Promise<void>;
}
//# sourceMappingURL=PDFEmbeddedFile.d.ts.map
```

### es/api/PDFEmbeddedPage.d.ts

```ts
import Embeddable from "./Embeddable";
import PDFDocument from "./PDFDocument";
import { PDFPageEmbedder, PDFRef } from "../core";
/**
 * Represents a PDF page that has been embedded in a [[PDFDocument]].
 */
export default class PDFEmbeddedPage implements Embeddable {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFDocument.embedPdf]] and
     * > [[PDFDocument.embedPage]] methods, which will create instances of
     * > [[PDFEmbeddedPage]] for you.
     *
     * Create an instance of [[PDFEmbeddedPage]] from an existing ref and embedder
     *
     * @param ref The unique reference for this embedded page.
     * @param doc The document to which the embedded page will belong.
     * @param embedder The embedder that will be used to embed the page.
     */
    static of: (ref: PDFRef, doc: PDFDocument, embedder: PDFPageEmbedder) => PDFEmbeddedPage;
    /** The unique reference assigned to this embedded page within the document. */
    readonly ref: PDFRef;
    /** The document to which this embedded page belongs. */
    readonly doc: PDFDocument;
    /** The width of this page in pixels. */
    readonly width: number;
    /** The height of this page in pixels. */
    readonly height: number;
    private alreadyEmbedded;
    private readonly embedder;
    private constructor();
    /**
     * Compute the width and height of this page after being scaled by the
     * given `factor`. For example:
     * ```js
     * embeddedPage.width  // => 500
     * embeddedPage.height // => 250
     *
     * const scaled = embeddedPage.scale(0.5)
     * scaled.width  // => 250
     * scaled.height // => 125
     * ```
     * This operation is often useful before drawing a page with
     * [[PDFPage.drawPage]] to compute the `width` and `height` options.
     * @param factor The factor by which this page should be scaled.
     * @returns The width and height of the page after being scaled.
     */
    scale(factor: number): {
        width: number;
        height: number;
    };
    /**
     * Get the width and height of this page. For example:
     * ```js
     * const { width, height } = embeddedPage.size()
     * ```
     * @returns The width and height of the page.
     */
    size(): {
        width: number;
        height: number;
    };
    /**
     * > **NOTE:** You probably don't need to call this method directly. The
     * > [[PDFDocument.save]] and [[PDFDocument.saveAsBase64]] methods will
     * > automatically ensure all embeddable pages get embedded.
     *
     * Embed this embeddable page in its document.
     *
     * @returns Resolves when the embedding is complete.
     */
    embed(): Promise<void>;
}
//# sourceMappingURL=PDFEmbeddedPage.d.ts.map
```

### es/api/PDFFont.d.ts

```ts
import Embeddable from "./Embeddable";
import PDFDocument from "./PDFDocument";
import { CustomFontEmbedder, PDFHexString, PDFRef, StandardFontEmbedder } from "../core";
export declare type FontEmbedder = CustomFontEmbedder | StandardFontEmbedder;
/**
 * Represents a font that has been embedded in a [[PDFDocument]].
 */
export default class PDFFont implements Embeddable {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFDocument.embedFont]] and
     * > [[PDFDocument.embedStandardFont]] methods, which will create instances
     * > of [[PDFFont]] for you.
     *
     * Create an instance of [[PDFFont]] from an existing ref and embedder
     *
     * @param ref The unique reference for this font.
     * @param doc The document to which the font will belong.
     * @param embedder The embedder that will be used to embed the font.
     */
    static of: (ref: PDFRef, doc: PDFDocument, embedder: FontEmbedder) => PDFFont;
    /** The unique reference assigned to this font within the document. */
    readonly ref: PDFRef;
    /** The document to which this font belongs. */
    readonly doc: PDFDocument;
    /** The name of this font. */
    readonly name: string;
    private modified;
    private readonly embedder;
    private constructor();
    /**
     * > **NOTE:** You probably don't need to call this method directly. The
     * > [[PDFPage.drawText]] method will automatically encode the text it is
     * > given.
     *
     * Encodes a string of text in this font.
     *
     * @param text The text to be encoded.
     * @returns The encoded text as a hex string.
     */
    encodeText(text: string): PDFHexString;
    /**
     * Measure the width of a string of text drawn in this font at a given size.
     * For example:
     * ```js
     * const width = font.widthOfTextAtSize('Foo Bar Qux Baz', 36)
     * ```
     * @param text The string of text to be measured.
     * @param size The font size to be used for this measurement.
     * @returns The width of the string of text when drawn in this font at the
     *          given size.
     */
    widthOfTextAtSize(text: string, size: number): number;
    /**
     * Measure the height of this font at a given size. For example:
     * ```js
     * const height = font.heightAtSize(24)
     * ```
     *
     * The `options.descender` value controls whether or not the font's
     * descender is included in the height calculation.
     *
     * @param size The font size to be used for this measurement.
     * @param options The options to be used when computing this measurement.
     * @returns The height of this font at the given size.
     */
    heightAtSize(size: number, options?: {
        descender?: boolean;
    }): number;
    /**
     * Compute the font size at which this font is a given height. For example:
     * ```js
     * const fontSize = font.sizeAtHeight(12)
     * ```
     * @param height The height to be used for this calculation.
     * @returns The font size at which this font is the given height.
     */
    sizeAtHeight(height: number): number;
    /**
     * Get the set of unicode code points that can be represented by this font.
     * @returns The set of unicode code points supported by this font.
     */
    getCharacterSet(): number[];
    /**
     * > **NOTE:** You probably don't need to call this method directly. The
     * > [[PDFDocument.save]] and [[PDFDocument.saveAsBase64]] methods will
     * > automatically ensure all fonts get embedded.
     *
     * Embed this font in its document.
     *
     * @returns Resolves when the embedding is complete.
     */
    embed(): Promise<void>;
}
//# sourceMappingURL=PDFFont.d.ts.map
```

### es/api/PDFImage.d.ts

```ts
import Embeddable from "./Embeddable";
import PDFDocument from "./PDFDocument";
import { JpegEmbedder, PDFRef, PngEmbedder } from "../core";
export declare type ImageEmbedder = JpegEmbedder | PngEmbedder;
/**
 * Represents an image that has been embedded in a [[PDFDocument]].
 */
export default class PDFImage implements Embeddable {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFDocument.embedPng]] and [[PDFDocument.embedJpg]]
     * > methods, which will create instances of [[PDFImage]] for you.
     *
     * Create an instance of [[PDFImage]] from an existing ref and embedder
     *
     * @param ref The unique reference for this image.
     * @param doc The document to which the image will belong.
     * @param embedder The embedder that will be used to embed the image.
     */
    static of: (ref: PDFRef, doc: PDFDocument, embedder: ImageEmbedder) => PDFImage;
    /** The unique reference assigned to this image within the document. */
    readonly ref: PDFRef;
    /** The document to which this image belongs. */
    readonly doc: PDFDocument;
    /** The width of this image in pixels. */
    readonly width: number;
    /** The height of this image in pixels. */
    readonly height: number;
    private embedder;
    private embedTask;
    private constructor();
    /**
     * Compute the width and height of this image after being scaled by the
     * given `factor`. For example:
     * ```js
     * image.width  // => 500
     * image.height // => 250
     *
     * const scaled = image.scale(0.5)
     * scaled.width  // => 250
     * scaled.height // => 125
     * ```
     * This operation is often useful before drawing an image with
     * [[PDFPage.drawImage]] to compute the `width` and `height` options.
     * @param factor The factor by which this image should be scaled.
     * @returns The width and height of the image after being scaled.
     */
    scale(factor: number): {
        width: number;
        height: number;
    };
    /**
     * Get the width and height of this image after scaling it as large as
     * possible while maintaining its aspect ratio and not exceeding the
     * specified `width` and `height`. For example:
     * ```
     * image.width  // => 500
     * image.height // => 250
     *
     * const scaled = image.scaleToFit(750, 1000)
     * scaled.width  // => 750
     * scaled.height // => 375
     * ```
     * The `width` and `height` parameters can also be thought of as the width
     * and height of a box that the scaled image must fit within.
     * @param width The bounding box's width.
     * @param height The bounding box's height.
     * @returns The width and height of the image after being scaled.
     */
    scaleToFit(width: number, height: number): {
        width: number;
        height: number;
    };
    /**
     * Get the width and height of this image. For example:
     * ```js
     * const { width, height } = image.size()
     * ```
     * @returns The width and height of the image.
     */
    size(): {
        width: number;
        height: number;
    };
    /**
     * > **NOTE:** You probably don't need to call this method directly. The
     * > [[PDFDocument.save]] and [[PDFDocument.saveAsBase64]] methods will
     * > automatically ensure all images get embedded.
     *
     * Embed this image in its document.
     *
     * @returns Resolves when the embedding is complete.
     */
    embed(): Promise<void>;
}
//# sourceMappingURL=PDFImage.d.ts.map
```

### es/api/PDFJavaScript.d.ts

```ts
import Embeddable from "./Embeddable";
import PDFDocument from "./PDFDocument";
import JavaScriptEmbedder from "../core/embedders/JavaScriptEmbedder";
import { PDFRef } from "../core";
/**
 * Represents JavaScript that has been embedded in a [[PDFDocument]].
 */
export default class PDFJavaScript implements Embeddable {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFDocument.addJavaScript]] method, which will
     * create instances of [[PDFJavaScript]] for you.
     *
     * Create an instance of [[PDFJavaScript]] from an existing ref and script
     *
     * @param ref The unique reference for this script.
     * @param doc The document to which the script will belong.
     * @param embedder The embedder that will be used to embed the script.
     */
    static of: (ref: PDFRef, doc: PDFDocument, embedder: JavaScriptEmbedder) => PDFJavaScript;
    /** The unique reference assigned to this embedded script within the document. */
    readonly ref: PDFRef;
    /** The document to which this embedded script belongs. */
    readonly doc: PDFDocument;
    private alreadyEmbedded;
    private readonly embedder;
    private constructor();
    /**
     * > **NOTE:** You probably don't need to call this method directly. The
     * > [[PDFDocument.save]] and [[PDFDocument.saveAsBase64]] methods will
     * > automatically ensure all JavaScripts get embedded.
     *
     * Embed this JavaScript in its document.
     *
     * @returns Resolves when the embedding is complete.
     */
    embed(): Promise<void>;
}
//# sourceMappingURL=PDFJavaScript.d.ts.map
```

### es/api/PDFPage.d.ts

```ts
import { Color } from "./colors";
import PDFDocument from "./PDFDocument";
import PDFEmbeddedPage from "./PDFEmbeddedPage";
import PDFFont from "./PDFFont";
import PDFImage from "./PDFImage";
import { PDFPageDrawCircleOptions, PDFPageDrawEllipseOptions, PDFPageDrawImageOptions, PDFPageDrawLineOptions, PDFPageDrawPageOptions, PDFPageDrawRectangleOptions, PDFPageDrawSquareOptions, PDFPageDrawSVGOptions, PDFPageDrawTextOptions } from "./PDFPageOptions";
import { Rotation } from "./rotations";
import { PDFOperator, PDFPageLeaf, PDFRef } from "../core";
/**
 * Represents a single page of a [[PDFDocument]].
 */
export default class PDFPage {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFDocument.addPage]] and [[PDFDocument.insertPage]]
     * > methods, which can create instances of [[PDFPage]] for you.
     *
     * Create an instance of [[PDFPage]] from an existing leaf node.
     *
     * @param leafNode The leaf node to be wrapped.
     * @param ref The unique reference for the page.
     * @param doc The document to which the page will belong.
     */
    static of: (leafNode: PDFPageLeaf, ref: PDFRef, doc: PDFDocument) => PDFPage;
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFDocument.addPage]] and [[PDFDocument.insertPage]]
     * > methods, which can create instances of [[PDFPage]] for you.
     *
     * Create an instance of [[PDFPage]].
     *
     * @param doc The document to which the page will belong.
     */
    static create: (doc: PDFDocument) => PDFPage;
    /** The low-level PDFDictionary wrapped by this page. */
    readonly node: PDFPageLeaf;
    /** The unique reference assigned to this page within the document. */
    readonly ref: PDFRef;
    /** The document to which this page belongs. */
    readonly doc: PDFDocument;
    private fontKey?;
    private font?;
    private fontSize;
    private fontColor;
    private lineHeight;
    private x;
    private y;
    private contentStream?;
    private contentStreamRef?;
    private constructor();
    /**
     * Rotate this page by a multiple of 90 degrees. For example:
     * ```js
     * import { degrees } from 'pdf-lib'
     *
     * page.setRotation(degrees(-90))
     * page.setRotation(degrees(0))
     * page.setRotation(degrees(90))
     * page.setRotation(degrees(180))
     * page.setRotation(degrees(270))
     * ```
     * @param angle The angle to rotate this page.
     */
    setRotation(angle: Rotation): void;
    /**
     * Get this page's rotation angle in degrees. For example:
     * ```js
     * const rotationAngle = page.getRotation().angle;
     * ```
     * @returns The rotation angle of the page in degrees (always a multiple of
     *          90 degrees).
     */
    getRotation(): Rotation;
    /**
     * Resize this page by increasing or decreasing its width and height. For
     * example:
     * ```js
     * page.setSize(250, 500)
     * page.setSize(page.getWidth() + 50, page.getHeight() + 100)
     * page.setSize(page.getWidth() - 50, page.getHeight() - 100)
     * ```
     *
     * Note that the PDF specification does not allow for pages to have explicit
     * widths and heights. Instead it defines the "size" of a page in terms of
     * five rectangles: the MediaBox, CropBox, BleedBox, TrimBox, and ArtBox. As a
     * result, this method cannot directly change the width and height of a page.
     * Instead, it works by adjusting these five boxes.
     *
     * This method performs the following steps:
     *   1. Set width & height of MediaBox.
     *   2. Set width & height of CropBox, if it has same dimensions as MediaBox.
     *   3. Set width & height of BleedBox, if it has same dimensions as MediaBox.
     *   4. Set width & height of TrimBox, if it has same dimensions as MediaBox.
     *   5. Set width & height of ArtBox, if it has same dimensions as MediaBox.
     *
     * This approach works well for most PDF documents as all PDF pages must
     * have a MediaBox, but relatively few have a CropBox, BleedBox, TrimBox, or
     * ArtBox. And when they do have these additional boxes, they often have the
     * same dimensions as the MediaBox. However, if you find this method does not
     * work for your document, consider setting the boxes directly:
     *   * [[PDFPage.setMediaBox]]
     *   * [[PDFPage.setCropBox]]
     *   * [[PDFPage.setBleedBox]]
     *   * [[PDFPage.setTrimBox]]
     *   * [[PDFPage.setArtBox]]
     *
     * @param width The new width of the page.
     * @param height The new height of the page.
     */
    setSize(width: number, height: number): void;
    /**
     * Resize this page by increasing or decreasing its width. For example:
     * ```js
     * page.setWidth(250)
     * page.setWidth(page.getWidth() + 50)
     * page.setWidth(page.getWidth() - 50)
     * ```
     *
     * This method uses [[PDFPage.setSize]] to set the page's width.
     *
     * @param width The new width of the page.
     */
    setWidth(width: number): void;
    /**
     * Resize this page by increasing or decreasing its height. For example:
     * ```js
     * page.setHeight(500)
     * page.setHeight(page.getWidth() + 100)
     * page.setHeight(page.getWidth() - 100)
     * ```
     *
     * This method uses [[PDFPage.setSize]] to set the page's height.
     *
     * @param height The new height of the page.
     */
    setHeight(height: number): void;
    /**
     * Set the MediaBox of this page. For example:
     * ```js
     * const mediaBox = page.getMediaBox()
     *
     * page.setMediaBox(0, 0, 250, 500)
     * page.setMediaBox(mediaBox.x, mediaBox.y, 50, 100)
     * page.setMediaBox(15, 5, mediaBox.width - 50, mediaBox.height - 100)
     * ```
     *
     * See [[PDFPage.getMediaBox]] for details about what the MediaBox represents.
     *
     * @param x The x coordinate of the lower left corner of the new MediaBox.
     * @param y The y coordinate of the lower left corner of the new MediaBox.
     * @param width The width of the new MediaBox.
     * @param height The height of the new MediaBox.
     */
    setMediaBox(x: number, y: number, width: number, height: number): void;
    /**
     * Set the CropBox of this page. For example:
     * ```js
     * const cropBox = page.getCropBox()
     *
     * page.setCropBox(0, 0, 250, 500)
     * page.setCropBox(cropBox.x, cropBox.y, 50, 100)
     * page.setCropBox(15, 5, cropBox.width - 50, cropBox.height - 100)
     * ```
     *
     * See [[PDFPage.getCropBox]] for details about what the CropBox represents.
     *
     * @param x The x coordinate of the lower left corner of the new CropBox.
     * @param y The y coordinate of the lower left corner of the new CropBox.
     * @param width The width of the new CropBox.
     * @param height The height of the new CropBox.
     */
    setCropBox(x: number, y: number, width: number, height: number): void;
    /**
     * Set the BleedBox of this page. For example:
     * ```js
     * const bleedBox = page.getBleedBox()
     *
     * page.setBleedBox(0, 0, 250, 500)
     * page.setBleedBox(bleedBox.x, bleedBox.y, 50, 100)
     * page.setBleedBox(15, 5, bleedBox.width - 50, bleedBox.height - 100)
     * ```
     *
     * See [[PDFPage.getBleedBox]] for details about what the BleedBox represents.
     *
     * @param x The x coordinate of the lower left corner of the new BleedBox.
     * @param y The y coordinate of the lower left corner of the new BleedBox.
     * @param width The width of the new BleedBox.
     * @param height The height of the new BleedBox.
     */
    setBleedBox(x: number, y: number, width: number, height: number): void;
    /**
     * Set the TrimBox of this page. For example:
     * ```js
     * const trimBox = page.getTrimBox()
     *
     * page.setTrimBox(0, 0, 250, 500)
     * page.setTrimBox(trimBox.x, trimBox.y, 50, 100)
     * page.setTrimBox(15, 5, trimBox.width - 50, trimBox.height - 100)
     * ```
     *
     * See [[PDFPage.getTrimBox]] for details about what the TrimBox represents.
     *
     * @param x The x coordinate of the lower left corner of the new TrimBox.
     * @param y The y coordinate of the lower left corner of the new TrimBox.
     * @param width The width of the new TrimBox.
     * @param height The height of the new TrimBox.
     */
    setTrimBox(x: number, y: number, width: number, height: number): void;
    /**
     * Set the ArtBox of this page. For example:
     * ```js
     * const artBox = page.getArtBox()
     *
     * page.setArtBox(0, 0, 250, 500)
     * page.setArtBox(artBox.x, artBox.y, 50, 100)
     * page.setArtBox(15, 5, artBox.width - 50, artBox.height - 100)
     * ```
     *
     * See [[PDFPage.getArtBox]] for details about what the ArtBox represents.
     *
     * @param x The x coordinate of the lower left corner of the new ArtBox.
     * @param y The y coordinate of the lower left corner of the new ArtBox.
     * @param width The width of the new ArtBox.
     * @param height The height of the new ArtBox.
     */
    setArtBox(x: number, y: number, width: number, height: number): void;
    /**
     * Get this page's width and height. For example:
     * ```js
     * const { width, height } = page.getSize()
     * ```
     *
     * This method uses [[PDFPage.getMediaBox]] to obtain the page's
     * width and height.
     *
     * @returns The width and height of the page.
     */
    getSize(): {
        width: number;
        height: number;
    };
    /**
     * Get this page's width. For example:
     * ```js
     * const width = page.getWidth()
     * ```
     *
     * This method uses [[PDFPage.getSize]] to obtain the page's size.
     *
     * @returns The width of the page.
     */
    getWidth(): number;
    /**
     * Get this page's height. For example:
     * ```js
     * const height = page.getHeight()
     * ```
     *
     * This method uses [[PDFPage.getSize]] to obtain the page's size.
     *
     * @returns The height of the page.
     */
    getHeight(): number;
    /**
     * Get the rectangle defining this page's MediaBox. For example:
     * ```js
     * const { x, y, width, height } = page.getMediaBox()
     * ```
     *
     * The MediaBox of a page defines the boundaries of the physical medium on
     * which the page is to be displayed/printed. It may include extended area
     * surrounding the page content for bleed marks, printing marks, etc...
     * It may also include areas close to the edges of the medium that cannot be
     * marked because of physical limitations of the output device. Content
     * falling outside this boundary may safely be discarded without affecting
     * the meaning of the PDF file.
     *
     * @returns An object defining the lower left corner of the MediaBox and its
     *          width & height.
     */
    getMediaBox(): {
        x: number;
        y: number;
        width: number;
        height: number;
    };
    /**
     * Get the rectangle defining this page's CropBox. For example:
     * ```js
     * const { x, y, width, height } = page.getCropBox()
     * ```
     *
     * The CropBox of a page defines the region to which the contents of the page
     * shall be clipped when displayed or printed. Unlike the other boxes, the
     * CropBox does not necessarily represent the physical page geometry. It
     * merely imposes clipping on the page contents.
     *
     * The CropBox's default value is the page's MediaBox.
     *
     * @returns An object defining the lower left corner of the CropBox and its
     *          width & height.
     */
    getCropBox(): {
        x: number;
        y: number;
        width: number;
        height: number;
    };
    /**
     * Get the rectangle defining this page's BleedBox. For example:
     * ```js
     * const { x, y, width, height } = page.getBleedBox()
     * ```
     *
     * The BleedBox of a page defines the region to which the contents of the
     * page shall be clipped when output in a production environment. This may
     * include any extra bleed area needed to accommodate the physical
     * limitations of cutting, folding, and trimming equipment. The actual
     * printed page may include printing marks that fall outside the BleedBox.
     *
     * The BleedBox's default value is the page's CropBox.
     *
     * @returns An object defining the lower left corner of the BleedBox and its
     *          width & height.
     */
    getBleedBox(): {
        x: number;
        y: number;
        width: number;
        height: number;
    };
    /**
     * Get the rectangle defining this page's TrimBox. For example:
     * ```js
     * const { x, y, width, height } = page.getTrimBox()
     * ```
     *
     * The TrimBox of a page defines the intended dimensions of the finished
     * page after trimming. It may be smaller than the MediaBox to allow for
     * production-related content, such as printing instructions, cut marks, or
     * color bars.
     *
     * The TrimBox's default value is the page's CropBox.
     *
     * @returns An object defining the lower left corner of the TrimBox and its
     *          width & height.
     */
    getTrimBox(): {
        x: number;
        y: number;
        width: number;
        height: number;
    };
    /**
     * Get the rectangle defining this page's ArtBox. For example:
     * ```js
     * const { x, y, width, height } = page.getArtBox()
     * ```
     *
     * The ArtBox of a page defines the extent of the page's meaningful content
     * (including potential white space).
     *
     * The ArtBox's default value is the page's CropBox.
     *
     * @returns An object defining the lower left corner of the ArtBox and its
     *          width & height.
     */
    getArtBox(): {
        x: number;
        y: number;
        width: number;
        height: number;
    };
    /**
     * Translate this page's content to a new location on the page. This operation
     * is often useful after resizing the page with [[setSize]]. For example:
     * ```js
     * // Add 50 units of whitespace to the top and right of the page
     * page.setSize(page.getWidth() + 50, page.getHeight() + 50)
     *
     * // Move the page's content from the lower-left corner of the page
     * // to the top-right corner.
     * page.translateContent(50, 50)
     *
     * // Now there are 50 units of whitespace to the left and bottom of the page
     * ```
     * See also: [[resetPosition]]
     * @param x The new position on the x-axis for this page's content.
     * @param y The new position on the y-axis for this page's content.
     */
    translateContent(x: number, y: number): void;
    /**
     * Scale the size, content, and annotations of a page.
     *
     * For example:
     * ```js
     * page.scale(0.5, 0.5);
     * ```
     *
     * @param x The factor by which the width for the page should be scaled
     *          (e.g. `0.5` is 50%).
     * @param y The factor by which the height for the page should be scaled
     *          (e.g. `2.0` is 200%).
     */
    scale(x: number, y: number): void;
    /**
     * Scale the content of a page. This is useful after resizing an existing
     * page. This scales only the content, not the annotations.
     *
     * For example:
     * ```js
     * // Bisect the size of the page
     * page.setSize(page.getWidth() / 2, page.getHeight() / 2);
     *
     * // Scale the content of the page down by 50% in x and y
     * page.scaleContent(0.5, 0.5);
     * ```
     * See also: [[scaleAnnotations]]
     * @param x The factor by which the x-axis for the content should be scaled
     *          (e.g. `0.5` is 50%).
     * @param y The factor by which the y-axis for the content should be scaled
     *          (e.g. `2.0` is 200%).
     */
    scaleContent(x: number, y: number): void;
    /**
     * Scale the annotations of a page. This is useful if you want to scale a
     * page with comments or other annotations.
     * ```js
     * // Scale the content of the page down by 50% in x and y
     * page.scaleContent(0.5, 0.5);
     *
     * // Scale the content of the page down by 50% in x and y
     * page.scaleAnnotations(0.5, 0.5);
     * ```
     * See also: [[scaleContent]]
     * @param x The factor by which the x-axis for the annotations should be
     *          scaled (e.g. `0.5` is 50%).
     * @param y The factor by which the y-axis for the annotations should be
     *          scaled (e.g. `2.0` is 200%).
     */
    scaleAnnotations(x: number, y: number): void;
    /**
     * Reset the x and y coordinates of this page to `(0, 0)`. This operation is
     * often useful after calling [[translateContent]]. For example:
     * ```js
     * // Shift the page's contents up and to the right by 50 units
     * page.translateContent(50, 50)
     *
     * // This text will shifted - it will be drawn at (50, 50)
     * page.drawText('I am shifted')
     *
     * // Move back to (0, 0)
     * page.resetPosition()
     *
     * // This text will not be shifted - it will be drawn at (0, 0)
     * page.drawText('I am not shifted')
     * ```
     */
    resetPosition(): void;
    /**
     * Choose a default font for this page. The default font will be used whenever
     * text is drawn on this page and no font is specified. For example:
     * ```js
     * import { StandardFonts } from 'pdf-lib'
     *
     * const timesRomanFont = await pdfDoc.embedFont(StandardFonts.TimesRoman)
     * const helveticaFont = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const courierFont = await pdfDoc.embedFont(StandardFonts.Courier)
     *
     * const page = pdfDoc.addPage()
     *
     * page.setFont(helveticaFont)
     * page.drawText('I will be drawn in Helvetica')
     *
     * page.setFont(timesRomanFont)
     * page.drawText('I will be drawn in Courier', { font: courierFont })
     * ```
     * @param font The default font to be used when drawing text on this page.
     */
    setFont(font: PDFFont): void;
    /**
     * Choose a default font size for this page. The default font size will be
     * used whenever text is drawn on this page and no font size is specified.
     * For example:
     * ```js
     * page.setFontSize(12)
     * page.drawText('I will be drawn in size 12')
     *
     * page.setFontSize(36)
     * page.drawText('I will be drawn in size 24', { fontSize: 24 })
     * ```
     * @param fontSize The default font size to be used when drawing text on this
     *                 page.
     */
    setFontSize(fontSize: number): void;
    /**
     * Choose a default font color for this page. The default font color will be
     * used whenever text is drawn on this page and no font color is specified.
     * For example:
     * ```js
     * import { rgb, cmyk, grayscale } from 'pdf-lib'
     *
     * page.setFontColor(rgb(0.97, 0.02, 0.97))
     * page.drawText('I will be drawn in pink')
     *
     * page.setFontColor(cmyk(0.4, 0.7, 0.39, 0.15))
     * page.drawText('I will be drawn in gray', { color: grayscale(0.5) })
     * ```
     * @param fontColor The default font color to be used when drawing text on
     *                  this page.
     */
    setFontColor(fontColor: Color): void;
    /**
     * Choose a default line height for this page. The default line height will be
     * used whenever text is drawn on this page and no line height is specified.
     * For example:
     * ```js
     * page.setLineHeight(12);
     * page.drawText('These lines will be vertically \n separated by 12 units')
     *
     * page.setLineHeight(36);
     * page.drawText('These lines will be vertically \n separated by 24 units', {
     *   lineHeight: 24
     * })
     * ```
     * @param lineHeight The default line height to be used when drawing text on
     *                   this page.
     */
    setLineHeight(lineHeight: number): void;
    /**
     * Get the default position of this page. For example:
     * ```js
     * const { x, y } = page.getPosition()
     * ```
     * @returns The default position of the page.
     */
    getPosition(): {
        x: number;
        y: number;
    };
    /**
     * Get the default x coordinate of this page. For example:
     * ```js
     * const x = page.getX()
     * ```
     * @returns The default x coordinate of the page.
     */
    getX(): number;
    /**
     * Get the default y coordinate of this page. For example:
     * ```js
     * const y = page.getY()
     * ```
     * @returns The default y coordinate of the page.
     */
    getY(): number;
    /**
     * Change the default position of this page. For example:
     * ```js
     * page.moveTo(0, 0)
     * page.drawText('I will be drawn at the origin')
     *
     * page.moveTo(0, 25)
     * page.drawText('I will be drawn 25 units up')
     *
     * page.moveTo(25, 25)
     * page.drawText('I will be drawn 25 units up and 25 units to the right')
     * ```
     * @param x The new default position on the x-axis for this page.
     * @param y The new default position on the y-axis for this page.
     */
    moveTo(x: number, y: number): void;
    /**
     * Change the default position of this page to be further down the y-axis.
     * For example:
     * ```js
     * page.moveTo(50, 50)
     * page.drawText('I will be drawn at (50, 50)')
     *
     * page.moveDown(10)
     * page.drawText('I will be drawn at (50, 40)')
     * ```
     * @param yDecrease The amount by which the page's default position along the
     *                  y-axis should be decreased.
     */
    moveDown(yDecrease: number): void;
    /**
     * Change the default position of this page to be further up the y-axis.
     * For example:
     * ```js
     * page.moveTo(50, 50)
     * page.drawText('I will be drawn at (50, 50)')
     *
     * page.moveUp(10)
     * page.drawText('I will be drawn at (50, 60)')
     * ```
     * @param yIncrease The amount by which the page's default position along the
     *                  y-axis should be increased.
     */
    moveUp(yIncrease: number): void;
    /**
     * Change the default position of this page to be further left on the x-axis.
     * For example:
     * ```js
     * page.moveTo(50, 50)
     * page.drawText('I will be drawn at (50, 50)')
     *
     * page.moveLeft(10)
     * page.drawText('I will be drawn at (40, 50)')
     * ```
     * @param xDecrease The amount by which the page's default position along the
     *                  x-axis should be decreased.
     */
    moveLeft(xDecrease: number): void;
    /**
     * Change the default position of this page to be further right on the y-axis.
     * For example:
     * ```js
     * page.moveTo(50, 50)
     * page.drawText('I will be drawn at (50, 50)')
     *
     * page.moveRight(10)
     * page.drawText('I will be drawn at (60, 50)')
     * ```
     * @param xIncrease The amount by which the page's default position along the
     *                  x-axis should be increased.
     */
    moveRight(xIncrease: number): void;
    /**
     * Push one or more operators to the end of this page's current content
     * stream. For example:
     * ```js
     * import {
     *   pushGraphicsState,
     *   moveTo,
     *   lineTo,
     *   closePath,
     *   setFillingColor,
     *   rgb,
     *   fill,
     *   popGraphicsState,
     * } from 'pdf-lib'
     *
     * // Draw a green triangle in the lower-left corner of the page
     * page.pushOperators(
     *   pushGraphicsState(),
     *   moveTo(0, 0),
     *   lineTo(100, 0),
     *   lineTo(50, 100),
     *   closePath(),
     *   setFillingColor(rgb(0.0, 1.0, 0.0)),
     *   fill(),
     *   popGraphicsState(),
     * )
     * ```
     * @param operator The operators to be pushed.
     */
    pushOperators(...operator: PDFOperator[]): void;
    /**
     * Draw one or more lines of text on this page. For example:
     * ```js
     * import { StandardFonts, rgb } from 'pdf-lib'
     *
     * const helveticaFont = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const timesRomanFont = await pdfDoc.embedFont(StandardFonts.TimesRoman)
     *
     * const page = pdfDoc.addPage()
     *
     * page.setFont(helveticaFont)
     *
     * page.moveTo(5, 200)
     * page.drawText('The Life of an Egg', { size: 36 })
     *
     * page.moveDown(36)
     * page.drawText('An Epic Tale of Woe', { size: 30 })
     *
     * page.drawText(
     *   `Humpty Dumpty sat on a wall \n` +
     *   `Humpty Dumpty had a great fall; \n` +
     *   `All the king's horses and all the king's men \n` +
     *   `Couldn't put Humpty together again. \n`,
     *   {
     *     x: 25,
     *     y: 100,
     *     font: timesRomanFont,
     *     size: 24,
     *     color: rgb(1, 0, 0),
     *     lineHeight: 24,
     *     opacity: 0.75,
     *   },
     * )
     * ```
     * @param text The text to be drawn.
     * @param options The options to be used when drawing the text.
     */
    drawText(text: string, options?: PDFPageDrawTextOptions): void;
    /**
     * Draw an image on this page. For example:
     * ```js
     * import { degrees } from 'pdf-lib'
     *
     * const jpgUrl = 'https://pdf-lib.js.org/assets/cat_riding_unicorn.jpg'
     * const jpgImageBytes = await fetch(jpgUrl).then((res) => res.arrayBuffer())
     *
     * const jpgImage = await pdfDoc.embedJpg(jpgImageBytes)
     * const jpgDims = jpgImage.scale(0.5)
     *
     * const page = pdfDoc.addPage()
     *
     * page.drawImage(jpgImage, {
     *   x: 25,
     *   y: 25,
     *   width: jpgDims.width,
     *   height: jpgDims.height,
     *   rotate: degrees(30),
     *   opacity: 0.75,
     * })
     * ```
     * @param image The image to be drawn.
     * @param options The options to be used when drawing the image.
     */
    drawImage(image: PDFImage, options?: PDFPageDrawImageOptions): void;
    /**
     * Draw an embedded PDF page on this page. For example:
     * ```js
     * import { degrees } from 'pdf-lib'
     *
     * const pdfDoc = await PDFDocument.create()
     * const page = pdfDoc.addPage()
     *
     * const sourcePdfUrl = 'https://pdf-lib.js.org/assets/with_large_page_count.pdf'
     * const sourcePdf = await fetch(sourcePdfUrl).then((res) => res.arrayBuffer())
     *
     * // Embed page 74 from the PDF
     * const [embeddedPage] = await pdfDoc.embedPdf(sourcePdf, 73)
     *
     * page.drawPage(embeddedPage, {
     *   x: 250,
     *   y: 200,
     *   xScale: 0.5,
     *   yScale: 0.5,
     *   rotate: degrees(30),
     *   opacity: 0.75,
     * })
     * ```
     *
     * The `options` argument accepts both `width`/`height` and `xScale`/`yScale`
     * as options. Since each of these options defines the size of the drawn page,
     * if both options are given, `width` and `height` take precedence and the
     * corresponding scale variants are ignored.
     *
     * @param embeddedPage The embedded page to be drawn.
     * @param options The options to be used when drawing the embedded page.
     */
    drawPage(embeddedPage: PDFEmbeddedPage, options?: PDFPageDrawPageOptions): void;
    /**
     * Draw an SVG path on this page. For example:
     * ```js
     * import { rgb } from 'pdf-lib'
     *
     * const svgPath = 'M 0,20 L 100,160 Q 130,200 150,120 C 190,-40 200,200 300,150 L 400,90'
     *
     * // Draw path as black line
     * page.drawSvgPath(svgPath, { x: 25, y: 75 })
     *
     * // Change border style and opacity
     * page.drawSvgPath(svgPath, {
     *   x: 25,
     *   y: 275,
     *   borderColor: rgb(0.5, 0.5, 0.5),
     *   borderWidth: 2,
     *   borderOpacity: 0.75,
     * })
     *
     * // Set fill color and opacity
     * page.drawSvgPath(svgPath, {
     *   x: 25,
     *   y: 475,
     *   color: rgb(1.0, 0, 0),
     *   opacity: 0.75,
     * })
     *
     * // Draw 50% of original size
     * page.drawSvgPath(svgPath, {
     *   x: 25,
     *   y: 675,
     *   scale: 0.5,
     * })
     * ```
     * @param path The SVG path to be drawn.
     * @param options The options to be used when drawing the SVG path.
     */
    drawSvgPath(path: string, options?: PDFPageDrawSVGOptions): void;
    /**
     * Draw a line on this page. For example:
     * ```js
     * import { rgb } from 'pdf-lib'
     *
     * page.drawLine({
     *   start: { x: 25, y: 75 },
     *   end: { x: 125, y: 175 },
     *   thickness: 2,
     *   color: rgb(0.75, 0.2, 0.2),
     *   opacity: 0.75,
     * })
     * ```
     * @param options The options to be used when drawing the line.
     */
    drawLine(options: PDFPageDrawLineOptions): void;
    /**
     * Draw a rectangle on this page. For example:
     * ```js
     * import { degrees, grayscale, rgb } from 'pdf-lib'
     *
     * page.drawRectangle({
     *   x: 25,
     *   y: 75,
     *   width: 250,
     *   height: 75,
     *   rotate: degrees(-15),
     *   borderWidth: 5,
     *   borderColor: grayscale(0.5),
     *   color: rgb(0.75, 0.2, 0.2),
     *   opacity: 0.5,
     *   borderOpacity: 0.75,
     * })
     * ```
     * @param options The options to be used when drawing the rectangle.
     */
    drawRectangle(options?: PDFPageDrawRectangleOptions): void;
    /**
     * Draw a square on this page. For example:
     * ```js
     * import { degrees, grayscale, rgb } from 'pdf-lib'
     *
     * page.drawSquare({
     *   x: 25,
     *   y: 75,
     *   size: 100,
     *   rotate: degrees(-15),
     *   borderWidth: 5,
     *   borderColor: grayscale(0.5),
     *   color: rgb(0.75, 0.2, 0.2),
     *   opacity: 0.5,
     *   borderOpacity: 0.75,
     * })
     * ```
     * @param options The options to be used when drawing the square.
     */
    drawSquare(options?: PDFPageDrawSquareOptions): void;
    /**
     * Draw an ellipse on this page. For example:
     * ```js
     * import { grayscale, rgb } from 'pdf-lib'
     *
     * page.drawEllipse({
     *   x: 200,
     *   y: 75,
     *   xScale: 100,
     *   yScale: 50,
     *   borderWidth: 5,
     *   borderColor: grayscale(0.5),
     *   color: rgb(0.75, 0.2, 0.2),
     *   opacity: 0.5,
     *   borderOpacity: 0.75,
     * })
     * ```
     * @param options The options to be used when drawing the ellipse.
     */
    drawEllipse(options?: PDFPageDrawEllipseOptions): void;
    /**
     * Draw a circle on this page. For example:
     * ```js
     * import { grayscale, rgb } from 'pdf-lib'
     *
     * page.drawCircle({
     *   x: 200,
     *   y: 150,
     *   size: 100,
     *   borderWidth: 5,
     *   borderColor: grayscale(0.5),
     *   color: rgb(0.75, 0.2, 0.2),
     *   opacity: 0.5,
     *   borderOpacity: 0.75,
     * })
     * ```
     * @param options The options to be used when drawing the ellipse.
     */
    drawCircle(options?: PDFPageDrawCircleOptions): void;
    private setOrEmbedFont;
    private getFont;
    private resetFont;
    private getContentStream;
    private createContentStream;
    private maybeEmbedGraphicsState;
    private scaleAnnot;
}
//# sourceMappingURL=PDFPage.d.ts.map
```

### es/api/PDFPageOptions.d.ts

```ts
import { Color } from "./colors";
import PDFFont from "./PDFFont";
import { Rotation } from "./rotations";
import { LineCapStyle } from "./operators";
export declare enum BlendMode {
    Normal = "Normal",
    Multiply = "Multiply",
    Screen = "Screen",
    Overlay = "Overlay",
    Darken = "Darken",
    Lighten = "Lighten",
    ColorDodge = "ColorDodge",
    ColorBurn = "ColorBurn",
    HardLight = "HardLight",
    SoftLight = "SoftLight",
    Difference = "Difference",
    Exclusion = "Exclusion"
}
export interface PDFPageDrawTextOptions {
    color?: Color;
    opacity?: number;
    blendMode?: BlendMode;
    font?: PDFFont;
    size?: number;
    rotate?: Rotation;
    xSkew?: Rotation;
    ySkew?: Rotation;
    x?: number;
    y?: number;
    lineHeight?: number;
    maxWidth?: number;
    wordBreaks?: string[];
}
export interface PDFPageDrawImageOptions {
    x?: number;
    y?: number;
    width?: number;
    height?: number;
    rotate?: Rotation;
    xSkew?: Rotation;
    ySkew?: Rotation;
    opacity?: number;
    blendMode?: BlendMode;
}
export interface PDFPageDrawPageOptions {
    x?: number;
    y?: number;
    xScale?: number;
    yScale?: number;
    width?: number;
    height?: number;
    rotate?: Rotation;
    xSkew?: Rotation;
    ySkew?: Rotation;
    opacity?: number;
    blendMode?: BlendMode;
}
export interface PDFPageDrawSVGOptions {
    x?: number;
    y?: number;
    scale?: number;
    rotate?: Rotation;
    borderWidth?: number;
    color?: Color;
    opacity?: number;
    borderColor?: Color;
    borderOpacity?: number;
    borderDashArray?: number[];
    borderDashPhase?: number;
    borderLineCap?: LineCapStyle;
    blendMode?: BlendMode;
}
export interface PDFPageDrawLineOptions {
    start: {
        x: number;
        y: number;
    };
    end: {
        x: number;
        y: number;
    };
    thickness?: number;
    color?: Color;
    opacity?: number;
    lineCap?: LineCapStyle;
    dashArray?: number[];
    dashPhase?: number;
    blendMode?: BlendMode;
}
export interface PDFPageDrawRectangleOptions {
    x?: number;
    y?: number;
    width?: number;
    height?: number;
    rotate?: Rotation;
    xSkew?: Rotation;
    ySkew?: Rotation;
    borderWidth?: number;
    color?: Color;
    opacity?: number;
    borderColor?: Color;
    borderOpacity?: number;
    borderDashArray?: number[];
    borderDashPhase?: number;
    borderLineCap?: LineCapStyle;
    blendMode?: BlendMode;
}
export interface PDFPageDrawSquareOptions {
    x?: number;
    y?: number;
    size?: number;
    rotate?: Rotation;
    xSkew?: Rotation;
    ySkew?: Rotation;
    borderWidth?: number;
    color?: Color;
    opacity?: number;
    borderColor?: Color;
    borderOpacity?: number;
    borderDashArray?: number[];
    borderDashPhase?: number;
    borderLineCap?: LineCapStyle;
    blendMode?: BlendMode;
}
export interface PDFPageDrawEllipseOptions {
    x?: number;
    y?: number;
    xScale?: number;
    yScale?: number;
    rotate?: Rotation;
    color?: Color;
    opacity?: number;
    borderColor?: Color;
    borderOpacity?: number;
    borderWidth?: number;
    borderDashArray?: number[];
    borderDashPhase?: number;
    borderLineCap?: LineCapStyle;
    blendMode?: BlendMode;
}
export interface PDFPageDrawCircleOptions {
    x?: number;
    y?: number;
    size?: number;
    color?: Color;
    opacity?: number;
    borderColor?: Color;
    borderOpacity?: number;
    borderWidth?: number;
    borderDashArray?: number[];
    borderDashPhase?: number;
    borderLineCap?: LineCapStyle;
    blendMode?: BlendMode;
}
//# sourceMappingURL=PDFPageOptions.d.ts.map
```

### es/api/StandardFonts.d.ts

```ts
export declare enum StandardFonts {
    Courier = "Courier",
    CourierBold = "Courier-Bold",
    CourierOblique = "Courier-Oblique",
    CourierBoldOblique = "Courier-BoldOblique",
    Helvetica = "Helvetica",
    HelveticaBold = "Helvetica-Bold",
    HelveticaOblique = "Helvetica-Oblique",
    HelveticaBoldOblique = "Helvetica-BoldOblique",
    TimesRoman = "Times-Roman",
    TimesRomanBold = "Times-Bold",
    TimesRomanItalic = "Times-Italic",
    TimesRomanBoldItalic = "Times-BoldItalic",
    Symbol = "Symbol",
    ZapfDingbats = "ZapfDingbats"
}
//# sourceMappingURL=StandardFonts.d.ts.map
```

### es/api/colors.d.ts

```ts
export declare enum ColorTypes {
    Grayscale = "Grayscale",
    RGB = "RGB",
    CMYK = "CMYK"
}
export interface Grayscale {
    type: ColorTypes.Grayscale;
    gray: number;
}
export interface RGB {
    type: ColorTypes.RGB;
    red: number;
    green: number;
    blue: number;
}
export interface CMYK {
    type: ColorTypes.CMYK;
    cyan: number;
    magenta: number;
    yellow: number;
    key: number;
}
export declare type Color = Grayscale | RGB | CMYK;
export declare const grayscale: (gray: number) => Grayscale;
export declare const rgb: (red: number, green: number, blue: number) => RGB;
export declare const cmyk: (cyan: number, magenta: number, yellow: number, key: number) => CMYK;
export declare const setFillingColor: (color: Color) => import("../core/operators/PDFOperator").default;
export declare const setStrokingColor: (color: Color) => import("../core/operators/PDFOperator").default;
export declare const componentsToColor: (comps?: number[] | undefined, scale?: number) => Grayscale | RGB | CMYK | undefined;
export declare const colorToComponents: (color: Color) => number[];
//# sourceMappingURL=colors.d.ts.map
```

### es/api/errors.d.ts

```ts
export declare class EncryptedPDFError extends Error {
    constructor();
}
export declare class FontkitNotRegisteredError extends Error {
    constructor();
}
export declare class ForeignPageError extends Error {
    constructor();
}
export declare class RemovePageFromEmptyDocumentError extends Error {
    constructor();
}
export declare class NoSuchFieldError extends Error {
    constructor(name: string);
}
export declare class UnexpectedFieldTypeError extends Error {
    constructor(name: string, expected: any, actual: any);
}
export declare class MissingOnValueCheckError extends Error {
    constructor(onValue: any);
}
export declare class FieldAlreadyExistsError extends Error {
    constructor(name: string);
}
export declare class InvalidFieldNamePartError extends Error {
    constructor(namePart: string);
}
export declare class FieldExistsAsNonTerminalError extends Error {
    constructor(name: string);
}
export declare class RichTextFieldReadError extends Error {
    constructor(fieldName: string);
}
export declare class CombedTextLayoutError extends Error {
    constructor(lineLength: number, cellCount: number);
}
export declare class ExceededMaxLengthError extends Error {
    constructor(textLength: number, maxLength: number, name: string);
}
export declare class InvalidMaxLengthError extends Error {
    constructor(textLength: number, maxLength: number, name: string);
}
//# sourceMappingURL=errors.d.ts.map
```

### es/api/form/PDFButton.d.ts

```ts
import PDFDocument from "../PDFDocument";
import PDFPage from "../PDFPage";
import PDFFont from "../PDFFont";
import PDFImage from "../PDFImage";
import { ImageAlignment } from "../image/alignment";
import { AppearanceProviderFor } from "./appearances";
import PDFField, { FieldAppearanceOptions } from "./PDFField";
import { PDFRef, PDFAcroPushButton } from "../../core";
/**
 * Represents a button field of a [[PDFForm]].
 *
 * [[PDFButton]] fields are interactive controls that users can click with their
 * mouse. This type of [[PDFField]] is not stateful. The purpose of a button
 * is to perform an action when the user clicks on it, such as opening a print
 * modal or resetting the form. Buttons are typically rectangular in shape and
 * have a text label describing the action that they perform when clicked.
 */
export default class PDFButton extends PDFField {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFForm.getButton]] method, which will create an
     * > instance of [[PDFButton]] for you.
     *
     * Create an instance of [[PDFButton]] from an existing acroPushButton and ref
     *
     * @param acroPushButton The underlying `PDFAcroPushButton` for this button.
     * @param ref The unique reference for this button.
     * @param doc The document to which this button will belong.
     */
    static of: (acroPushButton: PDFAcroPushButton, ref: PDFRef, doc: PDFDocument) => PDFButton;
    /** The low-level PDFAcroPushButton wrapped by this button. */
    readonly acroField: PDFAcroPushButton;
    private constructor();
    /**
     * Display an image inside the bounds of this button's widgets. For example:
     * ```js
     * const pngImage = await pdfDoc.embedPng(...)
     * const button = form.getButton('some.button.field')
     * button.setImage(pngImage, ImageAlignment.Center)
     * ```
     * This will update the appearances streams for each of this button's widgets.
     * @param image The image that should be displayed.
     * @param alignment The alignment of the image.
     */
    setImage(image: PDFImage, alignment?: ImageAlignment): void;
    /**
     * Set the font size for this field. Larger font sizes will result in larger
     * text being displayed when PDF readers render this button. Font sizes may
     * be integer or floating point numbers. Supplying a negative font size will
     * cause this method to throw an error.
     *
     * For example:
     * ```js
     * const button = form.getButton('some.button.field')
     * button.setFontSize(4)
     * button.setFontSize(15.7)
     * ```
     *
     * > This method depends upon the existence of a default appearance
     * > (`/DA`) string. If this field does not have a default appearance string,
     * > or that string does not contain a font size (via the `Tf` operator),
     * > then this method will throw an error.
     *
     * @param fontSize The font size to be used when rendering text in this field.
     */
    setFontSize(fontSize: number): void;
    /**
     * Show this button on the specified page with the given text. For example:
     * ```js
     * const ubuntuFont = await pdfDoc.embedFont(ubuntuFontBytes)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const button = form.createButton('some.button.field')
     *
     * button.addToPage('Do Stuff', page, {
     *   x: 50,
     *   y: 75,
     *   width: 200,
     *   height: 100,
     *   textColor: rgb(1, 0, 0),
     *   backgroundColor: rgb(0, 1, 0),
     *   borderColor: rgb(0, 0, 1),
     *   borderWidth: 2,
     *   rotate: degrees(90),
     *   font: ubuntuFont,
     * })
     * ```
     * This will create a new widget for this button field.
     * @param text The text to be displayed for this button widget.
     * @param page The page to which this button widget should be added.
     * @param options The options to be used when adding this button widget.
     */
    addToPage(text: string, page: PDFPage, options?: FieldAppearanceOptions): void;
    /**
     * Returns `true` if this button has been marked as dirty, or if any of this
     * button's widgets do not have an appearance stream. For example:
     * ```js
     * const button = form.getButton('some.button.field')
     * if (button.needsAppearancesUpdate()) console.log('Needs update')
     * ```
     * @returns Whether or not this button needs an appearance update.
     */
    needsAppearancesUpdate(): boolean;
    /**
     * Update the appearance streams for each of this button's widgets using
     * the default appearance provider for buttons. For example:
     * ```js
     * const helvetica = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const button = form.getButton('some.button.field')
     * button.defaultUpdateAppearances(helvetica)
     * ```
     * @param font The font to be used for creating the appearance streams.
     */
    defaultUpdateAppearances(font: PDFFont): void;
    /**
     * Update the appearance streams for each of this button's widgets using
     * the given appearance provider. If no `provider` is passed, the default
     * appearance provider for buttons will be used. For example:
     * ```js
     * const helvetica = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const button = form.getButton('some.button.field')
     * button.updateAppearances(helvetica, (field, widget, font) => {
     *   ...
     *   return {
     *     normal: drawButton(...),
     *     down: drawButton(...),
     *   }
     * })
     * ```
     * @param font The font to be used for creating the appearance streams.
     * @param provider Optionally, the appearance provider to be used for
     *                 generating the contents of the appearance streams.
     */
    updateAppearances(font: PDFFont, provider?: AppearanceProviderFor<PDFButton>): void;
    private updateWidgetAppearance;
}
//# sourceMappingURL=PDFButton.d.ts.map
```

### es/api/form/PDFCheckBox.d.ts

```ts
import PDFDocument from "../PDFDocument";
import PDFPage from "../PDFPage";
import { AppearanceProviderFor } from "./appearances";
import PDFField, { FieldAppearanceOptions } from "./PDFField";
import { PDFRef, PDFAcroCheckBox } from "../../core";
/**
 * Represents a check box field of a [[PDFForm]].
 *
 * [[PDFCheckBox]] fields are interactive boxes that users can click with their
 * mouse. This type of [[PDFField]] has two states: `on` and `off`. The purpose
 * of a check box is to enable users to select from one or more options, where
 * each option is represented by a single check box. Check boxes are typically
 * square in shape and display a check mark when they are in the `on` state.
 */
export default class PDFCheckBox extends PDFField {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFForm.getCheckBox]] method, which will create an
     * > instance of [[PDFCheckBox]] for you.
     *
     * Create an instance of [[PDFCheckBox]] from an existing acroCheckBox and ref
     *
     * @param acroCheckBox The underlying `PDFAcroCheckBox` for this check box.
     * @param ref The unique reference for this check box.
     * @param doc The document to which this check box will belong.
     */
    static of: (acroCheckBox: PDFAcroCheckBox, ref: PDFRef, doc: PDFDocument) => PDFCheckBox;
    /** The low-level PDFAcroCheckBox wrapped by this check box. */
    readonly acroField: PDFAcroCheckBox;
    private constructor();
    /**
     * Mark this check box. This operation is analogous to a human user clicking
     * a check box to fill it in a PDF reader. This method will update the
     * underlying state of the check box field to indicate it has been selected.
     * PDF libraries and readers will be able to extract this value from the
     * saved document and determine that it was selected.
     *
     * For example:
     * ```js
     * const checkBox = form.getCheckBox('some.checkBox.field')
     * checkBox.check()
     * ```
     *
     * This method will mark this check box as dirty, causing its appearance
     * streams to be updated when either [[PDFDocument.save]] or
     * [[PDFForm.updateFieldAppearances]] is called. The updated appearance
     * streams will display a check mark inside the widgets of this check box
     * field.
     */
    check(): void;
    /**
     * Clears this check box. This operation is analogous to a human user clicking
     * a check box to unmark it in a PDF reader. This method will update the
     * underlying state of the check box field to indicate it has been deselected.
     * PDF libraries and readers will be able to extract this value from the
     * saved document and determine that it was not selected.
     *
     * For example:
     * ```js
     * const checkBox = form.getCheckBox('some.checkBox.field')
     * checkBox.uncheck()
     * ```
     *
     * This method will mark this check box as dirty. See [[PDFCheckBox.check]]
     * for more details about what this means.
     */
    uncheck(): void;
    /**
     * Returns `true` if this check box is selected (either by a human user via
     * a PDF reader, or else programmatically via software). For example:
     * ```js
     * const checkBox = form.getCheckBox('some.checkBox.field')
     * if (checkBox.isChecked()) console.log('check box is selected')
     * ```
     * @returns Whether or not this check box is selected.
     */
    isChecked(): boolean;
    /**
     * Show this check box on the specified page. For example:
     * ```js
     * const helvetica = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const checkBox = form.createCheckBox('some.checkBox.field')
     *
     * checkBox.addToPage(page, {
     *   x: 50,
     *   y: 75,
     *   width: 25,
     *   height: 25,
     *   textColor: rgb(1, 0, 0),
     *   backgroundColor: rgb(0, 1, 0),
     *   borderColor: rgb(0, 0, 1),
     *   borderWidth: 2,
     *   rotate: degrees(90),
     * })
     * ```
     * This will create a new widget for this check box field.
     * @param page The page to which this check box widget should be added.
     * @param options The options to be used when adding this check box widget.
     */
    addToPage(page: PDFPage, options?: FieldAppearanceOptions): void;
    /**
     * Returns `true` if any of this check box's widgets do not have an
     * appearance stream for its current state. For example:
     * ```js
     * const checkBox = form.getCheckBox('some.checkBox.field')
     * if (checkBox.needsAppearancesUpdate()) console.log('Needs update')
     * ```
     * @returns Whether or not this check box needs an appearance update.
     */
    needsAppearancesUpdate(): boolean;
    /**
     * Update the appearance streams for each of this check box's widgets using
     * the default appearance provider for check boxes. For example:
     * ```js
     * const checkBox = form.getCheckBox('some.checkBox.field')
     * checkBox.defaultUpdateAppearances()
     * ```
     */
    defaultUpdateAppearances(): void;
    /**
     * Update the appearance streams for each of this check box's widgets using
     * the given appearance provider. If no `provider` is passed, the default
     * appearance provider for check boxs will be used. For example:
     * ```js
     * const checkBox = form.getCheckBox('some.checkBox.field')
     * checkBox.updateAppearances((field, widget) => {
     *   ...
     *   return {
     *     normal: { on: drawCheckBox(...), off: drawCheckBox(...) },
     *     down: { on: drawCheckBox(...), off: drawCheckBox(...) },
     *   }
     * })
     * ```
     * @param provider Optionally, the appearance provider to be used for
     *                 generating the contents of the appearance streams.
     */
    updateAppearances(provider?: AppearanceProviderFor<PDFCheckBox>): void;
    private updateWidgetAppearance;
}
//# sourceMappingURL=PDFCheckBox.d.ts.map
```

### es/api/form/PDFDropdown.d.ts

```ts
import PDFDocument from "../PDFDocument";
import PDFPage from "../PDFPage";
import PDFFont from "../PDFFont";
import PDFField, { FieldAppearanceOptions } from "./PDFField";
import { AppearanceProviderFor } from "./appearances";
import { PDFRef, PDFAcroComboBox } from "../../core";
/**
 * Represents a dropdown field of a [[PDFForm]].
 *
 * [[PDFDropdown]] fields are interactive text boxes that display a single
 * element (the currently selected value). The purpose of a dropdown is to
 * enable users to select a single option from a set of possible options. Users
 * can click on a dropdown to view the full list of options it provides.
 * Clicking on an option in the list will cause it to be selected and displayed
 * in the dropdown's text box. Some dropdowns allow users to enter text
 * directly into the box from their keyboard, rather than only being allowed to
 * choose an option from the list (see [[PDFDropdown.isEditable]]).
 */
export default class PDFDropdown extends PDFField {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFForm.getDropdown]] method, which will create an
     * > instance of [[PDFDropdown]] for you.
     *
     * Create an instance of [[PDFDropdown]] from an existing acroComboBox and ref
     *
     * @param acroComboBox The underlying `PDFAcroComboBox` for this dropdown.
     * @param ref The unique reference for this dropdown.
     * @param doc The document to which this dropdown will belong.
     */
    static of: (acroComboBox: PDFAcroComboBox, ref: PDFRef, doc: PDFDocument) => PDFDropdown;
    /** The low-level PDFAcroComboBox wrapped by this dropdown. */
    readonly acroField: PDFAcroComboBox;
    private constructor();
    /**
     * Get the list of available options for this dropdown. These options will be
     * displayed to users who click on this dropdown in a PDF reader.
     * For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * const options = dropdown.getOptions()
     * console.log('Dropdown options:', options)
     * ```
     * @returns The options for this dropdown.
     */
    getOptions(): string[];
    /**
     * Get the selected options for this dropdown. These are the values that were
     * selected by a human user via a PDF reader, or programatically via
     * software.
     * For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * const selections = dropdown.getSelected()
     * console.log('Dropdown selections:', selections)
     * ```
     * > **NOTE:** Note that PDF readers only display one selected option when
     * > rendering dropdowns. However, the PDF specification does allow for
     * > multiple values to be selected in a dropdown. As such, the `pdf-lib`
     * > API supports this. However, in most cases the array returned by this
     * > method will contain only a single element (or no elements).
     * @returns The selected options in this dropdown.
     */
    getSelected(): string[];
    /**
     * Set the list of options that are available for this dropdown. These are
     * the values that will be available for users to select when they view this
     * dropdown in a PDF reader. Note that preexisting options for this dropdown
     * will be removed. Only the values passed as `options` will be available to
     * select.
     * For example:
     * ```js
     * const dropdown = form.getDropdown('planets.dropdown')
     * dropdown.setOptions(['Earth', 'Mars', 'Pluto', 'Venus'])
     * ```
     * @param options The options that should be available in this dropdown.
     */
    setOptions(options: string[]): void;
    /**
     * Add to the list of options that are available for this dropdown. Users
     * will be able to select these values in a PDF reader. In addition to the
     * values passed as `options`, any preexisting options for this dropdown will
     * still be available for users to select.
     * For example:
     * ```js
     * const dropdown = form.getDropdown('rockets.dropdown')
     * dropdown.addOptions(['Saturn IV', 'Falcon Heavy'])
     * ```
     * @param options New options that should be available in this dropdown.
     */
    addOptions(options: string | string[]): void;
    /**
     * Select one or more values for this dropdown. This operation is analogous
     * to a human user opening the dropdown in a PDF reader and clicking on a
     * value to select it. This method will update the underlying state of the
     * dropdown to indicate which values have been selected. PDF libraries and
     * readers will be able to extract these values from the saved document and
     * determine which values were selected.
     *
     * For example:
     * ```js
     * const dropdown = form.getDropdown('best.superhero.dropdown')
     * dropdown.select('One Punch Man')
     * ```
     *
     * This method will mark this dropdown as dirty, causing its appearance
     * streams to be updated when either [[PDFDocument.save]] or
     * [[PDFForm.updateFieldAppearances]] is called. The updated streams will
     * display the selected option inside the widgets of this dropdown.
     *
     * **IMPORTANT:** The default font used to update appearance streams is
     * [[StandardFonts.Helvetica]]. Note that this is a WinAnsi font. This means
     * that encoding errors will be thrown if the selected option for this field
     * contains characters outside the WinAnsi character set (the latin alphabet).
     *
     * Embedding a custom font and passing it to
     * [[PDFForm.updateFieldAppearances]] or [[PDFDropdown.updateAppearances]]
     * allows you to generate appearance streams with characters outside the
     * latin alphabet (assuming the custom font supports them).
     *
     * Selecting an option that does not exist in this dropdown's option list
     * (see [[PDFDropdown.getOptions]]) will enable editing on this dropdown
     * (see [[PDFDropdown.enableEditing]]).
     *
     * > **NOTE:** PDF readers only display one selected option when rendering
     * > dropdowns. However, the PDF specification does allow for multiple values
     * > to be selected in a dropdown. As such, the `pdf-lib` API supports this.
     * > However, it is not recommended to select more than one value with this
     * > method, as only one will be visible. [[PDFOptionList]] fields are better
     * > suited for displaying multiple selected values.
     *
     * @param options The options to be selected.
     * @param merge Whether or not existing selections should be preserved.
     */
    select(options: string | string[], merge?: boolean): void;
    /**
     * Clear all selected values for this dropdown. This operation is equivalent
     * to selecting an empty list. This method will update the underlying state
     * of the dropdown to indicate that no values have been selected.
     * For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.clear()
     * ```
     * This method will mark this text field as dirty. See [[PDFDropdown.select]]
     * for more details about what this means.
     */
    clear(): void;
    /**
     * Set the font size for this field. Larger font sizes will result in larger
     * text being displayed when PDF readers render this dropdown. Font sizes may
     * be integer or floating point numbers. Supplying a negative font size will
     * cause this method to throw an error.
     *
     * For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.setFontSize(4)
     * dropdown.setFontSize(15.7)
     * ```
     *
     * > This method depends upon the existence of a default appearance
     * > (`/DA`) string. If this field does not have a default appearance string,
     * > or that string does not contain a font size (via the `Tf` operator),
     * > then this method will throw an error.
     *
     * @param fontSize The font size to be used when rendering text in this field.
     */
    setFontSize(fontSize: number): void;
    /**
     * Returns `true` if users are allowed to edit the selected value of this
     * dropdown directly and are not constrained by the list of available
     * options. See [[PDFDropdown.enableEditing]] and
     * [[PDFDropdown.disableEditing]]. For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * if (dropdown.isEditable()) console.log('Editing is enabled')
     * ```
     * @returns Whether or not this dropdown is editable.
     */
    isEditable(): boolean;
    /**
     * Allow users to edit the selected value of this dropdown in PDF readers
     * with their keyboard. This means that the selected value of this dropdown
     * will not be constrained by the list of available options. However, if this
     * dropdown has any available options, users will still be allowed to select
     * from that list.
     * For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.enableEditing()
     * ```
     */
    enableEditing(): void;
    /**
     * Do not allow users to edit the selected value of this dropdown in PDF
     * readers with their keyboard. This will constrain the selected value of
     * this dropdown to the list of available options. Users will only be able
     * to select an option from that list.
     * For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.disableEditing()
     * ```
     */
    disableEditing(): void;
    /**
     * Returns `true` if the option list of this dropdown is always displayed
     * in alphabetical order, irrespective of the order in which the options
     * were added to the dropdown. See [[PDFDropdown.enableSorting]] and
     * [[PDFDropdown.disableSorting]]. For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * if (dropdown.isSorted()) console.log('Sorting is enabled')
     * ```
     * @returns Whether or not this dropdown's options are sorted.
     */
    isSorted(): boolean;
    /**
     * Always display the option list of this dropdown in alphabetical order,
     * irrespective of the order in which the options were added to this dropdown.
     * For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.enableSorting()
     * ```
     */
    enableSorting(): void;
    /**
     * Do not always display the option list of this dropdown in alphabetical
     * order. Instead, display the options in whichever order they were added
     * to the list. For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.disableSorting()
     * ```
     */
    disableSorting(): void;
    /**
     * Returns `true` if multiple options can be selected from this dropdown's
     * option list. See [[PDFDropdown.enableMultiselect]] and
     * [[PDFDropdown.disableMultiselect]]. For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * if (dropdown.isMultiselect()) console.log('Multiselect is enabled')
     * ```
     * @returns Whether or not multiple options can be selected.
     */
    isMultiselect(): boolean;
    /**
     * Allow users to select more than one option from this dropdown's option
     * list. For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.enableMultiselect()
     * ```
     */
    enableMultiselect(): void;
    /**
     * Do not allow users to select more than one option from this dropdown's
     * option list. For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.disableMultiselect()
     * ```
     */
    disableMultiselect(): void;
    /**
     * Returns `true` if the selected option should be spell checked by PDF
     * readers. Spell checking will only be performed if this dropdown allows
     * editing (see [[PDFDropdown.isEditable]]). See
     * [[PDFDropdown.enableSpellChecking]] and
     * [[PDFDropdown.disableSpellChecking]]. For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * if (dropdown.isSpellChecked()) console.log('Spell checking is enabled')
     * ```
     * @returns Whether or not this dropdown can be spell checked.
     */
    isSpellChecked(): boolean;
    /**
     * Allow PDF readers to spell check the selected option of this dropdown.
     * For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.enableSpellChecking()
     * ```
     */
    enableSpellChecking(): void;
    /**
     * Do not allow PDF readers to spell check the selected option of this
     * dropdown. For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.disableSpellChecking()
     * ```
     */
    disableSpellChecking(): void;
    /**
     * Returns `true` if the option selected by a user is stored, or "committed",
     * when the user clicks the option. The alternative is that the user's
     * selection is stored when the user leaves this dropdown field (by clicking
     * outside of it - on another field, for example). See
     * [[PDFDropdown.enableSelectOnClick]] and
     * [[PDFDropdown.disableSelectOnClick]]. For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * if (dropdown.isSelectOnClick()) console.log('Select on click is enabled')
     * ```
     * @returns Whether or not options are selected immediately after they are
     *          clicked.
     */
    isSelectOnClick(): boolean;
    /**
     * Store the option selected by a user immediately after the user clicks the
     * option. Do not wait for the user to leave this dropdown field (by clicking
     * outside of it - on another field, for example). For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.enableSelectOnClick()
     * ```
     */
    enableSelectOnClick(): void;
    /**
     * Wait to store the option selected by a user until they leave this dropdown
     * field (by clicking outside of it - on another field, for example).
     * For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.disableSelectOnClick()
     * ```
     */
    disableSelectOnClick(): void;
    /**
     * Show this dropdown on the specified page. For example:
     * ```js
     * const ubuntuFont = await pdfDoc.embedFont(ubuntuFontBytes)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const dropdown = form.createDropdown('best.gundam')
     * dropdown.setOptions(['Exia', 'Dynames'])
     * dropdown.select('Exia')
     *
     * dropdown.addToPage(page, {
     *   x: 50,
     *   y: 75,
     *   width: 200,
     *   height: 100,
     *   textColor: rgb(1, 0, 0),
     *   backgroundColor: rgb(0, 1, 0),
     *   borderColor: rgb(0, 0, 1),
     *   borderWidth: 2,
     *   rotate: degrees(90),
     *   font: ubuntuFont,
     * })
     * ```
     * This will create a new widget for this dropdown field.
     * @param page The page to which this dropdown widget should be added.
     * @param options The options to be used when adding this dropdown widget.
     */
    addToPage(page: PDFPage, options?: FieldAppearanceOptions): void;
    /**
     * Returns `true` if this dropdown has been marked as dirty, or if any of
     * this dropdown's widgets do not have an appearance stream. For example:
     * ```js
     * const dropdown = form.getDropdown('some.dropdown.field')
     * if (dropdown.needsAppearancesUpdate()) console.log('Needs update')
     * ```
     * @returns Whether or not this dropdown needs an appearance update.
     */
    needsAppearancesUpdate(): boolean;
    /**
     * Update the appearance streams for each of this dropdown's widgets using
     * the default appearance provider for dropdowns. For example:
     * ```js
     * const helvetica = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.defaultUpdateAppearances(helvetica)
     * ```
     * @param font The font to be used for creating the appearance streams.
     */
    defaultUpdateAppearances(font: PDFFont): void;
    /**
     * Update the appearance streams for each of this dropdown's widgets using
     * the given appearance provider. If no `provider` is passed, the default
     * appearance provider for dropdowns will be used. For example:
     * ```js
     * const helvetica = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const dropdown = form.getDropdown('some.dropdown.field')
     * dropdown.updateAppearances(helvetica, (field, widget, font) => {
     *   ...
     *   return drawTextField(...)
     * })
     * ```
     * @param font The font to be used for creating the appearance streams.
     * @param provider Optionally, the appearance provider to be used for
     *                 generating the contents of the appearance streams.
     */
    updateAppearances(font: PDFFont, provider?: AppearanceProviderFor<PDFDropdown>): void;
    private updateWidgetAppearance;
}
//# sourceMappingURL=PDFDropdown.d.ts.map
```

### es/api/form/PDFField.d.ts

```ts
import PDFDocument from "../PDFDocument";
import PDFFont from "../PDFFont";
import { AppearanceMapping } from "./appearances";
import { Color } from "../colors";
import { Rotation } from "../rotations";
import { PDFRef, PDFWidgetAnnotation, PDFOperator, PDFName, PDFDict, PDFAcroTerminal } from "../../core";
import { ImageAlignment } from '../image';
import PDFImage from '../PDFImage';
export interface FieldAppearanceOptions {
    x?: number;
    y?: number;
    width?: number;
    height?: number;
    textColor?: Color;
    backgroundColor?: Color;
    borderColor?: Color;
    borderWidth?: number;
    rotate?: Rotation;
    font?: PDFFont;
    hidden?: boolean;
}
export declare const assertFieldAppearanceOptions: (options?: FieldAppearanceOptions | undefined) => void;
/**
 * Represents a field of a [[PDFForm]].
 *
 * This class is effectively abstract. All fields in a [[PDFForm]] will
 * actually be an instance of a subclass of this class.
 *
 * Note that each field in a PDF is represented by a single field object.
 * However, a given field object may be rendered at multiple locations within
 * the document (across one or more pages). The rendering of a field is
 * controlled by its widgets. Each widget causes its field to be displayed at a
 * particular location in the document.
 *
 * Most of the time each field in a PDF has only a single widget, and thus is
 * only rendered once. However, if a field is rendered multiple times, it will
 * have multiple widgets - one for each location it is rendered.
 *
 * This abstraction of field objects and widgets is defined in the PDF
 * specification and dictates how PDF files store fields and where they are
 * to be rendered.
 */
export default class PDFField {
    /** The low-level PDFAcroTerminal wrapped by this field. */
    readonly acroField: PDFAcroTerminal;
    /** The unique reference assigned to this field within the document. */
    readonly ref: PDFRef;
    /** The document to which this field belongs. */
    readonly doc: PDFDocument;
    protected constructor(acroField: PDFAcroTerminal, ref: PDFRef, doc: PDFDocument);
    /**
     * Get the fully qualified name of this field. For example:
     * ```js
     * const fields = form.getFields()
     * fields.forEach(field => {
     *   const name = field.getName()
     *   console.log('Field name:', name)
     * })
     * ```
     * Note that PDF fields are structured as a tree. Each field is the
     * descendent of a series of ancestor nodes all the way up to the form node,
     * which is always the root of the tree. Each node in the tree (except for
     * the form node) has a partial name. Partial names can be composed of any
     * unicode characters except a period (`.`). The fully qualified name of a
     * field is composed of the partial names of all its ancestors joined
     * with periods. This means that splitting the fully qualified name on
     * periods and taking the last element of the resulting array will give you
     * the partial name of a specific field.
     * @returns The fully qualified name of this field.
     */
    getName(): string;
    /**
     * Returns `true` if this field is read only. This means that PDF readers
     * will not allow users to interact with the field or change its value. See
     * [[PDFField.enableReadOnly]] and [[PDFField.disableReadOnly]].
     * For example:
     * ```js
     * const field = form.getField('some.field')
     * if (field.isReadOnly()) console.log('Read only is enabled')
     * ```
     * @returns Whether or not this is a read only field.
     */
    isReadOnly(): boolean;
    /**
     * Prevent PDF readers from allowing users to interact with this field or
     * change its value. The field will not respond to mouse or keyboard input.
     * For example:
     * ```js
     * const field = form.getField('some.field')
     * field.enableReadOnly()
     * ```
     * Useful for fields whose values are computed, imported from a database, or
     * prefilled by software before being displayed to the user.
     */
    enableReadOnly(): void;
    /**
     * Allow users to interact with this field and change its value in PDF
     * readers via mouse and keyboard input. For example:
     * ```js
     * const field = form.getField('some.field')
     * field.disableReadOnly()
     * ```
     */
    disableReadOnly(): void;
    /**
     * Returns `true` if this field must have a value when the form is submitted.
     * See [[PDFField.enableRequired]] and [[PDFField.disableRequired]].
     * For example:
     * ```js
     * const field = form.getField('some.field')
     * if (field.isRequired()) console.log('Field is required')
     * ```
     * @returns Whether or not this field is required.
     */
    isRequired(): boolean;
    /**
     * Require this field to have a value when the form is submitted.
     * For example:
     * ```js
     * const field = form.getField('some.field')
     * field.enableRequired()
     * ```
     */
    enableRequired(): void;
    /**
     * Do not require this field to have a value when the form is submitted.
     * For example:
     * ```js
     * const field = form.getField('some.field')
     * field.disableRequired()
     * ```
     */
    disableRequired(): void;
    /**
     * Returns `true` if this field's value should be exported when the form is
     * submitted. See [[PDFField.enableExporting]] and
     * [[PDFField.disableExporting]].
     * For example:
     * ```js
     * const field = form.getField('some.field')
     * if (field.isExported()) console.log('Exporting is enabled')
     * ```
     * @returns Whether or not this field's value should be exported.
     */
    isExported(): boolean;
    /**
     * Indicate that this field's value should be exported when the form is
     * submitted in a PDF reader. For example:
     * ```js
     * const field = form.getField('some.field')
     * field.enableExporting()
     * ```
     */
    enableExporting(): void;
    /**
     * Indicate that this field's value should **not** be exported when the form
     * is submitted in a PDF reader. For example:
     * ```js
     * const field = form.getField('some.field')
     * field.disableExporting()
     * ```
     */
    disableExporting(): void;
    /** @ignore */
    needsAppearancesUpdate(): boolean;
    /** @ignore */
    defaultUpdateAppearances(_font: PDFFont): void;
    protected markAsDirty(): void;
    protected markAsClean(): void;
    protected isDirty(): boolean;
    protected createWidget(options: {
        x: number;
        y: number;
        width: number;
        height: number;
        textColor?: Color;
        backgroundColor?: Color;
        borderColor?: Color;
        borderWidth: number;
        rotate: Rotation;
        caption?: string;
        hidden?: boolean;
        page?: PDFRef;
    }): PDFWidgetAnnotation;
    protected updateWidgetAppearanceWithFont(widget: PDFWidgetAnnotation, font: PDFFont, { normal, rollover, down }: AppearanceMapping<PDFOperator[]>): void;
    protected updateOnOffWidgetAppearance(widget: PDFWidgetAnnotation, onValue: PDFName, { normal, rollover, down, }: AppearanceMapping<{
        on: PDFOperator[];
        off: PDFOperator[];
    }>): void;
    protected updateWidgetAppearances(widget: PDFWidgetAnnotation, { normal, rollover, down }: AppearanceMapping<PDFRef | PDFDict>): void;
    private createAppearanceStream;
    /**
     * Create a FormXObject of the supplied image and add it to context.
     * The FormXObject size is calculated based on the widget (including
     * the alignment).
     * @param widget The widget that should display the image.
     * @param alignment The alignment of the image.
     * @param image The image that should be displayed.
     * @returns The ref for the FormXObject that was added to the context.
     */
    protected createImageAppearanceStream(widget: PDFWidgetAnnotation, image: PDFImage, alignment: ImageAlignment): PDFRef;
    private createAppearanceDict;
}
//# sourceMappingURL=PDFField.d.ts.map
```

### es/api/form/PDFForm.d.ts

```ts
import PDFDocument from "../PDFDocument";
import PDFField from "./PDFField";
import PDFButton from "./PDFButton";
import PDFCheckBox from "./PDFCheckBox";
import PDFDropdown from "./PDFDropdown";
import PDFOptionList from "./PDFOptionList";
import PDFRadioGroup from "./PDFRadioGroup";
import PDFSignature from "./PDFSignature";
import PDFTextField from "./PDFTextField";
import PDFFont from "../PDFFont";
import { PDFAcroForm, PDFRef } from "../../core";
export interface FlattenOptions {
    updateFieldAppearances: boolean;
}
/**
 * Represents the interactive form of a [[PDFDocument]].
 *
 * Interactive forms (sometimes called _AcroForms_) are collections of fields
 * designed to gather information from a user. A PDF document may contains any
 * number of fields that appear on various pages, all of which make up a single,
 * global interactive form spanning the entire document. This means that
 * instances of [[PDFDocument]] shall contain at most one [[PDFForm]].
 *
 * The fields of an interactive form are represented by [[PDFField]] instances.
 */
export default class PDFForm {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFDocument.getForm]] method, which will create an
     * > instance of [[PDFForm]] for you.
     *
     * Create an instance of [[PDFForm]] from an existing acroForm and embedder
     *
     * @param acroForm The underlying `PDFAcroForm` for this form.
     * @param doc The document to which the form will belong.
     */
    static of: (acroForm: PDFAcroForm, doc: PDFDocument) => PDFForm;
    /** The low-level PDFAcroForm wrapped by this form. */
    readonly acroForm: PDFAcroForm;
    /** The document to which this form belongs. */
    readonly doc: PDFDocument;
    private readonly dirtyFields;
    private readonly defaultFontCache;
    private constructor();
    /**
     * Returns `true` if this [[PDFForm]] has XFA data. Most PDFs with form
     * fields do not use XFA as it is not widely supported by PDF readers.
     *
     * > `pdf-lib` does not support creation, modification, or reading of XFA
     * > fields.
     *
     * For example:
     * ```js
     * const form = pdfDoc.getForm()
     * if (form.hasXFA()) console.log('PDF has XFA data')
     * ```
     * @returns Whether or not this form has XFA data.
     */
    hasXFA(): boolean;
    /**
     * Disconnect the XFA data from this [[PDFForm]] (if any exists). This will
     * force readers to fallback to standard fields if the [[PDFDocument]]
     * contains any. For example:
     *
     * For example:
     * ```js
     * const form = pdfDoc.getForm()
     * form.deleteXFA()
     * ```
     */
    deleteXFA(): void;
    /**
     * Get all fields contained in this [[PDFForm]]. For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const fields = form.getFields()
     * fields.forEach(field => {
     *   const type = field.constructor.name
     *   const name = field.getName()
     *   console.log(`${type}: ${name}`)
     * })
     * ```
     * @returns An array of all fields in this form.
     */
    getFields(): PDFField[];
    /**
     * Get the field in this [[PDFForm]] with the given name. For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const field = form.getFieldMaybe('Page1.Foo.Bar[0]')
     * if (field) console.log('Field exists!')
     * ```
     * @param name A fully qualified field name.
     * @returns The field with the specified name, if one exists.
     */
    getFieldMaybe(name: string): PDFField | undefined;
    /**
     * Get the field in this [[PDFForm]] with the given name. For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const field = form.getField('Page1.Foo.Bar[0]')
     * ```
     * If no field exists with the provided name, an error will be thrown.
     * @param name A fully qualified field name.
     * @returns The field with the specified name.
     */
    getField(name: string): PDFField;
    /**
     * Get the button field in this [[PDFForm]] with the given name. For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const button = form.getButton('Page1.Foo.Button[0]')
     * ```
     * An error will be thrown if no field exists with the provided name, or if
     * the field exists but is not a button.
     * @param name A fully qualified button name.
     * @returns The button with the specified name.
     */
    getButton(name: string): PDFButton;
    /**
     * Get the check box field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const checkBox = form.getCheckBox('Page1.Foo.CheckBox[0]')
     * checkBox.check()
     * ```
     * An error will be thrown if no field exists with the provided name, or if
     * the field exists but is not a check box.
     * @param name A fully qualified check box name.
     * @returns The check box with the specified name.
     */
    getCheckBox(name: string): PDFCheckBox;
    /**
     * Get the dropdown field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const dropdown = form.getDropdown('Page1.Foo.Dropdown[0]')
     * const options = dropdown.getOptions()
     * dropdown.select(options[0])
     * ```
     * An error will be thrown if no field exists with the provided name, or if
     * the field exists but is not a dropdown.
     * @param name A fully qualified dropdown name.
     * @returns The dropdown with the specified name.
     */
    getDropdown(name: string): PDFDropdown;
    /**
     * Get the option list field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const optionList = form.getOptionList('Page1.Foo.OptionList[0]')
     * const options = optionList.getOptions()
     * optionList.select(options[0])
     * ```
     * An error will be thrown if no field exists with the provided name, or if
     * the field exists but is not an option list.
     * @param name A fully qualified option list name.
     * @returns The option list with the specified name.
     */
    getOptionList(name: string): PDFOptionList;
    /**
     * Get the radio group field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const radioGroup = form.getRadioGroup('Page1.Foo.RadioGroup[0]')
     * const options = radioGroup.getOptions()
     * radioGroup.select(options[0])
     * ```
     * An error will be thrown if no field exists with the provided name, or if
     * the field exists but is not a radio group.
     * @param name A fully qualified radio group name.
     * @returns The radio group with the specified name.
     */
    getRadioGroup(name: string): PDFRadioGroup;
    /**
     * Get the signature field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const signature = form.getSignature('Page1.Foo.Signature[0]')
     * ```
     * An error will be thrown if no field exists with the provided name, or if
     * the field exists but is not a signature.
     * @param name A fully qualified signature name.
     * @returns The signature with the specified name.
     */
    getSignature(name: string): PDFSignature;
    /**
     * Get the text field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const form = pdfDoc.getForm()
     * const textField = form.getTextField('Page1.Foo.TextField[0]')
     * textField.setText('Are you designed to act or to be acted upon?')
     * ```
     * An error will be thrown if no field exists with the provided name, or if
     * the field exists but is not a text field.
     * @param name A fully qualified text field name.
     * @returns The text field with the specified name.
     */
    getTextField(name: string): PDFTextField;
    /**
     * Create a new button field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const font = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const button = form.createButton('cool.new.button')
     *
     * button.addToPage('Do Stuff', font, page)
     * ```
     * An error will be thrown if a field already exists with the provided name.
     * @param name The fully qualified name for the new button.
     * @returns The new button field.
     */
    createButton(name: string): PDFButton;
    /**
     * Create a new check box field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const font = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const checkBox = form.createCheckBox('cool.new.checkBox')
     *
     * checkBox.addToPage(page)
     * ```
     * An error will be thrown if a field already exists with the provided name.
     * @param name The fully qualified name for the new check box.
     * @returns The new check box field.
     */
    createCheckBox(name: string): PDFCheckBox;
    /**
     * Create a new dropdown field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const font = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const dropdown = form.createDropdown('cool.new.dropdown')
     *
     * dropdown.addToPage(font, page)
     * ```
     * An error will be thrown if a field already exists with the provided name.
     * @param name The fully qualified name for the new dropdown.
     * @returns The new dropdown field.
     */
    createDropdown(name: string): PDFDropdown;
    /**
     * Create a new option list field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const font = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const optionList = form.createOptionList('cool.new.optionList')
     *
     * optionList.addToPage(font, page)
     * ```
     * An error will be thrown if a field already exists with the provided name.
     * @param name The fully qualified name for the new option list.
     * @returns The new option list field.
     */
    createOptionList(name: string): PDFOptionList;
    /**
     * Create a new radio group field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const font = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const radioGroup = form.createRadioGroup('cool.new.radioGroup')
     *
     * radioGroup.addOptionToPage('is-dog', page, { y: 0 })
     * radioGroup.addOptionToPage('is-cat', page, { y: 75 })
     * ```
     * An error will be thrown if a field already exists with the provided name.
     * @param name The fully qualified name for the new radio group.
     * @returns The new radio group field.
     */
    createRadioGroup(name: string): PDFRadioGroup;
    /**
     * Create a new text field in this [[PDFForm]] with the given name.
     * For example:
     * ```js
     * const font = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const textField = form.createTextField('cool.new.textField')
     *
     * textField.addToPage(font, page)
     * ```
     * An error will be thrown if a field already exists with the provided name.
     * @param name The fully qualified name for the new radio group.
     * @returns The new radio group field.
     */
    createTextField(name: string): PDFTextField;
    /**
     * Flatten all fields in this [[PDFForm]].
     *
     * Flattening a form field will take the current appearance for each of that
     * field's widgets and make them part of their page's content stream. All form
     * fields and annotations associated are then removed. Note that once a form
     * has been flattened its fields can no longer be accessed or edited.
     *
     * This operation is often used after filling form fields to ensure a
     * consistent appearance across different PDF readers and/or printers.
     * Another common use case is to copy a template document with form fields
     * into another document. In this scenario you would load the template
     * document, fill its fields, flatten it, and then copy its pages into the
     * recipient document - the filled fields will be copied over.
     *
     * For example:
     * ```js
     * const form = pdfDoc.getForm();
     * form.flatten();
     * ```
     */
    flatten(options?: FlattenOptions): void;
    /**
     * Remove a field from this [[PDFForm]].
     *
     * For example:
     * ```js
     * const form = pdfDoc.getForm();
     * const ageField = form.getFields().find(x => x.getName() === 'Age');
     * form.removeField(ageField);
     * ```
     */
    removeField(field: PDFField): void;
    /**
     * Update the appearance streams for all widgets of all fields in this
     * [[PDFForm]]. Appearance streams will only be created for a widget if it
     * does not have any existing appearance streams, or the field's value has
     * changed (e.g. by calling [[PDFTextField.setText]] or
     * [[PDFDropdown.select]]).
     *
     * For example:
     * ```js
     * const courier = await pdfDoc.embedFont(StandardFonts.Courier)
     * const form = pdfDoc.getForm()
     * form.updateFieldAppearances(courier)
     * ```
     *
     * **IMPORTANT:** The default value for the `font` parameter is
     * [[StandardFonts.Helvetica]]. Note that this is a WinAnsi font. This means
     * that encoding errors will be thrown if any fields contain text with
     * characters outside the WinAnsi character set (the latin alphabet).
     *
     * Embedding a custom font and passing that as the `font`
     * parameter allows you to generate appearance streams with non WinAnsi
     * characters (assuming your custom font supports them).
     *
     * > **NOTE:** The [[PDFDocument.save]] method will call this method to
     * > update appearances automatically if a form was accessed via the
     * > [[PDFDocument.getForm]] method prior to saving.
     *
     * @param font Optionally, the font to use when creating new appearances.
     */
    updateFieldAppearances(font?: PDFFont): void;
    /**
     * Mark a field as dirty. This will cause its appearance streams to be
     * updated by [[PDFForm.updateFieldAppearances]].
     * ```js
     * const form = pdfDoc.getForm()
     * const field = form.getField('foo.bar')
     * form.markFieldAsDirty(field.ref)
     * ```
     * @param fieldRef The reference to the field that should be marked.
     */
    markFieldAsDirty(fieldRef: PDFRef): void;
    /**
     * Mark a field as dirty. This will cause its appearance streams to not be
     * updated by [[PDFForm.updateFieldAppearances]].
     * ```js
     * const form = pdfDoc.getForm()
     * const field = form.getField('foo.bar')
     * form.markFieldAsClean(field.ref)
     * ```
     * @param fieldRef The reference to the field that should be marked.
     */
    markFieldAsClean(fieldRef: PDFRef): void;
    /**
     * Returns `true` is the specified field has been marked as dirty.
     * ```js
     * const form = pdfDoc.getForm()
     * const field = form.getField('foo.bar')
     * if (form.fieldIsDirty(field.ref)) console.log('Field is dirty')
     * ```
     * @param fieldRef The reference to the field that should be checked.
     * @returns Whether or not the specified field is dirty.
     */
    fieldIsDirty(fieldRef: PDFRef): boolean;
    getDefaultFont(): PDFFont;
    private findWidgetPage;
    private findWidgetAppearanceRef;
    private findOrCreateNonTerminals;
    private findNonTerminal;
    private embedDefaultFont;
}
//# sourceMappingURL=PDFForm.d.ts.map
```

### es/api/form/PDFOptionList.d.ts

```ts
import PDFDocument from "../PDFDocument";
import PDFPage from "../PDFPage";
import PDFFont from "../PDFFont";
import PDFField, { FieldAppearanceOptions } from "./PDFField";
import { AppearanceProviderFor } from "./appearances";
import { PDFRef, PDFAcroListBox } from "../../core";
/**
 * Represents an option list field of a [[PDFForm]].
 *
 * [[PDFOptionList]] fields are interactive lists of options. The purpose of an
 * option list is to enable users to select one or more options from a set of
 * possible options. Users are able to see the full set of options without
 * first having to click on the field (though scrolling may be necessary).
 * Clicking an option in the list will cause it to be selected and displayed
 * with a highlighted background. Some option lists allow users to select
 * more than one option (see [[PDFOptionList.isMultiselect]]).
 */
export default class PDFOptionList extends PDFField {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFForm.getOptionList]] method, which will create
     * > an instance of [[PDFOptionList]] for you.
     *
     * Create an instance of [[PDFOptionList]] from an existing acroListBox and
     * ref
     *
     * @param acroComboBox The underlying `PDFAcroListBox` for this option list.
     * @param ref The unique reference for this option list.
     * @param doc The document to which this option list will belong.
     */
    static of: (acroListBox: PDFAcroListBox, ref: PDFRef, doc: PDFDocument) => PDFOptionList;
    /** The low-level PDFAcroListBox wrapped by this option list. */
    readonly acroField: PDFAcroListBox;
    private constructor();
    /**
     * Get the list of available options for this option list. These options will
     * be displayed to users who view this option list in a PDF reader.
     * For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * const options = optionList.getOptions()
     * console.log('Option List options:', options)
     * ```
     * @returns The options for this option list.
     */
    getOptions(): string[];
    /**
     * Get the selected options for this option list. These are the values that
     * were selected by a human user via a PDF reader, or programatically via
     * software.
     * For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * const selections = optionList.getSelected()
     * console.log('Option List selections:', selections)
     * ```
     * @returns The selected options for this option list.
     */
    getSelected(): string[];
    /**
     * Set the list of options that are available for this option list. These are
     * the values that will be available for users to select when they view this
     * option list in a PDF reader. Note that preexisting options for this
     * option list will be removed. Only the values passed as `options` will be
     * available to select.
     *
     * For example:
     * ```js
     * const optionList = form.getOptionList('planets.optionList')
     * optionList.setOptions(['Earth', 'Mars', 'Pluto', 'Venus'])
     * ```
     *
     * This method will mark this option list as dirty, causing its appearance
     * streams to be updated when either [[PDFDocument.save]] or
     * [[PDFForm.updateFieldAppearances]] is called. The updated streams will
     * display the options this field contains inside the widgets of this text
     * field (with selected options highlighted).
     *
     * **IMPORTANT:** The default font used to update appearance streams is
     * [[StandardFonts.Helvetica]]. Note that this is a WinAnsi font. This means
     * that encoding errors will be thrown if this field contains any options
     * with characters outside the WinAnsi character set (the latin alphabet).
     *
     * Embedding a custom font and passing it to
     * [[PDFForm.updateFieldAppearances]] or [[PDFOptionList.updateAppearances]]
     * allows you to generate appearance streams with characters outside the
     * latin alphabet (assuming the custom font supports them).
     *
     * @param options The options that should be available in this option list.
     */
    setOptions(options: string[]): void;
    /**
     * Add to the list of options that are available for this option list. Users
     * will be able to select these values in a PDF reader. In addition to the
     * values passed as `options`, any preexisting options for this option list
     * will still be available for users to select.
     * For example:
     * ```js
     * const optionList = form.getOptionList('rockets.optionList')
     * optionList.addOptions(['Saturn IV', 'Falcon Heavy'])
     * ```
     * This method will mark this option list as dirty. See
     * [[PDFOptionList.setOptions]] for more details about what this means.
     * @param options New options that should be available in this option list.
     */
    addOptions(options: string | string[]): void;
    /**
     * Select one or more values for this option list. This operation is analogous
     * to a human user opening the option list in a PDF reader and clicking on one
     * or more values to select them. This method will update the underlying state
     * of the option list to indicate which values have been selected. PDF
     * libraries and readers will be able to extract these values from the saved
     * document and determine which values were selected.
     * For example:
     * ```js
     * const optionList = form.getOptionList('best.superheroes.optionList')
     * optionList.select(['One Punch Man', 'Iron Man'])
     * ```
     * This method will mark this option list as dirty. See
     * [[PDFOptionList.setOptions]] for more details about what this means.
     * @param options The options to be selected.
     * @param merge Whether or not existing selections should be preserved.
     */
    select(options: string | string[], merge?: boolean): void;
    /**
     * Clear all selected values for this option list. This operation is
     * equivalent to selecting an empty list. This method will update the
     * underlying state of the option list to indicate that no values have been
     * selected.
     * For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.clear()
     * ```
     * This method will mark this option list as dirty. See
     * [[PDFOptionList.setOptions]] for more details about what this means.
     */
    clear(): void;
    /**
     * Set the font size for the text in this field. There needs to be a
     * default appearance string (DA) set with a font value specified
     * for this to work. For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.setFontSize(4);
     * ```
     * @param fontSize The font size to set the font to.
     */
    /**
     * Set the font size for this field. Larger font sizes will result in larger
     * text being displayed when PDF readers render this option list. Font sizes
     * may be integer or floating point numbers. Supplying a negative font size
     * will cause this method to throw an error.
     *
     * For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.setFontSize(4)
     * optionList.setFontSize(15.7)
     * ```
     *
     * > This method depends upon the existence of a default appearance
     * > (`/DA`) string. If this field does not have a default appearance string,
     * > or that string does not contain a font size (via the `Tf` operator),
     * > then this method will throw an error.
     *
     * @param fontSize The font size to be used when rendering text in this field.
     */
    setFontSize(fontSize: number): void;
    /**
     * Returns `true` if the options of this option list are always displayed
     * in alphabetical order, irrespective of the order in which the options
     * were added to the option list. See [[PDFOptionList.enableSorting]] and
     * [[PDFOptionList.disableSorting]]. For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * if (optionList.isSorted()) console.log('Sorting is enabled')
     * ```
     * @returns Whether or not this option list is sorted.
     */
    isSorted(): boolean;
    /**
     * Always display the options of this option list in alphabetical order,
     * irrespective of the order in which the options were added to this option
     * list.
     * For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.enableSorting()
     * ```
     */
    enableSorting(): void;
    /**
     * Do not always display the options of this option list in alphabetical
     * order. Instead, display the options in whichever order they were added
     * to this option list. For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.disableSorting()
     * ```
     */
    disableSorting(): void;
    /**
     * Returns `true` if multiple options can be selected from this option list.
     * See [[PDFOptionList.enableMultiselect]] and
     * [[PDFOptionList.disableMultiselect]]. For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * if (optionList.isMultiselect()) console.log('Multiselect is enabled')
     * ```
     * @returns Whether or not multiple options can be selected.
     */
    isMultiselect(): boolean;
    /**
     * Allow users to select more than one option from this option list.
     * For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.enableMultiselect()
     * ```
     */
    enableMultiselect(): void;
    /**
     * Do not allow users to select more than one option from this option list.
     * For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.disableMultiselect()
     * ```
     */
    disableMultiselect(): void;
    /**
     * Returns `true` if the option selected by a user is stored, or "committed",
     * when the user clicks the option. The alternative is that the user's
     * selection is stored when the user leaves this option list field (by
     * clicking outside of it - on another field, for example). See
     * [[PDFOptionList.enableSelectOnClick]] and
     * [[PDFOptionList.disableSelectOnClick]]. For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * if (optionList.isSelectOnClick()) console.log('Select on click is enabled')
     * ```
     * @returns Whether or not options are selected immediately after they are
     *          clicked.
     */
    isSelectOnClick(): boolean;
    /**
     * Store the option selected by a user immediately after the user clicks the
     * option. Do not wait for the user to leave this option list field (by
     * clicking outside of it - on another field, for example). For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.enableSelectOnClick()
     * ```
     */
    enableSelectOnClick(): void;
    /**
     * Wait to store the option selected by a user until they leave this option
     * list field (by clicking outside of it - on another field, for example).
     * For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.disableSelectOnClick()
     * ```
     */
    disableSelectOnClick(): void;
    /**
     * Show this option list on the specified page. For example:
     * ```js
     * const ubuntuFont = await pdfDoc.embedFont(ubuntuFontBytes)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const optionList = form.createOptionList('best.gundams')
     * optionList.setOptions(['Exia', 'Dynames', 'Kyrios', 'Virtue'])
     * optionList.select(['Exia', 'Virtue'])
     *
     * optionList.addToPage(page, {
     *   x: 50,
     *   y: 75,
     *   width: 200,
     *   height: 100,
     *   textColor: rgb(1, 0, 0),
     *   backgroundColor: rgb(0, 1, 0),
     *   borderColor: rgb(0, 0, 1),
     *   borderWidth: 2,
     *   rotate: degrees(90),
     *   font: ubuntuFont,
     * })
     * ```
     * This will create a new widget for this option list field.
     * @param page The page to which this option list widget should be added.
     * @param options The options to be used when adding this option list widget.
     */
    addToPage(page: PDFPage, options?: FieldAppearanceOptions): void;
    /**
     * Returns `true` if this option list has been marked as dirty, or if any of
     * this option list's widgets do not have an appearance stream. For example:
     * ```js
     * const optionList = form.getOptionList('some.optionList.field')
     * if (optionList.needsAppearancesUpdate()) console.log('Needs update')
     * ```
     * @returns Whether or not this option list needs an appearance update.
     */
    needsAppearancesUpdate(): boolean;
    /**
     * Update the appearance streams for each of this option list's widgets using
     * the default appearance provider for option lists. For example:
     * ```js
     * const helvetica = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.defaultUpdateAppearances(helvetica)
     * ```
     * @param font The font to be used for creating the appearance streams.
     */
    defaultUpdateAppearances(font: PDFFont): void;
    /**
     * Update the appearance streams for each of this option list's widgets using
     * the given appearance provider. If no `provider` is passed, the default
     * appearance provider for option lists will be used. For example:
     * ```js
     * const helvetica = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const optionList = form.getOptionList('some.optionList.field')
     * optionList.updateAppearances(helvetica, (field, widget, font) => {
     *   ...
     *   return drawOptionList(...)
     * })
     * ```
     * @param font The font to be used for creating the appearance streams.
     * @param provider Optionally, the appearance provider to be used for
     *                 generating the contents of the appearance streams.
     */
    updateAppearances(font: PDFFont, provider?: AppearanceProviderFor<PDFOptionList>): void;
    private updateWidgetAppearance;
}
//# sourceMappingURL=PDFOptionList.d.ts.map
```

### es/api/form/PDFRadioGroup.d.ts

```ts
import PDFDocument from "../PDFDocument";
import PDFPage from "../PDFPage";
import PDFField, { FieldAppearanceOptions } from "./PDFField";
import { AppearanceProviderFor } from "./appearances";
import { PDFRef, PDFAcroRadioButton } from "../../core";
/**
 * Represents a radio group field of a [[PDFForm]].
 *
 * [[PDFRadioGroup]] fields are collections of radio buttons. The purpose of a
 * radio group is to enable users to select one option from a set of mutually
 * exclusive choices. Each choice in a radio group is represented by a radio
 * button. Radio buttons each have two states: `on` and `off`. At most one
 * radio button in a group may be in the `on` state at any time. Users can
 * click on a radio button to select it (and thereby automatically deselect any
 * other radio button that might have already been selected). Some radio
 * groups allow users to toggle a selected radio button `off` by clicking on
 * it (see [[PDFRadioGroup.isOffToggleable]]).
 *
 * Note that some radio groups allow multiple radio buttons to be in the `on`
 * state at the same type **if** they represent the same underlying value (see
 * [[PDFRadioGroup.isMutuallyExclusive]]).
 */
export default class PDFRadioGroup extends PDFField {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFForm.getOptionList]] method, which will create an
     * > instance of [[PDFOptionList]] for you.
     *
     * Create an instance of [[PDFOptionList]] from an existing acroRadioButton
     * and ref
     *
     * @param acroRadioButton The underlying `PDFAcroRadioButton` for this
     *                        radio group.
     * @param ref The unique reference for this radio group.
     * @param doc The document to which this radio group will belong.
     */
    static of: (acroRadioButton: PDFAcroRadioButton, ref: PDFRef, doc: PDFDocument) => PDFRadioGroup;
    /** The low-level PDFAcroRadioButton wrapped by this radio group. */
    readonly acroField: PDFAcroRadioButton;
    private constructor();
    /**
     * Get the list of available options for this radio group. Each option is
     * represented by a radio button. These radio buttons are displayed at
     * various locations in the document, potentially on different pages (though
     * typically they are stacked horizontally or vertically on the same page).
     * For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * const options = radioGroup.getOptions()
     * console.log('Radio Group options:', options)
     * ```
     * @returns The options for this radio group.
     */
    getOptions(): string[];
    /**
     * Get the selected option for this radio group. The selected option is
     * represented by the radio button in this group that is turned on. At most
     * one radio button in a group can be selected. If no buttons in this group
     * are selected, `undefined` is returned.
     * For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * const selected = radioGroup.getSelected()
     * console.log('Selected radio button:', selected)
     * ```
     * @returns The selected option for this radio group.
     */
    getSelected(): string | undefined;
    /**
     * Select an option for this radio group. This operation is analogous to a
     * human user clicking one of the radio buttons in this group via a PDF
     * reader to toggle it on. This method will update the underlying state of
     * the radio group to indicate which option has been selected. PDF libraries
     * and readers will be able to extract this value from the saved document and
     * determine which option was selected.
     *
     * For example:
     * ```js
     * const radioGroup = form.getRadioGroup('best.superhero.radioGroup')
     * radioGroup.select('One Punch Man')
     * ```
     *
     * This method will mark this radio group as dirty, causing its appearance
     * streams to be updated when either [[PDFDocument.save]] or
     * [[PDFForm.updateFieldAppearances]] is called. The updated appearance
     * streams will display a dot inside the widget of this check box field
     * that represents the selected option.
     *
     * @param option The option to be selected.
     */
    select(option: string): void;
    /**
     * Clear any selected option for this dropdown. This will result in all
     * radio buttons in this group being toggled off. This method will update
     * the underlying state of the dropdown to indicate that no radio buttons
     * have been selected.
     * For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * radioGroup.clear()
     * ```
     * This method will mark this radio group as dirty. See
     * [[PDFRadioGroup.select]] for more details about what this means.
     */
    clear(): void;
    /**
     * Returns `true` if users can click on radio buttons in this group to toggle
     * them off. The alternative is that once a user clicks on a radio button
     * to select it, the only way to deselect it is by selecting on another radio
     * button in the group. See [[PDFRadioGroup.enableOffToggling]] and
     * [[PDFRadioGroup.disableOffToggling]]. For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * if (radioGroup.isOffToggleable()) console.log('Off toggling is enabled')
     * ```
     */
    isOffToggleable(): boolean;
    /**
     * Allow users to click on selected radio buttons in this group to toggle
     * them off. For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * radioGroup.enableOffToggling()
     * ```
     * > **NOTE:** This feature is documented in the PDF specification
     * > (Table 226). However, most PDF readers do not respect this option and
     * > prevent users from toggling radio buttons off even when it is enabled.
     * > At the time of this writing (9/6/2020) Mac's Preview software did
     * > respect the option. Adobe Acrobat, Foxit Reader, and Google Chrome did
     * > not.
     */
    enableOffToggling(): void;
    /**
     * Prevent users from clicking on selected radio buttons in this group to
     * toggle them off. Clicking on a selected radio button will have no effect.
     * The only way to deselect a selected radio button is to click on a
     * different radio button in the group. For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * radioGroup.disableOffToggling()
     * ```
     */
    disableOffToggling(): void;
    /**
     * Returns `true` if the radio buttons in this group are mutually exclusive.
     * This means that when the user selects a radio button, only that specific
     * button will be turned on. Even if other radio buttons in the group
     * represent the same value, they will not be enabled. The alternative to
     * this is that clicking a radio button will select that button along with
     * any other radio buttons in the group that share the same value. See
     * [[PDFRadioGroup.enableMutualExclusion]] and
     * [[PDFRadioGroup.disableMutualExclusion]].
     * For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * if (radioGroup.isMutuallyExclusive()) console.log('Mutual exclusion is enabled')
     * ```
     */
    isMutuallyExclusive(): boolean;
    /**
     * When the user clicks a radio button in this group it will be selected. In
     * addition, any other radio buttons in this group that share the same
     * underlying value will also be selected. For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * radioGroup.enableMutualExclusion()
     * ```
     * Note that this option must be enabled prior to adding options to the
     * radio group. It does not currently apply retroactively to existing
     * radio buttons in the group.
     */
    enableMutualExclusion(): void;
    /**
     * When the user clicks a radio button in this group only it will be selected.
     * No other radio buttons in the group will be selected, even if they share
     * the same underlying value. For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * radioGroup.disableMutualExclusion()
     * ```
     * Note that this option must be disabled prior to adding options to the
     * radio group. It does not currently apply retroactively to existing
     * radio buttons in the group.
     */
    disableMutualExclusion(): void;
    /**
     * Add a new radio button to this group on the specified page. For example:
     * ```js
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const radioGroup = form.createRadioGroup('best.gundam')
     *
     * const options = {
     *   x: 50,
     *   width: 25,
     *   height: 25,
     *   textColor: rgb(1, 0, 0),
     *   backgroundColor: rgb(0, 1, 0),
     *   borderColor: rgb(0, 0, 1),
     *   borderWidth: 2,
     *   rotate: degrees(90),
     * }
     *
     * radioGroup.addOptionToPage('Exia', page, { ...options, y: 50 })
     * radioGroup.addOptionToPage('Dynames', page, { ...options, y: 110 })
     * ```
     * This will create a new radio button widget for this radio group field.
     * @param option The option that the radio button widget represents.
     * @param page The page to which the radio button widget should be added.
     * @param options The options to be used when adding the radio button widget.
     */
    addOptionToPage(option: string, page: PDFPage, options?: FieldAppearanceOptions): void;
    /**
     * Returns `true` if any of this group's radio button widgets do not have an
     * appearance stream for their current state. For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * if (radioGroup.needsAppearancesUpdate()) console.log('Needs update')
     * ```
     * @returns Whether or not this radio group needs an appearance update.
     */
    needsAppearancesUpdate(): boolean;
    /**
     * Update the appearance streams for each of this group's radio button widgets
     * using the default appearance provider for radio groups. For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * radioGroup.defaultUpdateAppearances()
     * ```
     */
    defaultUpdateAppearances(): void;
    /**
     * Update the appearance streams for each of this group's radio button widgets
     * using the given appearance provider. If no `provider` is passed, the
     * default appearance provider for radio groups will be used. For example:
     * ```js
     * const radioGroup = form.getRadioGroup('some.radioGroup.field')
     * radioGroup.updateAppearances((field, widget) => {
     *   ...
     *   return {
     *     normal: { on: drawRadioButton(...), off: drawRadioButton(...) },
     *     down: { on: drawRadioButton(...), off: drawRadioButton(...) },
     *   }
     * })
     * ```
     * @param provider Optionally, the appearance provider to be used for
     *                 generating the contents of the appearance streams.
     */
    updateAppearances(provider?: AppearanceProviderFor<PDFRadioGroup>): void;
    private updateWidgetAppearance;
}
//# sourceMappingURL=PDFRadioGroup.d.ts.map
```

### es/api/form/PDFSignature.d.ts

```ts
import PDFDocument from "../PDFDocument";
import PDFField from "./PDFField";
import { PDFRef, PDFAcroSignature } from "../../core";
/**
 * Represents a signature field of a [[PDFForm]].
 *
 * [[PDFSignature]] fields are digital signatures. `pdf-lib` does not
 * currently provide any specialized APIs for creating digital signatures or
 * reading the contents of existing digital signatures.
 */
export default class PDFSignature extends PDFField {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFForm.getSignature]] method, which will create an
     * > instance of [[PDFSignature]] for you.
     *
     * Create an instance of [[PDFSignature]] from an existing acroSignature and
     * ref
     *
     * @param acroSignature The underlying `PDFAcroSignature` for this signature.
     * @param ref The unique reference for this signature.
     * @param doc The document to which this signature will belong.
     */
    static of: (acroSignature: PDFAcroSignature, ref: PDFRef, doc: PDFDocument) => PDFSignature;
    /** The low-level PDFAcroSignature wrapped by this signature. */
    readonly acroField: PDFAcroSignature;
    private constructor();
    needsAppearancesUpdate(): boolean;
}
//# sourceMappingURL=PDFSignature.d.ts.map
```

### es/api/form/PDFTextField.d.ts

```ts
import PDFDocument from "../PDFDocument";
import PDFPage from "../PDFPage";
import PDFFont from "../PDFFont";
import PDFImage from "../PDFImage";
import PDFField, { FieldAppearanceOptions } from "./PDFField";
import { AppearanceProviderFor } from "./appearances";
import { TextAlignment } from "../text/alignment";
import { PDFRef, PDFAcroText } from "../../core";
/**
 * Represents a text field of a [[PDFForm]].
 *
 * [[PDFTextField]] fields are boxes that display text entered by the user. The
 * purpose of a text field is to enable users to enter text or view text values
 * in the document prefilled by software. Users can click on a text field and
 * input text via their keyboard. Some text fields allow multiple lines of text
 * to be entered (see [[PDFTextField.isMultiline]]).
 */
export default class PDFTextField extends PDFField {
    /**
     * > **NOTE:** You probably don't want to call this method directly. Instead,
     * > consider using the [[PDFForm.getTextField]] method, which will create an
     * > instance of [[PDFTextField]] for you.
     *
     * Create an instance of [[PDFTextField]] from an existing acroText and ref
     *
     * @param acroText The underlying `PDFAcroText` for this text field.
     * @param ref The unique reference for this text field.
     * @param doc The document to which this text field will belong.
     */
    static of: (acroText: PDFAcroText, ref: PDFRef, doc: PDFDocument) => PDFTextField;
    /** The low-level PDFAcroText wrapped by this text field. */
    readonly acroField: PDFAcroText;
    private constructor();
    /**
     * Get the text that this field contains. This text is visible to users who
     * view this field in a PDF reader.
     *
     * For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * const text = textField.getText()
     * console.log('Text field contents:', text)
     * ```
     *
     * Note that if this text field contains no underlying value, `undefined`
     * will be returned. Text fields may also contain an underlying value that
     * is simply an empty string (`''`). This detail is largely irrelevant for
     * most applications. In general, you'll want to treat both cases the same
     * way and simply consider the text field to be empty. In either case, the
     * text field will appear empty to users when viewed in a PDF reader.
     *
     * An error will be thrown if this is a rich text field. `pdf-lib` does not
     * support reading rich text fields. Nor do most PDF readers and writers.
     * Rich text fields are based on XFA (XML Forms Architecture). Relatively few
     * PDFs use rich text fields or XFA. Unlike PDF itself, XFA is not an ISO
     * standard. XFA has been deprecated in PDF 2.0:
     * * https://en.wikipedia.org/wiki/XFA
     * * http://blog.pdfshareforms.com/pdf-2-0-release-bid-farewell-xfa-forms/
     *
     * @returns The text contained in this text field.
     */
    getText(): string | undefined;
    /**
     * Set the text for this field. This operation is analogous to a human user
     * clicking on the text field in a PDF reader and typing in text via their
     * keyboard. This method will update the underlying state of the text field
     * to indicate what text has been set. PDF libraries and readers will be able
     * to extract these values from the saved document and determine what text
     * was set.
     *
     * For example:
     * ```js
     * const textField = form.getTextField('best.superhero.text.field')
     * textField.setText('One Punch Man')
     * ```
     *
     * This method will mark this text field as dirty, causing its appearance
     * streams to be updated when either [[PDFDocument.save]] or
     * [[PDFForm.updateFieldAppearances]] is called. The updated streams will
     * display the text this field contains inside the widgets of this text
     * field.
     *
     * **IMPORTANT:** The default font used to update appearance streams is
     * [[StandardFonts.Helvetica]]. Note that this is a WinAnsi font. This means
     * that encoding errors will be thrown if this field contains text outside
     * the WinAnsi character set (the latin alphabet).
     *
     * Embedding a custom font and passing it to
     * [[PDFForm.updateFieldAppearances]] or [[PDFTextField.updateAppearances]]
     * allows you to generate appearance streams with characters outside the
     * latin alphabet (assuming the custom font supports them).
     *
     * If this is a rich text field, it will be converted to a standard text
     * field in order to set the text. `pdf-lib` does not support writing rich
     * text strings. Nor do most PDF readers and writers. See
     * [[PDFTextField.getText]] for more information about rich text fields and
     * their deprecation in PDF 2.0.
     *
     * @param text The text this field should contain.
     */
    setText(text: string | undefined): void;
    /**
     * Get the alignment for this text field. This value represents the
     * justification of the text when it is displayed to the user in PDF readers.
     * There are three possible alignments: left, center, and right. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * const alignment = textField.getAlignment()
     * if (alignment === TextAlignment.Left) console.log('Text is left justified')
     * if (alignment === TextAlignment.Center) console.log('Text is centered')
     * if (alignment === TextAlignment.Right) console.log('Text is right justified')
     * ```
     * @returns The alignment of this text field.
     */
    getAlignment(): TextAlignment;
    /**
     * Set the alignment for this text field. This will determine the
     * justification of the text when it is displayed to the user in PDF readers.
     * There are three possible alignments: left, center, and right. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     *
     * // Text will be left justified when displayed
     * textField.setAlignment(TextAlignment.Left)
     *
     * // Text will be centered when displayed
     * textField.setAlignment(TextAlignment.Center)
     *
     * // Text will be right justified when displayed
     * textField.setAlignment(TextAlignment.Right)
     * ```
     * This method will mark this text field as dirty. See
     * [[PDFTextField.setText]] for more details about what this means.
     * @param alignment The alignment for this text field.
     */
    setAlignment(alignment: TextAlignment): void;
    /**
     * Get the maximum length of this field. This value represents the maximum
     * number of characters that can be typed into this field by the user. If
     * this field does not have a maximum length, `undefined` is returned.
     * For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * const maxLength = textField.getMaxLength()
     * if (maxLength === undefined) console.log('No max length')
     * else console.log(`Max length is ${maxLength}`)
     * ```
     * @returns The maximum number of characters allowed in this field, or
     *          `undefined` if no limit exists.
     */
    getMaxLength(): number | undefined;
    /**
     * Set the maximum length of this field. This limits the number of characters
     * that can be typed into this field by the user. This also limits the length
     * of the string that can be passed to [[PDFTextField.setText]]. This limit
     * can be removed by passing `undefined` as `maxLength`. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     *
     * // Allow between 0 and 5 characters to be entered
     * textField.setMaxLength(5)
     *
     * // Allow any number of characters to be entered
     * textField.setMaxLength(undefined)
     * ```
     * This method will mark this text field as dirty. See
     * [[PDFTextField.setText]] for more details about what this means.
     * @param maxLength The maximum number of characters allowed in this field, or
     *                  `undefined` to remove the limit.
     */
    setMaxLength(maxLength?: number): void;
    /**
     * Remove the maximum length for this text field. This allows any number of
     * characters to be typed into this field by the user. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.removeMaxLength()
     * ```
     * Calling this method is equivalent to passing `undefined` to
     * [[PDFTextField.setMaxLength]].
     */
    removeMaxLength(): void;
    /**
     * Display an image inside the bounds of this text field's widgets. For example:
     * ```js
     * const pngImage = await pdfDoc.embedPng(...)
     * const textField = form.getTextField('some.text.field')
     * textField.setImage(pngImage)
     * ```
     * This will update the appearances streams for each of this text field's widgets.
     * @param image The image that should be displayed.
     */
    setImage(image: PDFImage): void;
    /**
     * Set the font size for this field. Larger font sizes will result in larger
     * text being displayed when PDF readers render this text field. Font sizes
     * may be integer or floating point numbers. Supplying a negative font size
     * will cause this method to throw an error.
     *
     * For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.setFontSize(4)
     * textField.setFontSize(15.7)
     * ```
     *
     * > This method depends upon the existence of a default appearance
     * > (`/DA`) string. If this field does not have a default appearance string,
     * > or that string does not contain a font size (via the `Tf` operator),
     * > then this method will throw an error.
     *
     * @param fontSize The font size to be used when rendering text in this field.
     */
    setFontSize(fontSize: number): void;
    /**
     * Returns `true` if each line of text is shown on a new line when this
     * field is displayed in a PDF reader. The alternative is that all lines of
     * text are merged onto a single line when displayed. See
     * [[PDFTextField.enableMultiline]] and [[PDFTextField.disableMultiline]].
     * For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * if (textField.isMultiline()) console.log('Multiline is enabled')
     * ```
     * @returns Whether or not this is a multiline text field.
     */
    isMultiline(): boolean;
    /**
     * Display each line of text on a new line when this field is displayed in a
     * PDF reader. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.enableMultiline()
     * ```
     * This method will mark this text field as dirty. See
     * [[PDFTextField.setText]] for more details about what this means.
     */
    enableMultiline(): void;
    /**
     * Display each line of text on the same line when this field is displayed
     * in a PDF reader. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.disableMultiline()
     * ```
     * This method will mark this text field as dirty. See
     * [[PDFTextField.setText]] for more details about what this means.
     */
    disableMultiline(): void;
    /**
     * Returns `true` if this is a password text field. This means that the field
     * is intended for storing a secure password. See
     * [[PDFTextField.enablePassword]] and [[PDFTextField.disablePassword]].
     * For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * if (textField.isPassword()) console.log('Password is enabled')
     * ```
     * @returns Whether or not this is a password text field.
     */
    isPassword(): boolean;
    /**
     * Indicate that this text field is intended for storing a secure password.
     * For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.enablePassword()
     * ```
     * Values entered into password text fields should not be displayed on the
     * screen by PDF readers. Most PDF readers will display the value as
     * asterisks or bullets. PDF readers should never store values entered by the
     * user into password text fields. Similarly, applications should not
     * write data to a password text field.
     *
     * **Please note that this method does not cause entered values to be
     * encrypted or secured in any way! It simply sets a flag that PDF software
     * and readers can access to determine the _purpose_ of this field.**
     */
    enablePassword(): void;
    /**
     * Indicate that this text field is **not** intended for storing a secure
     * password. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.disablePassword()
     * ```
     */
    disablePassword(): void;
    /**
     * Returns `true` if the contents of this text field represent a file path.
     * See [[PDFTextField.enableFileSelection]] and
     * [[PDFTextField.disableFileSelection]]. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * if (textField.isFileSelector()) console.log('Is a file selector')
     * ```
     * @returns Whether or not this field should contain file paths.
     */
    isFileSelector(): boolean;
    /**
     * Indicate that this text field is intended to store a file path. The
     * contents of the file stored at that path should be submitted as the value
     * of the field. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.enableFileSelection()
     * ```
     */
    enableFileSelection(): void;
    /**
     * Indicate that this text field is **not** intended to store a file path.
     * For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.disableFileSelection()
     * ```
     */
    disableFileSelection(): void;
    /**
     * Returns `true` if the text entered in this field should be spell checked
     * by PDF readers. See [[PDFTextField.enableSpellChecking]] and
     * [[PDFTextField.disableSpellChecking]]. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * if (textField.isSpellChecked()) console.log('Spell checking is enabled')
     * ```
     * @returns Whether or not this field should be spell checked.
     */
    isSpellChecked(): boolean;
    /**
     * Allow PDF readers to spell check the text entered in this field.
     * For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.enableSpellChecking()
     * ```
     */
    enableSpellChecking(): void;
    /**
     * Do not allow PDF readers to spell check the text entered in this field.
     * For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.disableSpellChecking()
     * ```
     */
    disableSpellChecking(): void;
    /**
     * Returns `true` if PDF readers should allow the user to scroll the text
     * field when its contents do not fit within the field's view bounds. See
     * [[PDFTextField.enableScrolling]] and [[PDFTextField.disableScrolling]].
     * For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * if (textField.isScrollable()) console.log('Scrolling is enabled')
     * ```
     * @returns Whether or not the field is scrollable in PDF readers.
     */
    isScrollable(): boolean;
    /**
     * Allow PDF readers to present a scroll bar to the user when the contents
     * of this text field do not fit within its view bounds. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.enableScrolling()
     * ```
     * A horizontal scroll bar should be shown for singleline fields. A vertical
     * scroll bar should be shown for multiline fields.
     */
    enableScrolling(): void;
    /**
     * Do not allow PDF readers to present a scroll bar to the user when the
     * contents of this text field do not fit within its view bounds. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.disableScrolling()
     * ```
     */
    disableScrolling(): void;
    /**
     * Returns `true` if this is a combed text field. This means that the field
     * is split into `n` equal size cells with one character in each (where `n`
     * is equal to the max length of the text field). The result is that all
     * characters in this field are displayed an equal distance apart from one
     * another. See [[PDFTextField.enableCombing]] and
     * [[PDFTextField.disableCombing]]. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * if (textField.isCombed()) console.log('Combing is enabled')
     * ```
     * Note that in order for a text field to be combed, the following must be
     * true (in addition to enabling combing):
     * * It must not be a multiline field (see [[PDFTextField.isMultiline]])
     * * It must not be a password field (see [[PDFTextField.isPassword]])
     * * It must not be a file selector field (see [[PDFTextField.isFileSelector]])
     * * It must have a max length defined (see [[PDFTextField.setMaxLength]])
     * @returns Whether or not this field is combed.
     */
    isCombed(): boolean;
    /**
     * Split this field into `n` equal size cells with one character in each
     * (where `n` is equal to the max length of the text field). This will cause
     * all characters in the field to be displayed an equal distance apart from
     * one another. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.enableCombing()
     * ```
     *
     * In addition to calling this method, text fields must have a max length
     * defined in order to be combed (see [[PDFTextField.setMaxLength]]).
     *
     * This method will also call the following three methods internally:
     * * [[PDFTextField.disableMultiline]]
     * * [[PDFTextField.disablePassword]]
     * * [[PDFTextField.disableFileSelection]]
     *
     * This method will mark this text field as dirty. See
     * [[PDFTextField.setText]] for more details about what this means.
     */
    enableCombing(): void;
    /**
     * Turn off combing for this text field. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.disableCombing()
     * ```
     * See [[PDFTextField.isCombed]] and [[PDFTextField.enableCombing]] for more
     * information about what combing is.
     *
     * This method will mark this text field as dirty. See
     * [[PDFTextField.setText]] for more details about what this means.
     */
    disableCombing(): void;
    /**
     * Returns `true` if this text field contains rich text. See
     * [[PDFTextField.enableRichFormatting]] and
     * [[PDFTextField.disableRichFormatting]]. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * if (textField.isRichFormatted()) console.log('Rich formatting enabled')
     * ```
     * @returns Whether or not this field contains rich text.
     */
    isRichFormatted(): boolean;
    /**
     * Indicate that this field contains XFA data - or rich text. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.enableRichFormatting()
     * ```
     * Note that `pdf-lib` does not support reading or writing rich text fields.
     * Nor do most PDF readers and writers. Rich text fields are based on XFA
     * (XML Forms Architecture). Relatively few PDFs use rich text fields or XFA.
     * Unlike PDF itself, XFA is not an ISO standard. XFA has been deprecated in
     * PDF 2.0:
     * * https://en.wikipedia.org/wiki/XFA
     * * http://blog.pdfshareforms.com/pdf-2-0-release-bid-farewell-xfa-forms/
     */
    enableRichFormatting(): void;
    /**
     * Indicate that this is a standard text field that does not XFA data (rich
     * text). For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * textField.disableRichFormatting()
     * ```
     */
    disableRichFormatting(): void;
    /**
     * Show this text field on the specified page. For example:
     * ```js
     * const ubuntuFont = await pdfDoc.embedFont(ubuntuFontBytes)
     * const page = pdfDoc.addPage()
     *
     * const form = pdfDoc.getForm()
     * const textField = form.createTextField('best.gundam')
     * textField.setText('Exia')
     *
     * textField.addToPage(page, {
     *   x: 50,
     *   y: 75,
     *   width: 200,
     *   height: 100,
     *   textColor: rgb(1, 0, 0),
     *   backgroundColor: rgb(0, 1, 0),
     *   borderColor: rgb(0, 0, 1),
     *   borderWidth: 2,
     *   rotate: degrees(90),
     *   font: ubuntuFont,
     * })
     * ```
     * This will create a new widget for this text field.
     * @param page The page to which this text field widget should be added.
     * @param options The options to be used when adding this text field widget.
     */
    addToPage(page: PDFPage, options?: FieldAppearanceOptions): void;
    /**
     * Returns `true` if this text field has been marked as dirty, or if any of
     * this text field's widgets do not have an appearance stream. For example:
     * ```js
     * const textField = form.getTextField('some.text.field')
     * if (textField.needsAppearancesUpdate()) console.log('Needs update')
     * ```
     * @returns Whether or not this text field needs an appearance update.
     */
    needsAppearancesUpdate(): boolean;
    /**
     * Update the appearance streams for each of this text field's widgets using
     * the default appearance provider for text fields. For example:
     * ```js
     * const helvetica = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const textField = form.getTextField('some.text.field')
     * textField.defaultUpdateAppearances(helvetica)
     * ```
     * @param font The font to be used for creating the appearance streams.
     */
    defaultUpdateAppearances(font: PDFFont): void;
    /**
     * Update the appearance streams for each of this text field's widgets using
     * the given appearance provider. If no `provider` is passed, the default
     * appearance provider for text fields will be used. For example:
     * ```js
     * const helvetica = await pdfDoc.embedFont(StandardFonts.Helvetica)
     * const textField = form.getTextField('some.text.field')
     * textField.updateAppearances(helvetica, (field, widget, font) => {
     *   ...
     *   return drawTextField(...)
     * })
     * ```
     * @param font The font to be used for creating the appearance streams.
     * @param provider Optionally, the appearance provider to be used for
     *                 generating the contents of the appearance streams.
     */
    updateAppearances(font: PDFFont, provider?: AppearanceProviderFor<PDFTextField>): void;
    private updateWidgetAppearance;
}
//# sourceMappingURL=PDFTextField.d.ts.map
```

### es/api/form/appearances.d.ts

```ts
import { PDFOperator, PDFWidgetAnnotation } from "../../core";
import PDFFont from "../PDFFont";
import PDFButton from "./PDFButton";
import PDFCheckBox from "./PDFCheckBox";
import PDFDropdown from "./PDFDropdown";
import PDFField from "./PDFField";
import PDFOptionList from "./PDFOptionList";
import PDFRadioGroup from "./PDFRadioGroup";
import PDFSignature from "./PDFSignature";
import PDFTextField from "./PDFTextField";
/*********************** Appearance Provider Types ****************************/
declare type CheckBoxAppearanceProvider = (checkBox: PDFCheckBox, widget: PDFWidgetAnnotation) => AppearanceOrMapping<{
    on: PDFOperator[];
    off: PDFOperator[];
}>;
declare type RadioGroupAppearanceProvider = (radioGroup: PDFRadioGroup, widget: PDFWidgetAnnotation) => AppearanceOrMapping<{
    on: PDFOperator[];
    off: PDFOperator[];
}>;
declare type ButtonAppearanceProvider = (button: PDFButton, widget: PDFWidgetAnnotation, font: PDFFont) => AppearanceOrMapping<PDFOperator[]>;
declare type DropdownAppearanceProvider = (dropdown: PDFDropdown, widget: PDFWidgetAnnotation, font: PDFFont) => AppearanceOrMapping<PDFOperator[]>;
declare type OptionListAppearanceProvider = (optionList: PDFOptionList, widget: PDFWidgetAnnotation, font: PDFFont) => AppearanceOrMapping<PDFOperator[]>;
declare type TextFieldAppearanceProvider = (textField: PDFTextField, widget: PDFWidgetAnnotation, font: PDFFont) => AppearanceOrMapping<PDFOperator[]>;
declare type SignatureAppearanceProvider = (signature: PDFSignature, widget: PDFWidgetAnnotation, font: PDFFont) => AppearanceOrMapping<PDFOperator[]>;
/******************* Appearance Provider Utility Types ************************/
export declare type AppearanceMapping<T> = {
    normal: T;
    rollover?: T;
    down?: T;
};
declare type AppearanceOrMapping<T> = T | AppearanceMapping<T>;
export declare type AppearanceProviderFor<T extends PDFField> = T extends PDFCheckBox ? CheckBoxAppearanceProvider : T extends PDFRadioGroup ? RadioGroupAppearanceProvider : T extends PDFButton ? ButtonAppearanceProvider : T extends PDFDropdown ? DropdownAppearanceProvider : T extends PDFOptionList ? OptionListAppearanceProvider : T extends PDFTextField ? TextFieldAppearanceProvider : T extends PDFSignature ? SignatureAppearanceProvider : never;
/********************* Appearance Provider Functions **************************/
export declare const normalizeAppearance: <T>(appearance: T | AppearanceMapping<T>) => AppearanceMapping<T>;
export declare const defaultCheckBoxAppearanceProvider: AppearanceProviderFor<PDFCheckBox>;
export declare const defaultRadioGroupAppearanceProvider: AppearanceProviderFor<PDFRadioGroup>;
export declare const defaultButtonAppearanceProvider: AppearanceProviderFor<PDFButton>;
export declare const defaultTextFieldAppearanceProvider: AppearanceProviderFor<PDFTextField>;
export declare const defaultDropdownAppearanceProvider: AppearanceProviderFor<PDFDropdown>;
export declare const defaultOptionListAppearanceProvider: AppearanceProviderFor<PDFOptionList>;
export {};
//# sourceMappingURL=appearances.d.ts.map
```

### es/api/form/index.d.ts

```ts
export * from "./appearances";
export { default as PDFButton } from "./PDFButton";
export { default as PDFCheckBox } from "./PDFCheckBox";
export { default as PDFDropdown } from "./PDFDropdown";
export { default as PDFField } from "./PDFField";
export { default as PDFForm } from "./PDFForm";
export { default as PDFOptionList } from "./PDFOptionList";
export { default as PDFRadioGroup } from "./PDFRadioGroup";
export { default as PDFSignature } from "./PDFSignature";
export { default as PDFTextField } from "./PDFTextField";
//# sourceMappingURL=index.d.ts.map
```

### es/api/image/alignment.d.ts

```ts
export declare enum ImageAlignment {
    Left = 0,
    Center = 1,
    Right = 2
}
//# sourceMappingURL=alignment.d.ts.map
```

### es/api/image/index.d.ts

```ts
export * from "./alignment";
//# sourceMappingURL=index.d.ts.map
```

### es/api/index.d.ts

```ts
export * from "./form";
export * from "./text";
export * from "./colors";
export * from "./errors";
export * from "./image";
export * from "./objects";
export * from "./operations";
export * from "./operators";
export * from "./rotations";
export * from "./sizes";
export * from "./PDFPageOptions";
export * from "./PDFDocumentOptions";
export * from "./StandardFonts";
export { default as PDFDocument } from "./PDFDocument";
export { default as PDFFont } from "./PDFFont";
export { default as PDFImage } from "./PDFImage";
export { default as PDFPage } from "./PDFPage";
export { default as PDFEmbeddedPage } from "./PDFEmbeddedPage";
export { default as PDFJavaScript } from "./PDFJavaScript";
export { default as Embeddable } from "./Embeddable";
//# sourceMappingURL=index.d.ts.map
```

### es/api/objects.d.ts

```ts
import { PDFName, PDFNumber } from "../core";
export declare const asPDFName: (name: string | PDFName) => PDFName;
export declare const asPDFNumber: (num: number | PDFNumber) => PDFNumber;
export declare const asNumber: (num: number | PDFNumber) => number;
//# sourceMappingURL=objects.d.ts.map
```

### es/api/operations.d.ts

```ts
import { Color } from "./colors";
import { scale, LineCapStyle } from "./operators";
import { Rotation } from "./rotations";
import { PDFHexString, PDFName, PDFNumber, PDFOperator } from "../core";
export interface DrawTextOptions {
    color: Color;
    font: string | PDFName;
    size: number | PDFNumber;
    rotate: Rotation;
    xSkew: Rotation;
    ySkew: Rotation;
    x: number | PDFNumber;
    y: number | PDFNumber;
    graphicsState?: string | PDFName;
}
export declare const drawText: (line: PDFHexString, options: DrawTextOptions) => PDFOperator[];
export interface DrawLinesOfTextOptions extends DrawTextOptions {
    lineHeight: number | PDFNumber;
}
export declare const drawLinesOfText: (lines: PDFHexString[], options: DrawLinesOfTextOptions) => PDFOperator[];
export declare const drawImage: (name: string | PDFName, options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    width: number | PDFNumber;
    height: number | PDFNumber;
    rotate: Rotation;
    xSkew: Rotation;
    ySkew: Rotation;
    graphicsState?: string | PDFName;
}) => PDFOperator[];
export declare const drawPage: (name: string | PDFName, options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    xScale: number | PDFNumber;
    yScale: number | PDFNumber;
    rotate: Rotation;
    xSkew: Rotation;
    ySkew: Rotation;
    graphicsState?: string | PDFName;
}) => PDFOperator[];
export declare const drawLine: (options: {
    start: {
        x: number | PDFNumber;
        y: number | PDFNumber;
    };
    end: {
        x: number | PDFNumber;
        y: number | PDFNumber;
    };
    thickness: number | PDFNumber;
    color: Color | undefined;
    dashArray?: (number | PDFNumber)[];
    dashPhase?: number | PDFNumber;
    lineCap?: LineCapStyle;
    graphicsState?: string | PDFName;
}) => PDFOperator[];
export declare const drawRectangle: (options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    width: number | PDFNumber;
    height: number | PDFNumber;
    borderWidth: number | PDFNumber;
    color: Color | undefined;
    borderColor: Color | undefined;
    rotate: Rotation;
    xSkew: Rotation;
    ySkew: Rotation;
    borderLineCap?: LineCapStyle;
    borderDashArray?: (number | PDFNumber)[];
    borderDashPhase?: number | PDFNumber;
    graphicsState?: string | PDFName;
}) => PDFOperator[];
/** @deprecated */
export declare const drawEllipsePath: (config: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    xScale: number | PDFNumber;
    yScale: number | PDFNumber;
}) => PDFOperator[];
export declare const drawEllipse: (options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    xScale: number | PDFNumber;
    yScale: number | PDFNumber;
    rotate?: Rotation;
    color: Color | undefined;
    borderColor: Color | undefined;
    borderWidth: number | PDFNumber;
    borderDashArray?: (number | PDFNumber)[];
    borderDashPhase?: number | PDFNumber;
    graphicsState?: string | PDFName;
    borderLineCap?: LineCapStyle;
}) => PDFOperator[];
export declare const drawSvgPath: (path: string, options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    rotate?: Rotation;
    scale: number | PDFNumber | undefined;
    color: Color | undefined;
    borderColor: Color | undefined;
    borderWidth: number | PDFNumber;
    borderDashArray?: (number | PDFNumber)[];
    borderDashPhase?: number | PDFNumber;
    borderLineCap?: LineCapStyle;
    graphicsState?: string | PDFName;
}) => PDFOperator[];
export declare const drawCheckMark: (options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    size: number | PDFNumber;
    thickness: number | PDFNumber;
    color: Color | undefined;
}) => PDFOperator[];
export declare const rotateInPlace: (options: {
    width: number | PDFNumber;
    height: number | PDFNumber;
    rotation: 0 | 90 | 180 | 270;
}) => PDFOperator[];
export declare const drawCheckBox: (options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    width: number | PDFNumber;
    height: number | PDFNumber;
    thickness: number | PDFNumber;
    borderWidth: number | PDFNumber;
    markColor: Color | undefined;
    color: Color | undefined;
    borderColor: Color | undefined;
    filled: boolean;
}) => PDFOperator[];
export declare const drawRadioButton: (options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    width: number | PDFNumber;
    height: number | PDFNumber;
    borderWidth: number | PDFNumber;
    dotColor: Color | undefined;
    color: Color | undefined;
    borderColor: Color | undefined;
    filled: boolean;
}) => PDFOperator[];
export declare const drawButton: (options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    width: number | PDFNumber;
    height: number | PDFNumber;
    borderWidth: number | PDFNumber;
    color: Color | undefined;
    borderColor: Color | undefined;
    textLines: {
        encoded: PDFHexString;
        x: number;
        y: number;
    }[];
    textColor: Color;
    font: string | PDFName;
    fontSize: number | PDFNumber;
}) => PDFOperator[];
export interface DrawTextLinesOptions {
    color: Color;
    font: string | PDFName;
    size: number | PDFNumber;
    rotate: Rotation;
    xSkew: Rotation;
    ySkew: Rotation;
}
export declare const drawTextLines: (lines: {
    encoded: PDFHexString;
    x: number;
    y: number;
}[], options: DrawTextLinesOptions) => PDFOperator[];
export declare const drawTextField: (options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    width: number | PDFNumber;
    height: number | PDFNumber;
    borderWidth: number | PDFNumber;
    color: Color | undefined;
    borderColor: Color | undefined;
    textLines: {
        encoded: PDFHexString;
        x: number;
        y: number;
    }[];
    textColor: Color;
    font: string | PDFName;
    fontSize: number | PDFNumber;
    padding: number | PDFNumber;
}) => PDFOperator[];
export declare const drawOptionList: (options: {
    x: number | PDFNumber;
    y: number | PDFNumber;
    width: number | PDFNumber;
    height: number | PDFNumber;
    borderWidth: number | PDFNumber;
    color: Color | undefined;
    borderColor: Color | undefined;
    textLines: {
        encoded: PDFHexString;
        x: number;
        y: number;
        height: number;
    }[];
    textColor: Color;
    font: string | PDFName;
    fontSize: number | PDFNumber;
    lineHeight: number | PDFNumber;
    selectedLines: number[];
    selectedColor: Color;
    padding: number | PDFNumber;
}) => PDFOperator[];
//# sourceMappingURL=operations.d.ts.map
```

### es/api/operators.d.ts

```ts
import { PDFHexString, PDFName, PDFNumber, PDFOperator } from "../core";
export declare const clip: () => PDFOperator;
export declare const clipEvenOdd: () => PDFOperator;
export declare const concatTransformationMatrix: (a: number | PDFNumber, b: number | PDFNumber, c: number | PDFNumber, d: number | PDFNumber, e: number | PDFNumber, f: number | PDFNumber) => PDFOperator;
export declare const translate: (xPos: number | PDFNumber, yPos: number | PDFNumber) => PDFOperator;
export declare const scale: (xPos: number | PDFNumber, yPos: number | PDFNumber) => PDFOperator;
export declare const rotateRadians: (angle: number | PDFNumber) => PDFOperator;
export declare const rotateDegrees: (angle: number | PDFNumber) => PDFOperator;
export declare const skewRadians: (xSkewAngle: number | PDFNumber, ySkewAngle: number | PDFNumber) => PDFOperator;
export declare const skewDegrees: (xSkewAngle: number | PDFNumber, ySkewAngle: number | PDFNumber) => PDFOperator;
export declare const setDashPattern: (dashArray: (number | PDFNumber)[], dashPhase: number | PDFNumber) => PDFOperator;
export declare const restoreDashPattern: () => PDFOperator;
export declare enum LineCapStyle {
    Butt = 0,
    Round = 1,
    Projecting = 2
}
export declare const setLineCap: (style: LineCapStyle) => PDFOperator;
export declare enum LineJoinStyle {
    Miter = 0,
    Round = 1,
    Bevel = 2
}
export declare const setLineJoin: (style: LineJoinStyle) => PDFOperator;
export declare const setGraphicsState: (state: string | PDFName) => PDFOperator;
export declare const pushGraphicsState: () => PDFOperator;
export declare const popGraphicsState: () => PDFOperator;
export declare const setLineWidth: (width: number | PDFNumber) => PDFOperator;
export declare const appendBezierCurve: (x1: number | PDFNumber, y1: number | PDFNumber, x2: number | PDFNumber, y2: number | PDFNumber, x3: number | PDFNumber, y3: number | PDFNumber) => PDFOperator;
export declare const appendQuadraticCurve: (x1: number | PDFNumber, y1: number | PDFNumber, x2: number | PDFNumber, y2: number | PDFNumber) => PDFOperator;
export declare const closePath: () => PDFOperator;
export declare const moveTo: (xPos: number | PDFNumber, yPos: number | PDFNumber) => PDFOperator;
export declare const lineTo: (xPos: number | PDFNumber, yPos: number | PDFNumber) => PDFOperator;
/**
 * @param xPos x coordinate for the lower left corner of the rectangle
 * @param yPos y coordinate for the lower left corner of the rectangle
 * @param width width of the rectangle
 * @param height height of the rectangle
 */
export declare const rectangle: (xPos: number | PDFNumber, yPos: number | PDFNumber, width: number | PDFNumber, height: number | PDFNumber) => PDFOperator;
/**
 * @param xPos x coordinate for the lower left corner of the square
 * @param yPos y coordinate for the lower left corner of the square
 * @param size width and height of the square
 */
export declare const square: (xPos: number, yPos: number, size: number) => PDFOperator;
export declare const stroke: () => PDFOperator;
export declare const fill: () => PDFOperator;
export declare const fillAndStroke: () => PDFOperator;
export declare const endPath: () => PDFOperator;
export declare const nextLine: () => PDFOperator;
export declare const moveText: (x: number | PDFNumber, y: number | PDFNumber) => PDFOperator;
export declare const showText: (text: PDFHexString) => PDFOperator;
export declare const beginText: () => PDFOperator;
export declare const endText: () => PDFOperator;
export declare const setFontAndSize: (name: string | PDFName, size: number | PDFNumber) => PDFOperator;
export declare const setCharacterSpacing: (spacing: number | PDFNumber) => PDFOperator;
export declare const setWordSpacing: (spacing: number | PDFNumber) => PDFOperator;
/** @param squeeze horizontal character spacing */
export declare const setCharacterSqueeze: (squeeze: number | PDFNumber) => PDFOperator;
export declare const setLineHeight: (lineHeight: number | PDFNumber) => PDFOperator;
export declare const setTextRise: (rise: number | PDFNumber) => PDFOperator;
export declare enum TextRenderingMode {
    Fill = 0,
    Outline = 1,
    FillAndOutline = 2,
    Invisible = 3,
    FillAndClip = 4,
    OutlineAndClip = 5,
    FillAndOutlineAndClip = 6,
    Clip = 7
}
export declare const setTextRenderingMode: (mode: TextRenderingMode) => PDFOperator;
export declare const setTextMatrix: (a: number | PDFNumber, b: number | PDFNumber, c: number | PDFNumber, d: number | PDFNumber, e: number | PDFNumber, f: number | PDFNumber) => PDFOperator;
export declare const rotateAndSkewTextRadiansAndTranslate: (rotationAngle: number | PDFNumber, xSkewAngle: number | PDFNumber, ySkewAngle: number | PDFNumber, x: number | PDFNumber, y: number | PDFNumber) => PDFOperator;
export declare const rotateAndSkewTextDegreesAndTranslate: (rotationAngle: number | PDFNumber, xSkewAngle: number | PDFNumber, ySkewAngle: number | PDFNumber, x: number | PDFNumber, y: number | PDFNumber) => PDFOperator;
export declare const drawObject: (name: string | PDFName) => PDFOperator;
export declare const setFillingGrayscaleColor: (gray: number | PDFNumber) => PDFOperator;
export declare const setStrokingGrayscaleColor: (gray: number | PDFNumber) => PDFOperator;
export declare const setFillingRgbColor: (red: number | PDFNumber, green: number | PDFNumber, blue: number | PDFNumber) => PDFOperator;
export declare const setStrokingRgbColor: (red: number | PDFNumber, green: number | PDFNumber, blue: number | PDFNumber) => PDFOperator;
export declare const setFillingCmykColor: (cyan: number | PDFNumber, magenta: number | PDFNumber, yellow: number | PDFNumber, key: number | PDFNumber) => PDFOperator;
export declare const setStrokingCmykColor: (cyan: number | PDFNumber, magenta: number | PDFNumber, yellow: number | PDFNumber, key: number | PDFNumber) => PDFOperator;
export declare const beginMarkedContent: (tag: string | PDFName) => PDFOperator;
export declare const endMarkedContent: () => PDFOperator;
//# sourceMappingURL=operators.d.ts.map
```

### es/api/rotations.d.ts

```ts
export declare enum RotationTypes {
    Degrees = "degrees",
    Radians = "radians"
}
export interface Radians {
    type: RotationTypes.Radians;
    angle: number;
}
export interface Degrees {
    type: RotationTypes.Degrees;
    angle: number;
}
export declare type Rotation = Radians | Degrees;
export declare const radians: (radianAngle: number) => Radians;
export declare const degrees: (degreeAngle: number) => Degrees;
export declare const degreesToRadians: (degree: number) => number;
export declare const radiansToDegrees: (radian: number) => number;
export declare const toRadians: (rotation: Rotation) => number;
export declare const toDegrees: (rotation: Rotation) => number;
export declare const reduceRotation: (degreeAngle?: number) => 0 | 180 | 90 | 270;
export declare const adjustDimsForRotation: (dims: {
    width: number;
    height: number;
}, degreeAngle?: number) => {
    width: number;
    height: number;
};
export declare const rotateRectangle: (rectangle: {
    x: number;
    y: number;
    width: number;
    height: number;
}, borderWidth?: number, degreeAngle?: number) => {
    x: number;
    y: number;
    width: number;
    height: number;
};
//# sourceMappingURL=rotations.d.ts.map
```

### es/api/sizes.d.ts

```ts
export declare const PageSizes: {
    '4A0': [number, number];
    '2A0': [number, number];
    A0: [number, number];
    A1: [number, number];
    A2: [number, number];
    A3: [number, number];
    A4: [number, number];
    A5: [number, number];
    A6: [number, number];
    A7: [number, number];
    A8: [number, number];
    A9: [number, number];
    A10: [number, number];
    B0: [number, number];
    B1: [number, number];
    B2: [number, number];
    B3: [number, number];
    B4: [number, number];
    B5: [number, number];
    B6: [number, number];
    B7: [number, number];
    B8: [number, number];
    B9: [number, number];
    B10: [number, number];
    C0: [number, number];
    C1: [number, number];
    C2: [number, number];
    C3: [number, number];
    C4: [number, number];
    C5: [number, number];
    C6: [number, number];
    C7: [number, number];
    C8: [number, number];
    C9: [number, number];
    C10: [number, number];
    RA0: [number, number];
    RA1: [number, number];
    RA2: [number, number];
    RA3: [number, number];
    RA4: [number, number];
    SRA0: [number, number];
    SRA1: [number, number];
    SRA2: [number, number];
    SRA3: [number, number];
    SRA4: [number, number];
    Executive: [number, number];
    Folio: [number, number];
    Legal: [number, number];
    Letter: [number, number];
    Tabloid: [number, number];
};
//# sourceMappingURL=sizes.d.ts.map
```

### es/api/svgPath.d.ts

```ts
import { PDFOperator } from "../core";
export declare const svgPathToOperators: (path: string) => PDFOperator[];
//# sourceMappingURL=svgPath.d.ts.map
```

### es/api/text/alignment.d.ts

```ts
export declare enum TextAlignment {
    Left = 0,
    Center = 1,
    Right = 2
}
//# sourceMappingURL=alignment.d.ts.map
```

### es/api/text/index.d.ts

```ts
export * from "./alignment";
export * from "./layout";
//# sourceMappingURL=index.d.ts.map
```

### es/api/text/layout.d.ts

```ts
import PDFFont from "../PDFFont";
import { TextAlignment } from "./alignment";
import { PDFHexString } from "../../core";
export interface TextPosition {
    text: string;
    encoded: PDFHexString;
    x: number;
    y: number;
    width: number;
    height: number;
}
export interface LayoutBounds {
    x: number;
    y: number;
    width: number;
    height: number;
}
export interface LayoutTextOptions {
    alignment: TextAlignment;
    fontSize?: number;
    font: PDFFont;
    bounds: LayoutBounds;
}
export interface MultilineTextLayout {
    bounds: LayoutBounds;
    lines: TextPosition[];
    fontSize: number;
    lineHeight: number;
}
export declare const layoutMultilineText: (text: string, { alignment, fontSize, font, bounds }: LayoutTextOptions) => MultilineTextLayout;
export interface LayoutCombedTextOptions {
    fontSize?: number;
    font: PDFFont;
    bounds: LayoutBounds;
    cellCount: number;
}
export interface CombedTextLayout {
    bounds: LayoutBounds;
    cells: TextPosition[];
    fontSize: number;
}
export declare const layoutCombedText: (text: string, { fontSize, font, bounds, cellCount }: LayoutCombedTextOptions) => CombedTextLayout;
export interface LayoutSinglelineTextOptions {
    alignment: TextAlignment;
    fontSize?: number;
    font: PDFFont;
    bounds: LayoutBounds;
}
export interface SinglelineTextLayout {
    bounds: LayoutBounds;
    line: TextPosition;
    fontSize: number;
}
export declare const layoutSinglelineText: (text: string, { alignment, fontSize, font, bounds }: LayoutSinglelineTextOptions) => SinglelineTextLayout;
//# sourceMappingURL=layout.d.ts.map
```


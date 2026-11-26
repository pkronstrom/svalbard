---
name: html-app
description: Build a small interactive web app or tool (a checklist, calculator, tracker, form, quiz) as one HTML file that opens by double-click offline, with a finished look from Pico CSS, Alpine.js, the Inter and Fraunces fonts and ready themes.
requires: [js-core]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned (keep the quotes
around it in commands: a drive's path can hold spaces). Work in the user's
working folder, never inside `<folder>`.

1. Copy the template:

   ```sh
   cp "<folder>"/templates/app.html app.html
   ```

2. Pick libraries for what the app needs: `skill_search` with `kind`
   library, for example "chart", "spreadsheet", "markdown", "search",
   "pdf", "icons". Each quickstart gives the exact import line. Use only
   libraries it finds: they are on this drive.
3. Edit the marked regions of `app.html`:
   - `EDIT: page`: the app's markup inside `<main>`. Alpine attributes
     (`x-model`, `x-for`, `@click`, `x-text`) use the app's data.
   - `EDIT: app`: the data and methods. `localStorage` keeps them in the
     browser between visits.
   - `EDIT: theme`: colours and fonts; `"<folder>"/references/themes.md`
     has ten themes to paste.
   Add imports at the top of the module script. Put data files (CSV, JSON)
   beside the page and import them; never fetch them.
4. Pack it into one file:

   ```sh
   svalbard-drive html pack app.html -o app-packed.html
   ```

   It prints `wrote app-packed.html (…, single-file)` or the problems to
   fix.
5. Check your result. It must print `ok`:

   ```sh
   sh "<folder>"/examples/check.sh app-packed.html
   ```

6. Tell the user the packed file's full path. It opens by double-click on
   any computer, without a network.

## Design

- One clear purpose per page; the main action as the first button.
- Pico styles plain elements: `<article>` is a card, `role="group"` joins
  an input and a button, `<details>` folds, `<dialog>` is a modal.
- Headings in Fraunces, text in Inter, as the template sets; keep text
  readable on the accent colour.
- Icons: the Lucide quickstart; charts: the data-dashboard skill.

# marked 18.3.0 reference

This is the project's own documentation, joined at the version on this drive. Here nothing is installed or downloaded: import the package by name in a module script and pack the page with `svalbard-drive html pack`. Code blocks marked `not-for-this-stick` and lines starting "Not for this drive:" show package managers, CDN links or downloads; this drive does not use them.

## README

<a href="https://marked.js.org">
Not for this drive:   <img width="60px" height="60px" src="https://marked.js.org/img/logo-black.svg" align="right" />
</a>

## README: Marked

Not for this drive: [![npm](https://badgen.net/npm/v/marked)](https://www.npmjs.com/package/marked)
[![install size](https://badgen.net/packagephobia/install/marked)](https://packagephobia.now.sh/result?p=marked)
Not for this drive: [![downloads](https://badgen.net/npm/dt/marked)](https://www.npmjs.com/package/marked)
[![github actions](https://github.com/markedjs/marked/workflows/Tests/badge.svg)](https://github.com/markedjs/marked/actions)
Not for this drive: [![snyk](https://snyk.io/test/npm/marked/badge.svg)](https://snyk.io/test/npm/marked)
[![inspect.software](https://raw.githubusercontent.com/inspect-software/badges/main/v1/m/markedjs/marked.svg)](https://inspect.software/software/markedjs/marked)

- ⚡ built for speed
- ⬇️ low-level compiler for parsing markdown without caching or blocking for long periods of time
- ⚖️ light-weight while implementing all markdown features from the supported flavors & specifications
- 🛠️ easily adaptable with [custom extensions](https://marked.js.org/using_advanced#extensions)
- 🌐 works in a browser, on a server, or from a command line interface (CLI)

## README: Demo

Check out the [demo page](https://marked.js.org/demo/) to see Marked in action ⛹️

## README: Docs

Our [documentation pages](https://marked.js.org) are also rendered using marked 💯

Also read about:

* [Options](https://marked.js.org/using_advanced)
* [Extensibility](https://marked.js.org/using_pro)

## README: Compatibility

**Node.js:** Only [current and LTS](https://nodejs.org/en/about/releases/) Node.js versions are supported. End of life Node.js versions may become incompatible with Marked at any point in time.

**Browser:** [Baseline Widely Available](https://developer.mozilla.org/en-US/docs/Glossary/Baseline/Compatibility)

## README: Installation

**CLI:**

```not-for-this-stick
npm install -g marked
```

**In-browser:**

```not-for-this-stick
npm install marked
```

## README: Usage

### Warning: 🚨 Marked does not [sanitize](https://marked.js.org/using_advanced#options) the output HTML. Please use a sanitize library, like [DOMPurify](https://github.com/cure53/DOMPurify) (recommended), [sanitize-html](https://github.com/apostrophecms/sanitize-html) or [insane](https://github.com/bevacqua/insane) on the *output* HTML! 🚨

```
DOMPurify.sanitize(marked.parse(`<img src="x" onerror="alert('not happening')">`));
```

**CLI**

``` bash
# Example with stdin input
$ marked -o hello.html
hello world
^D
$ cat hello.html
<p>hello world</p>
```

```bash
# Print all options
$ marked --help
```

**Node.js**

```js
import { marked } from 'marked';
const html = marked.parse('# Marked in Node.js');
console.log(html);
```

**Browser**

```html
<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <title>Marked in the browser</title>
</head>
<body>
  <div id="content"></div>
  
  
</body>
</html>
```
or import esm module

```html

```

## README: License

Copyright (c) 2018+, MarkedJS. (MIT License)
Copyright (c) 2011-2018, Christopher Jeffrey. (MIT License)


## INDEX

Marked is

1. built for speed.<sup>*</sup>
2. a low-level markdown compiler for parsing markdown without caching or blocking for long periods of time.<sup>**</sup>
3. light-weight while implementing all markdown features from the supported flavors & specifications.<sup>***</sup>
4. easily adaptable with [custom extensions](/using_advanced#extensions).
5. available as a command line interface (CLI) and running in client- or server-side JavaScript projects.

<p><small><sup>*</sup> Still working on metrics for comparative analysis and definition.</small><br>
<small><sup>**</sup> Zero dependencies.</small><br>
<small><sup>***</sup> Strict compliance could result in slower processing when running comparative benchmarking.</small></p>


<h2 id="demo">Demo</h2>

Checkout the [demo page](./demo/) to see marked in action ⛹️

These documentation pages are also rendered using marked 💯


<h2 id="installation">Installation</h2>

Not for this drive: **CLI:** `npm install -g marked`

**In-browser:**
```not-for-this-stick
npm install marked
```
<h2 id="usage">Usage</h2>

### Warning: 🚨 Marked does not [sanitize](/using_advanced#options) the output HTML. If you are processing potentially unsafe strings, it's important to filter for possible XSS attacks. Some filtering options include [DOMPurify](https://github.com/cure53/DOMPurify) (recommended), [js-xss](https://github.com/leizongmin/js-xss), [sanitize-html](https://github.com/apostrophecms/sanitize-html) and [insane](https://github.com/bevacqua/insane) on the *output* HTML! 🚨

```
DOMPurify.sanitize(marked.parse(`<img src="x" onerror="alert('not happening')">`));
```

**⚠️ Input: special ZERO WIDTH unicode characters (for example `\uFEFF`) might interfere with parsing. Some text editors add them at the start of the file (see: [#2139](https://github.com/markedjs/marked/issues/2139)).**

```js
// remove the most common zerowidth characters from the start of the file
marked.parse(
  contents.replace(/^[\u200B\u200C\u200D\u200E\u200F\uFEFF]/,"")
)
```

**CLI**

``` bash
# Example with stdin input
$ marked -o hello.html
hello world
^D
$ cat hello.html
<p>hello world</p>
```

``` bash
# Example with string input
$ marked -s "*hello world*"
<p><em>hello world</em></p>
```

```bash
# Example with file input

echo "**bold text example**" > readme.md

$ marked -i readme.md -o readme.html
$ cat readme.html
<p><strong>bold text example</strong></p>
```

```bash
# Print all options
$ marked --help
```

*CLI Config*

A config file can be used to configure the marked cli.

If it is a `.json` file it should be a JSON object that will be passed to marked as options.

If `.js` is used it should have a default export of a marked options object or a function that takes `marked` as a parameter.
It can use the `marked` parameter to install extensions using `marked.use`.
The CLI waits for asynchronous config functions to finish before parsing the input.

By default the marked cli will look for a config file in your home directory in the following order.

- `~/.marked.json`
- `~/.marked.js`
- `~/.marked/index.js`

```bash
# Example with custom config

echo '{ "breaks": true }' > config.json

$ marked -s 'line1\nline2' -c config.json
<p>line1<br>line2</p>
```

**Browser**

```html
<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <title>Marked in the browser</title>
</head>
<body>
  <div id="content"></div>
  
  
</body>
</html>
```
or import esm module

```html

```

**Node.js**

```js
import { marked } from 'marked';
// or const { marked } = require('marked');

const html = marked.parse('# Marked in Node.js\n\nRendered by **marked**.');
```


Marked offers [advanced configurations](/using_advanced) and [extensibility](/using_pro) as well.

<h2 id="specifications">Supported Markdown specifications</h2>

We actively support the features of the following [Markdown flavors](https://github.com/commonmark/CommonMark/wiki/Markdown-Flavors).



By supporting the above Markdown flavors, it's possible that Marked can help you use other flavors as well; however, these are not actively supported by the community.

<h2 id="tools">List of Tools Using Marked</h2>

We actively support the usability of Marked in super-fast markdown transformation, some of Tools using `Marked` for single-page creations are

| Tools                                                               |                  Description                                               |
| :-----------------------------------------------------------------  | :------------------------------------------------------------------------  |
| [zero-md](https://zerodevx.github.io/zero-md/)                      | A native markdown-to-html web component to load and display an external MD file.It uses Marked for super-fast markdown transformation. |
| [texme](https://github.com/susam/texme)                             | TeXMe is a lightweight JavaScript utility to create self-rendering Markdown + LaTeX documents.             |
| [StrapDown.js](https://naereen.github.io/StrapDown.js/)             | StrapDown.js is an awesome on-the-fly Markdown to HTML text processor.                |
| [raito](https://raito.arnaud.at/)             | Mini Markdown Wiki/CMS in 8kb of JavaScript.                |
| [Homebrewery](https://homebrewery.naturalcrit.com/)             | The Homebrewery is a tool for making authentic looking D&D content using Markdown. It is distributed under the terms of the MIT.             |
| [marked_reader](https://github.com/CNOCTAVE/marked_reader)          | marked_reader is an open source Markdown reader packed by Electron. |
| [AtlasDocs](https://atlasdocs.io/en/markdown-to-html)             | Free document tools that run entirely in the browser; uses Marked to turn Markdown into HTML, PDF and DOCX without uploading files. |

<h2 id="security">Security</h2>

The only completely secure system is the one that doesn't exist in the first place. Having said that, we take the security of Marked very seriously.

Therefore, please disclose potential security issues by email to the project [committers](/authors) as well as the [listed owners within NPM](https://docs.npmjs.com/cli/owner). We will provide an initial assessment of security reports within 48 hours and should apply patches within 2 weeks (also, feel free to contribute a fix for the issue).


## USING_ADVANCED

<h2 id="instance">Marked instance</h2>

By default, Marked stores options and extensions in the global scope. That means changing the options in one script will also change the options in another script since they share the same instance.

If you don't want to mutate global scope, you can create a new instance of Marked to ensure options and extensions are locally scoped.

```js
import { Marked } from 'marked';
const marked = new Marked([options, extension, ...]);
```

|Argument |Type    |Notes                                                                  |
|:--------|:-------|:----------------------------------------------------------------------|
| options |`object`|The same arguments that can be passed to [`marked.use`](/using_pro#use)|

Be careful: marked.use(...) should not be used in a loop or function. It should only be used directly after new Marked is created or marked is imported.

## USING_ADVANCED: The `parse` function

```js
import { marked } from 'marked';
marked.parse(markdownString [,options])
```

|Argument                      |Type    |Notes                                                           |
|:-----------------------------|:-------|:---------------------------------------------------------------|
|markdownString                |`string`|String of markdown source to be compiled.                       |
|<a href="#options">options</a>|`object`|Hash of options. Can also use `marked.use` to set global options|

### Alternative using reference

```js
// Create reference instance
import { marked } from 'marked';

// Set options
marked.use({
  async: true,
  pedantic: false,
  gfm: true,
});

// Compile
console.log(marked.parse(markdownString));
```

<h2 id="options">Options</h2>

|Member      |Type      |Default  |Since    |Notes         |
|:-----------|:---------|:--------|:--------|:-------------|
|async       |`boolean` |`false`  |4.1.0    |If true, `walkTokens` functions can be async and `marked.parse` will return a promise that resolves when all walk tokens functions resolve.|
|breaks      |`boolean` |`false`  |v0.2.7   |If true, add `<br>` on a single line break (copies GitHub behavior on comments, but not on rendered markdown files). Requires `gfm` be `true`.|
|gfm         |`boolean` |`true`   |v0.2.1   |If true, use approved [GitHub Flavored Markdown (GFM) specification](https://github.github.com/gfm/).|
|pedantic    |`boolean` |`false`  |v0.2.1   |If true, conform to the original `markdown.pl` as much as possible. Don't fix original markdown bugs or behavior. Turns off and overrides `gfm`.|
|renderer    |`object`  |`new Renderer()`|v0.3.0|An object containing functions to render tokens to HTML. See [extensibility](/using_pro) for more details.|
|silent      |`boolean` |`false`  |v0.2.7   |If true, errors caught by `marked.parse` / `marked.parseInline` are returned as an HTML error message instead of being thrown. Problems the lexer or parser recover from (an unknown token type, the infinite-loop guard) are logged with `console.error` instead of thrown.|
|tokenizer    |`object`  |`new Tokenizer()`|v1.0.0|An object containing functions to create tokens from markdown. See [extensibility](/using_pro) for more details.|
|walkTokens   |`function`  |`null`|v1.1.0|A function which is called for every token. See [extensibility](/using_pro) for more details.|

## USING_ADVANCED: Old Options

|Member      |Type      |Default  |Since    |Notes         |
|:-----------|:---------|:--------|:--------|:-------------|
|smartLists (**removed**)| `boolean` | `false` |v0.2.8 | Removed in v3.0.0. |
|baseUrl (**removed**)|`string`  |`null`   |v0.3.9    |Removed in v8.0.0 use [`marked-base-url`](https://www.npmjs.com/package/marked-base-url) to prefix url for any relative link. |
|headerIds (**removed**)|`boolean` |`true`   |v0.4.0   |Removed in v8.0.0 use [`marked-gfm-heading-id`](https://www.npmjs.com/package/marked-gfm-heading-id) to include an `id` attribute when emitting headings (h1, h2, h3, etc).|
|headerPrefix (**removed**)|`string`  |`''`     |v0.3.0   |Removed in v8.0.0 use [`marked-gfm-heading-id`](https://www.npmjs.com/package/marked-gfm-heading-id) to add a string to prefix the `id` attribute when emitting headings (h1, h2, h3, etc).|
|highlight (**removed**)|`function`|`null`   |v0.3.0   |Removed in v8.0.0 use [`marked-highlight`](https://www.npmjs.com/package/marked-highlight) to add highlighting to code blocks. |
|langPrefix  (**removed**)|`string`  |`'language-'`|v0.3.0|Removed in v8.0.0 use [`marked-highlight`](https://www.npmjs.com/package/marked-highlight) to prefix the className in a `<code>` block. Useful for syntax highlighting.|
|mangle (**removed**)|`boolean` |`true`   |v0.3.4   |Removed in v8.0.0 use [`marked-mangle`](https://www.npmjs.com/package/marked-mangle) to mangle email addresses.|
|sanitize (**removed**)|`boolean` |`false`  |v0.2.1   |Removed in v8.0.0 use a sanitize library, like [DOMPurify](https://github.com/cure53/DOMPurify) (recommended), [sanitize-html](https://github.com/apostrophecms/sanitize-html) or [insane](https://github.com/bevacqua/insane) on the output HTML! |
|sanitizer  (**removed**)|`function`|`null`   |v0.3.4   |Removed in v8.0.0 use a sanitize library, like [DOMPurify](https://github.com/cure53/DOMPurify) (recommended), [sanitize-html](https://github.com/apostrophecms/sanitize-html) or [insane](https://github.com/bevacqua/insane) on the output HTML!|
|smartypants (**removed**)|`boolean` |`false`  |v0.2.9   |Removed in v8.0.0 use [`marked-smartypants`](https://www.npmjs.com/package/marked-smartypants) to use "smart" typographic punctuation for things like quotes and dashes.|
|xhtml (**removed**)|`boolean` |`false`  |v0.3.2   |Removed in v8.0.0 use [`marked-xhtml`](https://www.npmjs.com/package/marked-xhtml) to emit self-closing HTML tags for void elements (&lt;br/&gt;, &lt;img/&gt;, etc.) with a "/" as required by XHTML.|

<h2 id="extensions">Known Extensions</h2>

Marked can be extended using [custom extensions](/using_pro#extensions). This is a list of extensions that can be used with `marked.use(extension)`.



<div id="extension-search-container" class="relative my-6 max-w-lg">
  <span class="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-subtle-light dark:text-subtle-dark">
    <span class="material-icons text-xl">search</span>
  </span>
  <input
    type="text"
    id="extension-search"
    placeholder="Search extensions by name, package, or description..."
    class="w-full pl-10 pr-10 py-2.5 bg-gray-50 dark:bg-gray-800/50 border border-border-light dark:border-border-dark rounded-xl text-text-light dark:text-text-dark placeholder-subtle-light dark:placeholder-subtle-dark focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all duration-200 shadow-sm"
  />
  <button
    id="extension-search-clear"
    type="button"
    class="absolute inset-y-0 right-0 flex items-center pr-3 text-subtle-light dark:text-subtle-dark hover:text-text-light dark:hover:text-text-dark hidden"
    aria-label="Clear search"
  >
    <span class="material-icons text-lg">close</span>
  </button>
</div>

<div id="no-extensions-msg" class="hidden p-8 text-center bg-gray-50 dark:bg-gray-800/30 border border-dashed border-border-light dark:border-border-dark rounded-xl my-6">
  <span class="material-icons text-4xl text-subtle-light dark:text-subtle-dark mb-2">search_off</span>
  <p class="text-text-light dark:text-text-dark font-medium">No extensions found</p>
  <p class="text-sm text-subtle-light dark:text-subtle-dark mt-1">Try adjusting your keywords or search term.</p>
</div>



|Name|Package Name|Description|
|:---|:-----------|:----------|
|[ABCjs](https://www.npmjs.com/package/marked-abc)|[`marked-abc`](https://www.npmjs.com/package/marked-abc)|[ABCjs](https://www.abcjs.net/) sheet music rendering|
|[Admonition](https://www.npmjs.com/package/marked-admonition-extension)|[`marked-admonition-extension`](https://www.npmjs.com/package/marked-admonition-extension)| Admonition extension |
|[Alert](https://github.com/bent10/marked-extensions/tree/main/packages/alert)|[`marked-alert`](https://www.npmjs.com/package/marked-alert)|Enables [GFM alerts](https://github.com/orgs/community/discussions/16925)|
|[Base URL](https://github.com/markedjs/marked-base-url)|[`marked-base-url`](https://www.npmjs.com/package/marked-base-url)|Prefix relative urls with a base URL|
|[Bidi](https://github.com/markedjs/marked-bidi)|[`marked-bidi`](https://www.npmjs.com/package/marked-bidi)|Add Bidirectional text support to the HTML|
|[CJK Breaks](https://github.com/chirsz-ever/marked-cjk-breaks)|[`marked-cjk-breaks`](https://www.npmjs.com/package/marked-cjk-breaks)|Suppress soft linebreaks between east asian characters|
|[Code Format](https://github.com/bent10/marked-extensions/tree/main/packages/code-format)|[`marked-code-format`](https://www.npmjs.com/package/marked-code-format)|Formatting code blocks using Prettier|
|[Code JSX Renderer](https://github.com/bent10/marked-extensions/tree/main/packages/code-jsx-renderer)|[`marked-code-jsx-renderer`](https://www.npmjs.com/package/marked-code-jsx-renderer)|Render JSX code blocks using a custom renderer and components|
|[Code Preview](https://github.com/bent10/marked-extensions/tree/main/packages/code-preview)|[`marked-code-preview`](https://www.npmjs.com/package/marked-code-preview)|Transform code blocks into code previews|
|[Custom Heading ID](https://github.com/markedjs/marked-custom-heading-id)|[`marked-custom-heading-id`](https://www.npmjs.com/package/marked-custom-heading-id)|Specify a custom heading id in headings with the [Markdown Extended Syntax](https://www.markdownguide.org/extended-syntax/#heading-ids) `# heading {#custom-id}`|
|[Directive](https://github.com/bent10/marked-extensions/tree/main/packages/directive)|[`marked-directive`](https://www.npmjs.com/package/marked-directive)|Supports [directives syntax](https://talk.commonmark.org/t/generic-directives-plugins-syntax/444)|
|[Emoji](https://github.com/UziTech/marked-emoji)|[`marked-emoji`](https://www.npmjs.com/package/marked-emoji)|Add emoji support like on GitHub|
|[Extended Tables](https://github.com/calculuschild/marked-extended-tables)|[`marked-extended-tables`](https://www.npmjs.com/package/marked-extended-tables)|Extends the standard Github-Flavored tables to support advanced features: Column Spanning, Row Spanning, Multi-row headers|
|[Footnote](https://github.com/bent10/marked-extensions/tree/main/packages/footnote)|[`marked-footnote`](https://www.npmjs.com/package/marked-footnote)|Enables [GFM footnotes](https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax#footnotes)|
|[GFM Heading ID](https://github.com/markedjs/marked-gfm-heading-id)|[`marked-gfm-heading-id`](https://www.npmjs.com/package/marked-gfm-heading-id)|Use [`github-slugger`](https://github.com/Flet/github-slugger) to create the heading IDs and allow a custom prefix|
|[GitLab](https://github.com/UziTech/marked-gitlab)|[`marked-gitlab`](https://www.npmjs.com/package/marked-gitlab)|Extension for [GitLab Flavored Markdown (GLFM)](https://docs.gitlab.com/user/markdown/)|
|[Highlight](https://github.com/markedjs/marked-highlight)|[`marked-highlight`](https://www.npmjs.com/package/marked-highlight)|Highlight code blocks|
|[HTML Renderer](https://github.com/UziTech/marked-html-renderer)|[`marked-html-renderer`](https://www.npmjs.com/package/marked-html-renderer)|Output HTML Elements instead of a string|
|[Inline Annotation](https://github.com/BaksiLi/markdown-inline-annotation/tree/main/packages/marked)|[`marked-inline-annotation`](https://www.npmjs.com/package/marked-inline-annotation)|Add ruby (furigana), interlinear glosses, and emphasis dots to Markdown text|
|[Jira Renderer](https://github.com/mrmarble/marked-jira)|[`marked-jira`](https://www.npmjs.com/package/marked-jira)|Output Jira compatible format|
|[Katex Code](https://github.com/UziTech/marked-katex-extension)|[`marked-katex-extension`](https://www.npmjs.com/package/marked-katex-extension)|Render [katex](https://katex.org/) code|
|[LinkifyIt](https://github.com/UziTech/marked-linkify-it)|[`marked-linkify-it`](https://www.npmjs.com/package/marked-linkify-it)|Use [linkify-it](https://github.com/markdown-it/linkify-it) for urls|
|[Mangle](https://github.com/markedjs/marked-mangle)|[`marked-mangle`](https://www.npmjs.com/package/marked-mangle)|Mangle mailto links with HTML character references|
|[Math](https://github.com/webc-site/math/tree/main/plugin/marked)|[`@webc.site/math-marked`](https://www.npmjs.com/package/@webc.site/math-marked)|Ultra-lightweight native MathML math rendering extension|
|[Misskey-flavored Markdown](https://akkoma.dev/sfr/marked-mfm)|[`marked-mfm`](https://www.npmjs.com/package/marked-mfm)|Custom extension for [Misskey-flavored Markdown](https://github.com/misskey-dev/mfm.js/blob/develop/docs/syntax.md)|
|[More Lists](https://github.com/jasny/marked-more-lists)|[`marked-more-lists`](https://www.npmjs.com/package/marked-more-lists)|Support for alphabetical and roman numeral ordered lists|
|[Pele](https://github.com/yue1123/pele-markdown/tree/main/packages/marked-pele)|[`marked-pele`](https://www.npmjs.com/package/marked-pele)|Render [Mermaid](https://mermaid.js.org) diagrams to inline SVG at build time with [pele](https://pele.run), no client-side JS|
|[Plaintify](https://github.com/bent10/marked-extensions/tree/main/packages/plaintify)|[`marked-plaintify`](https://www.npmjs.com/package/marked-plaintify)|Converts Markdown to Plaintext|
|[Responsive Images](https://github.com/ELowry/MarkedResponsiveImages)|[`marked-responsive-images`](https://www.npmjs.com/package/marked-responsive-images)|Generate responsive (`srcset`) images based on structured filenames|
|[Shiki](https://github.com/bent10/marked-extensions/tree/main/packages/shiki)|[`marked-shiki`](https://www.npmjs.com/package/marked-shiki)|Integrating [Shiki](https://shiki.style/) syntax highlighting|
|[Sequential Hooks](https://github.com/bent10/marked-extensions/tree/main/packages/sequential-hooks)|[`marked-sequential-hooks`](https://www.npmjs.com/package/marked-sequential-hooks)|Enables the sequential preprocessing and post-processing within [sequential hooks](https://github.com/bent10/marked-extensions#sequential-hooks)|
|[Smartypants](https://github.com/markedjs/marked-smartypants)|[`marked-smartypants`](https://www.npmjs.com/package/marked-smartypants)|Use [smartypants](https://www.npmjs.com/package/smartypants) to use "smart" typographic punctuation for things like quotes and dashes|
|[Smartypants lite](https://github.com/calculuschild/marked-smartypants-lite)|[`marked-smartypants-lite`](https://www.npmjs.com/package/marked-smartypants-lite)|A faster lighter version of marked-smartypants that doesn't use any external dependencies to create "smart" typographic punctuation for things like quotes and dashes|
|[Token Position](https://github.com/UziTech/marked-token-position)|[`marked-token-position`](https://www.npmjs.com/package/marked-token-position)|Adds a `position` field to each token with `start` and `end` properties containing `line`, `column`, and `offset`|
|[Typograf](https://github.com/laidrivm/marked-typograf)|[`marked-typograf`](https://www.npmjs.com/package/marked-typograf)|Use [typograf](https://www.npmjs.com/package/typograf) as a more powerful and extendable alternative to Smartypants for creating “smart” typographic punctuation, such as quotes and dashes|
|[XHTML](https://github.com/markedjs/marked-xhtml)|[`marked-xhtml`](https://www.npmjs.com/package/marked-xhtml)|Emit self-closing HTML tags for void elements (&lt;br/&gt;, &lt;img/&gt;, etc.) with a "/" as required by XHTML|

<h2 id="user-examples">User Examples</h2>

<details>

<summary>Marked can render on-page content as markdown in the browser.</summary>

```html
<!DOCTYPE html>
<html>
 <head>
   
   <link rel="stylesheet" 
    href="https://cdnjs.cloudflare.com/ajax/libs/github-markdown-css/5.8.1/github-markdown.min.css"
    crossorigin="anonymous" referrerpolicy="no-referrer" />

   <title>Marked for the full page</title>
 </head>

 <body> 
  <textarea id="markdown-source" style="display: none;">
# Title

Lots of text using **markdown syntax.**
  </textarea>
  <div id="content" class="markdown-body"></div>
  
  
  
 </body>
</html>
```
</details>

<details id="numeric-character-references">

<summary>Preserve numeric character references in inline text tokens.</summary>

Inline text tokens decode numeric character references according to CommonMark.
For example, `&#27;` and `&#x1b;` in ordinary text become the U+001B escape
character in `token.text`. The token's `raw` property contains the Markdown it
consumed; `text` is not a lossless copy of that source.

A non-HTML renderer that needs literal numeric references in inline text can
intercept them with a [custom inline extension](/using_pro#extensions):

```js
import { Marked } from 'marked';

const marked = new Marked({
  extensions: [{
    name: 'numericEntity',
    level: 'inline',
    start(src) {
      return src.indexOf('&#');
    },
    tokenizer(src) {
      const match = /^&#(?:\d{1,7}|[xX][a-fA-F0-9]{1,6});/.exec(src);
      if (match) {
        return {
          type: 'numericEntity',
          raw: match[0],
          text: match[0]
        };
      }
    },
    renderer(token) {
      return token.text;
    }
  }]
});

const tokens = marked.lexer('before &#27;[2J and &#x1b;[2J after');
console.log(tokens[0].tokens
  .filter(token => token.type === 'numericEntity')
  .map(token => token.text));
// ['&#27;', '&#x1b;']
```

The extension's `start` hint lets the lexer find references after ordinary text,
not just at the beginning of the input. Your non-HTML renderer can handle the
`numericEntity` tokens without decoding their `text`.

This example changes inline text tokenization, not link destinations, code spans,
or raw HTML. It is not a terminal sanitizer: renderers must still decide how to
handle untrusted content, including literal control characters. When used with
the HTML parser, the example renderer emits the original reference spelling,
which a browser can still interpret as a character reference.

</details>

<h2 id="inline">Inline Markdown</h2>

You can parse inline markdown by running markdown through `marked.parseInline`.

```js
const blockHtml = marked.parse('**strong** _em_');
console.log(blockHtml); // '<p><strong>strong</strong> <em>em</em></p>'

const inlineHtml = marked.parseInline('**strong** _em_');
console.log(inlineHtml); // '<strong>strong</strong> <em>em</em>'
```

<h2 id="highlight">Highlighting</h2>

Use [`marked-highlight`](https://www.npmjs.com/package/marked-highlight) to highlight code blocks.

<h2 id="workers">Workers</h2>

To prevent ReDoS attacks you can run marked on a worker and terminate it when parsing takes longer than usual.

Marked can be run in a [worker thread](https://nodejs.org/api/worker_threads.html) on a node server, or a [web worker](https://developer.mozilla.org/en-US/docs/Web/API/Web_Workers_API) in a browser.

### Node Worker Thread

```js
// markedWorker.js

import { marked } from 'marked';
import { parentPort } from 'worker_threads';

parentPort.on('message', (markdownString) => {
  parentPort.postMessage(marked.parse(markdownString));
});
```

```js
// index.js

import { Worker } from 'worker_threads';
const markedWorker = new Worker('./markedWorker.js');

const markedTimeout = setTimeout(() => {
  markedWorker.terminate();
  throw new Error('Marked took too long!');
}, timeoutLimit);

markedWorker.on('message', (html) => {
  clearTimeout(markedTimeout);
  console.log(html);
  markedWorker.terminate();
});

markedWorker.postMessage(markdownString);
```

### Web Worker

> **NOTE**: Web Workers send the payload from `postMessage` in an object with the payload in a `.data` property

```js
// markedWorker.js

importScripts('path/to/marked.umd.js');

onmessage = (e) => {
  const markdownString = e.data
  postMessage(marked.parse(markdownString));
};
```

```js
// script.js

const markedWorker = new Worker('./markedWorker.js');

const markedTimeout = setTimeout(() => {
  markedWorker.terminate();
  throw new Error('Marked took too long!');
}, timeoutLimit);

markedWorker.onmessage = (e) => {
  clearTimeout(markedTimeout);
  const html = e.data;
  console.log(html);
  markedWorker.terminate();
};

markedWorker.postMessage(markdownString);
```

<h2 id="cli-extensions">CLI Extensions</h2>

You can use extensions in the CLI by creating a new CLI that imports marked and the marked binary.

```js
// file: myMarked
#!/usr/bin/node

import { marked } from 'marked';
import customHeadingId from 'marked-custom-heading-id';

marked.use(customHeadingId());

import 'marked/bin/marked';
```

```sh
$ ./myMarked -s "# heading {#custom-id}"
<h1 id="custom-id">heading</h1>
```


## USING_PRO

## USING_PRO: Extending Marked

To champion the single-responsibility and open/closed principles, we have tried to make it relatively painless to extend Marked. If you are looking to add custom functionality, this is the place to start.

<h2 id="terminology">Terminology Note</h2>

Before diving in, it's important to understand the terminology used in this documentation:

| Term | TypeScript Type | Description |
|------|----------------|-------------|
| **Marked Extension** | `MarkedExtension` | The configuration object passed to `marked.use()`. Can contain options, hooks, renderer overrides, tokenizer overrides, and custom extensions. |
| **Custom Extension** | `TokenizerAndRendererExtension` | Objects within the `extensions` array that define custom tokenizers and renderers for new syntax. |

In other words, a **Marked Extension** is the top-level plugin configuration, while **Custom Extensions** are the individual tokenizer/renderer definitions for custom syntax within that configuration.

```js
// Marked Extension (MarkedExtension)
marked.use({
  gfm: true,
  breaks: false,
  renderer: { /* renderer overrides */ },
  tokenizer: { /* tokenizer overrides */ },
  // Custom Extensions (TokenizerAndRendererExtension[])
  extensions: [
    { name: 'myCustomSyntax', level: 'block', tokenizer: fn, renderer: fn }
  ]
});
```

<h2 id="use">marked.use()</h2>

`marked.use(extension)` is the recommended way to extend Marked. The `extension` object can contain any [option](/using_advanced#options) available in Marked:


```js
import { marked } from 'marked';

marked.use({
  pedantic: false,
  gfm: true,
  breaks: false
});
```

You can also supply multiple `extension` objects at once.

```js
marked.use(myExtension, extension2, extension3);

\\ EQUIVALENT TO:

marked.use(myExtension);
marked.use(extension2);
marked.use(extension3);
```

All options will overwrite those previously set, except for the following options which will be merged with the existing framework and can be used to change or extend the functionality of Marked: `renderer`, `tokenizer`, `hooks`, `walkTokens`, and `extensions`.

* The `renderer`, `tokenizer`, and `hooks` options are objects with functions that will be merged into the built-in `renderer` and `tokenizer` respectively.

* The `walkTokens` option is a function that will be called to post-process every token before rendering.

* The `extensions` option is an array of objects that can contain additional custom `renderer` and `tokenizer` steps that will execute before any of the default parsing logic occurs.

Importantly, ensure that the extensions are only added to `marked` once (ie in the global scope of a regular JavaScript or TypeScript module). If they are added in a function that is called repeatedly, or in the JS for an HTML component in a library such as Svelte, your extensions will be added repeatedly, eventually causing a recursion error. If you cannot prevent the code from being run repeatedly, you should create a [Marked instance](/using_advanced#instance) so that your extensions are stored independently from the global instance Marked provides.

***

<h2>The Marked Pipeline</h2>

Before building your custom extensions, it is important to understand the components that Marked uses to translate from Markdown to HTML:

1) The user supplies Marked with an input string to be translated.
2) The `lexer` feeds segments of the input text string into each `tokenizer`, and from their output, generates a series of tokens in a nested tree structure.
3) Each `tokenizer` receives a segment of Markdown text and, if it matches a particular pattern, generates a token object containing any relevant information.
4) The `walkTokens` function will traverse every token in the tree and perform any final adjustments to the token contents.
4) The `parser` traverses the token tree and feeds each token into the appropriate `renderer`, and concatenates their outputs into the final HTML result.
5) Each `renderer` receives a token and manipulates its contents to generate a segment of HTML.

Marked provides methods for directly overriding the `renderer` and `tokenizer` for any existing token type, as well as inserting additional custom `renderer` and `tokenizer` functions to handle entirely custom syntax. For example, using `marked.use({renderer})` would modify a renderer, whereas `marked.use({extensions: [{renderer}]})` would add a new renderer. See the [custom extensions example](#custom-extensions-example) for insight on how to execute this.

***

<h2 id="renderer">The Renderer : <code>renderer</code></h2>

The renderer defines the HTML output of a given token. If you supply a `renderer` in the options object passed to `marked.use()`, any functions in the object will override the default handling of that token type.

Calling `marked.use()` to override the same function multiple times will give priority to the version that was assigned *last*. Overriding functions can return `false` to fall back to the previous override in the sequence, or resume default behavior if all overrides return `false`. Returning any other value (including nothing) will prevent fallback behavior.

**Example:** Overriding output of the default `heading` token by adding an embedded anchor tag like on GitHub.

```js
// Create reference instance
import { marked } from 'marked';

// Override function
const renderer = {
  heading({ tokens, depth }) {
    const text = this.parser.parseInline(tokens);
    const escapedText = text.toLowerCase().replace(/[^\w]+/g, '-');

    return `
            <h${depth}>
              <a name="${escapedText}" class="anchor" href="#${escapedText}">
                <span class="header-link"></span>
              </a>
              ${text}
            </h${depth}>`;
  }
};

marked.use({ renderer });

// Run marked
console.log(marked.parse('# heading+'));
```

**Output:**

```html
<h1>
  <a name="heading-" class="anchor" href="#heading-">
    <span class="header-link"></span>
  </a>
  heading+
</h1>
```
**Note:** Calling `marked.use()` in the following way will avoid overriding the `heading` token output but create a new `heading` renderer in the process.

```js
marked.use({
 extensions: [{
    name: 'heading',
    renderer(token) {
      return /* ... */
    }
  }]
})
```

### Block-level renderer methods

- <code>**space**(token: *Tokens.Space*): *string*</code>
- <code>**code**(token: *Tokens.Code*): *string*</code>
- <code>**blockquote**(token: *Tokens.Blockquote*): *string*</code>
- <code>**html**(token: *Tokens.HTML | Tokens.Tag*): *string*</code>
- <code>**def**(token: *Tokens.Def*): *string*</code>
- <code>**heading**(token: *Tokens.Heading*): *string*</code>
- <code>**hr**(token: *Tokens.Hr*): *string*</code>
- <code>**list**(token: *Tokens.List*): *string*</code>
- <code>**listitem**(token: *Tokens.ListItem*): *string*</code>
- <code>**checkbox**(token: *Tokens.Checkbox*): *string*</code>
- <code>**paragraph**(token: *Tokens.Paragraph*): *string*</code>
- <code>**table**(token: *Tokens.Table*): *string*</code>
- <code>**tablerow**(token: *Tokens.TableRow*): *string*</code>
- <code>**tablecell**(token: *Tokens.TableCell*): *string*</code>

### Inline-level renderer methods

- <code>**strong**(token: *Tokens.Strong*): *string*</code>
- <code>**em**(token: *Tokens.Em*): *string*</code>
- <code>**codespan**(token: *Tokens.Codespan*): *string*</code>
- <code>**br**(token: *Tokens.Br*): *string*</code>
- <code>**del**(token: *Tokens.Del*): *string*</code>
- <code>**link**(token: *Tokens.Link*): *string*</code>
- <code>**image**(token: *Tokens.Image*): *string*</code>
- <code>**text**(token: *Tokens.Text | Tokens.Escape | Tokens.Tag*): *string*</code>

The Tokens.* properties can be found [here](https://github.com/markedjs/marked/blob/master/src/Tokens.ts).

***

<h2 id="tokenizer">The Tokenizer : <code>tokenizer</code></h2>

The tokenizer defines how to turn markdown text into tokens. If you supply a `tokenizer` object to the Marked options, it will be merged with the built-in tokenizer and any functions inside will override the default handling of that token type.

Calling `marked.use()` to override the same function multiple times will give priority to the version that was assigned *last*. Overriding functions can return `false` to fall back to the previous override in the sequence, or resume default behavior if all overrides return `false`. Returning any other value (including nothing) will prevent fallback behavior.

**Example:** Overriding default `codespan` tokenizer to include LaTeX.

```js
// Create reference instance
import { marked } from 'marked';

// Override function
const tokenizer = {
  codespan(src) {
    const match = src.match(/^\$+([^\$\n]+?)\$+/);
    if (match) {
      return {
        type: 'codespan',
        raw: match[0],
        text: match[1].trim()
      };
    }

    // return false to use original codespan tokenizer
    return false;
  }
};

marked.use({ tokenizer });

// Run marked
console.log(marked.parse('$ latex code $\n\n` other code `'));
```

**Output:**

```html
<p><code>latex code</code></p>
<p><code>other code</code></p>
```
**NOTE**: This does not fully support latex, see issue [#1948](https://github.com/markedjs/marked/issues/1948).

### Block level tokenizer methods

- <code>**space**(src: *string*): *Tokens.Space*</code>
- <code>**code**(src: *string*): *Tokens.Code*</code>
- <code>**fences**(src: *string*): *Tokens.Code*</code>
- <code>**heading**(src: *string*): *Tokens.Heading*</code>
- <code>**hr**(src: *string*): *Tokens.Hr*</code>
- <code>**blockquote**(src: *string*): *Tokens.Blockquote*</code>
- <code>**list**(src: *string*): *Tokens.List*</code>
- <code>**html**(src: *string*): *Tokens.HTML*</code>
- <code>**def**(src: *string*): *Tokens.Def*</code>
- <code>**table**(src: *string*): *Tokens.Table*</code>
- <code>**lheading**(src: *string*): *Tokens.Heading*</code>
- <code>**paragraph**(src: *string*): *Tokens.Paragraph*</code>
- <code>**text**(src: *string*): *Tokens.Text*</code>

### Inline level tokenizer methods

- <code>**escape**(src: *string*): *Tokens.Escape*</code>
- <code>**tag**(src: *string*): *Tokens.Tag*</code>
- <code>**link**(src: *string*): *Tokens.Link | Tokens.Image*</code>
- <code>**reflink**(src: *string*, links: *object*): *Tokens.Link | Tokens.Image | Tokens.Text*</code>
- <code>**emStrong**(src: *string*, maskedSrc: *string*, prevChar: *string*): *Tokens.Em | Tokens.Strong*</code>
- <code>**codespan**(src: *string*): *Tokens.Codespan*</code>
- <code>**br**(src: *string*): *Tokens.Br*</code>
- <code>**del**(src: *string*, maskedSrc: *string*, prevChar: *string*): *Tokens.Del*</code>
- <code>**autolink**(src: *string*): *Tokens.Link*</code>
- <code>**url**(src: *string*): *Tokens.Link*</code>
- <code>**inlineText**(src: *string*): *Tokens.Text*</code>

The Tokens.* properties can be found [here](https://github.com/markedjs/marked/blob/master/src/Tokens.ts).

***

<h2 id="walk-tokens">Walk Tokens : <code>walkTokens</code></h2>

The walkTokens function gets called with every token. Child tokens are called before moving on to sibling tokens. Each token is passed by reference so updates are persisted when passed to the parser. When [`async`](#async) mode is enabled, the return value is awaited. Otherwise the return value is ignored.

`marked.use()` can be called multiple times with different `walkTokens` functions. Each function will be called in order, starting with the function that was assigned *last*.

**Example:** Overriding heading tokens to start at h2.

```js
import { marked } from 'marked';

// Override function
const walkTokens = (token) => {
  if (token.type === 'heading') {
    token.depth += 1;
  }
};

marked.use({ walkTokens });

// Run marked
console.log(marked.parse('# heading 2\n\n## heading 3'));
```

**Output:**

```html
<h2>heading 2</h2>
<h3>heading 3</h3>
```

***

<h2 id="hooks">Hooks : <code>hooks</code></h2>

Hooks are methods that hook into some part of marked. The following hooks are available:

| signature | description |
|-----------|-------------|
| `preprocess(markdown: string): string` | Process markdown before sending it to marked. |
| `postprocess(html: string): string` | Process html after marked has finished parsing. |
| `processAllTokens(tokens: Token[]): Token[]` | Process all tokens before walk tokens. |
| `emStrongMask(src: string): string` | Mask part of the content that should not be interpreted as Markdown em/strong delimiters. |
| `provideLexer(): (src: string, options?: MarkedOptions) => Token[]` | Provide function to tokenize markdown. |
| `provideParser(): (tokens: Token[], options?: MarkedOptions) => string` | Provide function to parse tokens. |

`marked.use()` can be called multiple times with different `hooks` functions. Each function will be called in order, starting with the function that was assigned *last*.

**Example:** Set options based on [front-matter](https://www.npmjs.com/package/front-matter)

```js
import { marked } from 'marked';
import fm from 'front-matter';

// Override function
function preprocess(markdown) {
  const { attributes, body } = fm(markdown);
  for (const prop in attributes) {
    if (prop in this.options) {
    this.options[prop] = attributes[prop];
    }
  }
  return body;
}

marked.use({ hooks: { preprocess } });

// Run marked
console.log(marked.parse(`
---
breaks: true
---

line1
line2
`.trim()));
```

**Output:**

```html
<p>line1<br>line2</p>
```

**Example:** Sanitize HTML with [isomorphic-dompurify](https://www.npmjs.com/package/isomorphic-dompurify)

```js
import { marked } from 'marked';
import DOMPurify from 'isomorphic-dompurify';

// Override function
function postprocess(html) {
  return DOMPurify.sanitize(html);
}

marked.use({ hooks: { postprocess } });

// Run marked
console.log(marked.parse(`
<img src=x onerror=alert(1)//>
`));
```

**Output:**

```html
<img src="x">
```

**Example:** Save reflinks for chunked rendering

```js
import { marked, Lexer } from 'marked';

let refLinks = {};

// Override function
function processAllTokens(tokens) {
  refLinks = tokens.links;
  return tokens;
}

function provideLexer(src, options) {
  return (src, options) => {
    const lexer = new Lexer(options);
    lexer.tokens.links = refLinks;
    return this.block ? lexer.lex(src) : lexer.inlineTokens(src);
  };
}

marked.use({ hooks: { processAllTokens, provideLexer } });

// Parse reflinks separately from markdown that uses them
marked.parse(`
[test]: http://example.com
`);

console.log(marked.parse(`
[test link][test]
`));
```

**Output:**

```html
<p><a href="http://example.com">test link</a></p>
```

**Example:** Mask underline characters inside Mathjax content delimited by `$`

```js
import { marked } from 'marked';

// Override function
function emStrongMask(src) {
  return src.replace(/\$([^$]+)\$/g, (match) => `[${'a'.repeat(match.length - 2)}]`);
}

marked.use({ hooks: { emStrongMask } });

console.log(marked.parse(`_The formula is $a_ b=c_ d$._`));
```

**Output:**

```html
<p><em>The formula is $a_ b=c_ d$.</em></p>
```

***

<h2 id="extensions">Custom Extensions : <code>extensions</code></h2>

You may supply an `extensions` array to the `options` object. This array can contain any number of `extension` objects, using the following properties:

<dl>
<dt><code><strong>name</strong></code></dt>
<dd>A string used to identify the token that will be handled by this extension.

If the name matches an existing extension name, or an existing method in the tokenizer/renderer methods listed above, they will override the previously assigned behavior, with priority on the extension that was assigned **last**. An extension can return `false` to fall back to the previous behavior.</dd>

<dt><code><strong>level</strong></code></dt>
<dd>A string to determine when to run the extension tokenizer. Must be equal to 'block' or 'inline'.

A **block-level** extension will be handled before any of the block-level tokenizer methods listed above, and generally consists of 'container-type' text (paragraphs, tables, blockquotes, etc.).

An **inline-level** extension will be handled inside each block-level token, before any of the inline-level tokenizer methods listed above. These generally consist of 'style-type' text (italics, bold, etc.).</dd>

<dt><code><strong>start</strong>(<i>string</i> src)</code></dt>
<dd>A function that returns the index of the next potential start of the custom token.

The index can be the result of a <code>src.match().index</code>, or even a simple <code>src.indexOf()</code>. Marked will use this function to ensure that it does not skip over any text that should be part of the custom token.</dd>

<dt><code><strong>tokenizer</strong>(<i>string</i> src, <i>array</i> tokens)</code></dt>
<dd>A function that reads string of Markdown text and returns a generated token. The token pattern should be found at the beginning of the <code>src</code> string. Accordingly, if using a Regular Expression to detect a token, it should be anchored to the string start (`^`). The <code>tokens</code> parameter contains the array of tokens that have been generated by the lexer up to that point, and can be used to access the previous token, for instance.

The return value should be an object with the following parameters:

<dl>
<dt><code><strong>type</strong></code></dt>
<dd>A string that matches the <code>name</code> parameter of the extension.</dd>

<dt><code><strong>raw</strong></code></dt>
<dd>A string containing all of the text that this token consumes from the source.</dd>

<dt><code><strong>tokens</strong> [optional]</code></dt>
<dd>An array of child tokens that will be traversed by the <code>walkTokens</code> function by default.</dd>
</dl>

The returned token can also contain any other custom parameters of your choice that your custom `renderer` might need to access.

The tokenizer function has access to the lexer in the `this` object, which can be used if any internal section of the string needs to be parsed further, such as in handling any inline syntax on the text within a block token. The key functions that may be useful include:

<dl>
<dt><code><strong>this.lexer.blockTokens</strong>(<i>string</i> text, <i>array</i> tokens)</code></dt>
<dd>This runs the block tokenizer functions (including any block-level extensions) on the provided text, and appends any resulting tokens onto the <code>tokens</code> array. The <code>tokens</code> array is also returned by the function. You might use this, for example, if your extension creates a "container"-type token (such as a blockquote) that can potentially include other block-level tokens inside.</dd>

<dt><code><strong>this.lexer.inline</strong>(<i>string</i> text, <i>array</i> tokens)</code></dt>
<dd>Parsing of inline-level tokens only occurs after all block-level tokens have been generated. This function adds <code>text</code> and <code>tokens</code> to a queue to be processed using inline-level tokenizers (including any inline-level extensions) at that later step. Tokens will be generated using the provided <code>text</code>, and any resulting tokens will be appended to the <code>tokens</code> array. Note that this function does **NOT** return anything since the inline processing cannot happen until the block-level processing is complete.</dd>

<dt><code><strong>this.lexer.inlineTokens</strong>(<i>string</i> text, <i>array</i> tokens)</code></dt>
<dd>Sometimes an inline-level token contains further nested inline tokens (such as a <pre><code>**strong**</code></pre> token inside of a <pre><code>### Heading</code></pre>). This runs the inline tokenizer functions (including any inline-level extensions) on the provided text, and appends any resulting tokens onto the <code>tokens</code> array. The <code>tokens</code> array is also returned by the function.</dd>
</dl>
</dd>

<dt><code><strong>renderer</strong>(<i>object</i> token)</code></dt>
<dd>A function that reads a token and returns the generated HTML output string.

The renderer function has access to the parser in the `this` object, which can be used if any part of the token needs needs to be parsed further, such as any child tokens. The key functions that may be useful include:

<dl>
<dt><code><strong>this.parser.parse</strong>(<i>array</i> tokens)</code></dt>
<dd>Runs the block renderer functions (including any extensions) on the provided array of tokens, and returns the resulting HTML string output. This is used to generate the HTML from any child block-level tokens, for example if your extension is a "container"-type token (such as a blockquote) that can potentially include other block-level tokens inside.</dd>

<dt><code><strong>this.parser.parseInline</strong>(<i>array</i> tokens)</code></dt>
<dd>Runs the inline renderer functions (including any extensions) on the provided array of tokens, and returns the resulting HTML string output. This is used to generate the HTML from any child inline-level tokens.</dd>
</dl>
</dd>

<dt><code><strong>childTokens</strong> [optional]</code></dt>
<dd>An array of strings that match the names of any token parameters that should be traversed by the <code>walkTokens</code> functions. For instance, if you want to use a second custom parameter to contain child tokens in addition to <code>tokens</code>, it could be listed here. If <code>childTokens</code> is provided, the <code>tokens</code> array will not be walked by default unless it is also included in the <code>childTokens</code> array.</dd>
</dl>

Not for this drive: > Note: If you would like to release an extension as an npm package you may use the [Marked Extension Template](https://github.com/markedjs/marked-extension-template) which includes all of the things you need to get started. Feel free to create an issue in that [repo](https://github.com/markedjs/marked-extension-template) if you need help.

**Example:** <a name="custom-extensions-example"></a>Add a custom syntax to generate `<dl>` description lists.

``` js
const descriptionList = {
  name: 'descriptionList',
  level: 'block',                                     // Is this a block-level or inline-level tokenizer?
  start(src) { return src.match(/:[^:\n]/)?.index; }, // Hint to Marked.js to stop and check for a match
  tokenizer(src, tokens) {
    const rule = /^(?::[^:\n]+:[^:\n]*(?:\n|$))+/;    // Regex for the complete token, anchor to string start
    const match = rule.exec(src);
    if (match) {
      const token = {                                 // Token to generate
        type: 'descriptionList',                      // Should match "name" above
        raw: match[0],                                // Text to consume from the source
        text: match[0].trim(),                        // Additional custom properties
        tokens: []                                    // Array where child inline tokens will be generated
      };
      this.lexer.inline(token.text, token.tokens);    // Queue this data to be processed for inline tokens
      return token;
    }
  },
  renderer(token) {
    return `<dl>${this.parser.parseInline(token.tokens)}\n</dl>`; // parseInline to turn child tokens into HTML
  }
};

const description = {
  name: 'description',
  level: 'inline',                                 // Is this a block-level or inline-level tokenizer?
  start(src) { return src.match(/:/)?.index; },    // Hint to Marked.js to stop and check for a match
  tokenizer(src, tokens) {
    const rule = /^:([^:\n]+):([^:\n]*)(?:\n|$)/;  // Regex for the complete token, anchor to string start
    const match = rule.exec(src);
    if (match) {
      return {                                         // Token to generate
        type: 'description',                           // Should match "name" above
        raw: match[0],                                 // Text to consume from the source
        dt: this.lexer.inlineTokens(match[1].trim()),  // Additional custom properties, including
        dd: this.lexer.inlineTokens(match[2].trim())   //   any further-nested inline tokens
      };
    }
  },
  renderer(token) {
    return `\n<dt>${this.parser.parseInline(token.dt)}</dt><dd>${this.parser.parseInline(token.dd)}</dd>`;
  },
  childTokens: ['dt', 'dd'],                 // Any child tokens to be visited by walkTokens
};

function walkTokens(token) {                        // Post-processing on the completed token tree
  if (token.type === 'strong') {
    token.text += ' walked';
    token.tokens = this.Lexer.lexInline(token.text)
  }
}
marked.use({ extensions: [descriptionList, description], walkTokens });

// EQUIVALENT TO:

marked.use({ extensions: [descriptionList] });
marked.use({ extensions: [description]     });
marked.use({ walkTokens })

console.log(marked.parse('A Description List:\n'
                 + ':   Topic 1   :  Description 1\n'
                 + ': **Topic 2** : *Description 2*'));
```

**Output**

``` bash
<p>A Description List:</p>
<dl>
<dt>Topic 1</dt><dd>Description 1</dd>
<dt><strong>Topic 2 walked</strong></dt><dd><em>Description 2</em></dd>
</dl>
```

***

<h2 id="async">Async Marked : <code>async</code></h2>

Marked will return a promise if the `async` option is true. The `async` option will tell marked to await any `walkTokens` functions before parsing the tokens and returning an HTML string.

Simple Example:

```js
const walkTokens = async (token) => {
  if (token.type === 'link') {
    try {
      await fetch(token.href);
    } catch (ex) {
      token.title = 'invalid';
    }
  }
};

marked.use({ walkTokens, async: true });

const markdown = `
[valid link](https://example.com)

[invalid link](https://invalidurl.com)
`;

const html = await marked.parse(markdown);
```

Custom Extension Example:

```js
const importUrl = {
  extensions: [{
    name: 'importUrl',
    level: 'block',
    start(src) { return src.indexOf('\n:'); },
    tokenizer(src) {
      const rule = /^:(https?:\/\/.+?):/;
      const match = rule.exec(src);
      if (match) {
        return {
          type: 'importUrl',
          raw: match[0],
          url: match[1],
          html: '' // will be replaced in walkTokens
        };
      }
    },
    renderer(token) {
      return token.html;
    }
  }],
  async: true, // needed to tell marked to return a promise
  async walkTokens(token) {
    if (token.type === 'importUrl') {
      const res = await fetch(token.url);
      token.html = await res.text();
    }
  }
};

marked.use(importUrl);

const markdown = `
# example.com

:https://example.com:
`;

const html = await marked.parse(markdown);
```

<h2 id="lexer">The Lexer</h2>

The lexer takes a markdown string and calls the tokenizer functions.


<h2 id="parser">The Parser</h2>

The parser takes tokens as input and calls the renderer functions.

<h2 id="extend">Access to Lexer and Parser</h2>

You also have direct access to the lexer and parser if you so desire. The lexer and parser options are the same as passed to `marked.setOptions()` except they have to be full options objects, they don't get merged with the current or default options.

``` js
const tokens = marked.lexer(markdown, options);
console.log(marked.parser(tokens, options));
```

``` js
const lexer = new marked.Lexer(options);
const tokens = lexer.lex(markdown);
console.log(tokens);
console.log(lexer.tokenizer.rules.block); // block level rules used
console.log(lexer.tokenizer.rules.inline); // inline level rules used
console.log(marked.Lexer.rules.block); // all block level rules
console.log(marked.Lexer.rules.inline); // all inline level rules
```

Note that the lexer can be used in two different ways:

- `marked.lexer()`: this method tokenizes a string and returns its tokens. Subsequent calls to `lexer()` ignore any previous calls.
- `new marked.Lexer().lex()`: this instance tokenizes a string and returns its tokens along with any previous tokens. Subsequent calls to `lex()` accumulate tokens.

``` bash
$ node
> require('marked').lexer('> I am using marked.')
[
  {
    type: "blockquote",
    raw: "> I am using marked.",
    tokens: [
      {
        type: "paragraph",
        raw: "I am using marked.",
        text: "I am using marked.",
        tokens: [
          {
            type: "text",
            raw: "I am using marked.",
            text: "I am using marked."
          }
        ]
      }
    ]
  },
  links: {}
]
```

The Lexer builds an array of tokens, which will be passed to the Parser.
The Parser processes each token in the token array:

``` js
import { marked } from 'marked';

const md = `
  # heading

  [link][1]

  [1]: #heading "heading"
`;

const tokens = marked.lexer(md);
console.log(tokens);

const html = marked.parser(tokens);
console.log(html);
```

``` bash
[
  {
    type: "heading",
    raw: "  # heading\n\n",
    depth: 1,
    text: "heading",
    tokens: [
      {
        type: "text",
        raw: "heading",
        text: "heading"
      }
    ]
  },
  {
    type: "paragraph",
    raw: "  [link][1]",
    text: "  [link][1]",
    tokens: [
      {
        type: "text",
        raw: "  ",
        text: "  "
      },
      {
        type: "link",
        raw: "[link][1]",
        text: "link",
        href: "#heading",
        title: "heading",
        tokens: [
          {
            type: "text",
            raw: "link",
            text: "link"
          }
        ]
      }
    ]
  },
  {
    type: "space",
    raw: "\n\n"
  },
  links: {
    "1": {
      href: "#heading",
      title: "heading"
    }
  }
]
<h1>heading</h1>
<p>  <a href="#heading" title="heading">link</a></p>
```



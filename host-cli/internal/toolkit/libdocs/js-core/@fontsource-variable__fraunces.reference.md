# @fontsource-variable/fraunces 5.3.0 reference

This is the project's own documentation, joined at the version on this drive. Here nothing is installed or downloaded: import the package by name in a module script and pack the page with `svalbard-drive html pack`. Code blocks marked `not-for-this-stick` and lines starting "Not for this drive:" show package managers, CDN links or downloads; this drive does not use them.

## Fontsource Fraunces

Not for this drive: [![npm (scoped)](https://img.shields.io/npm/v/@fontsource-variable/fraunces?color=brightgreen)](https://www.npmjs.com/package/@fontsource-variable/fraunces) [![Generic badge](https://img.shields.io/badge/fontsource-passing-brightgreen)](https://github.com/fontsource/fontsource) [![Monthly downloads](https://badgen.net/npm/dm/@fontsource-variable/fraunces)](https://github.com/fontsource/fontsource) [![Total downloads](https://badgen.net/npm/dt/@fontsource-variable/fraunces)](https://github.com/fontsource/fontsource)

The CSS and web font files to easily self-host the “Fraunces” variable font. Please visit the main [Fontsource website](https://fontsource.org/fonts/fraunces) to view more details on this package.

## Fontsource Fraunces: Quick Installation

Fontsource offers multiple methods to import the CSS, including using a bundler like Vite or using SASS. You can find full documentation [here](https://fontsource.org/docs/getting-started/introduction).

```not-for-this-stick
npm install @fontsource-variable/fraunces
```

Within your app entry file or site component, import it in.

```javascript
import "@fontsource-variable/fraunces"; // Defaults to wght axis
import "@fontsource-variable/fraunces/wght.css"; // Specify axis
import "@fontsource-variable/fraunces/wght-italic.css"; // Specify axis and style
```

Supported variables:
- Weights: `[100,200,300,400,500,600,700,800,900]`
- Styles: `[italic,normal]`
- Subsets: `[latin,latin-ext,vietnamese]`
- Axes: `[opsz,wght,SOFT,WONK]`

> Note: `italic` may not be supported by all fonts. To learn more about what axes and styles are supported, please visit the [Fontsource website](https://fontsource.org/fonts/fraunces).

Finally, you can reference the font name in a CSS stylesheet, CSS Module, or CSS-in-JS.

```css
body {
  font-family: "Fraunces Variable";
}
```

## Fontsource Fraunces: Licensing
Always make sure to read the license for each font you use. Most of the fonts in the collection use the SIL Open Font License, v1.1. Some fonts use the Apache 2 license. The Ubuntu fonts use the Ubuntu Font License v1.0.

Copyright 2020 The Fraunces Project Authors (github.com/undercasetype/Fraunces) Fraunces-Italic[SOFT,WONK,opsz,wght].ttf: Copyright 2020 The Fraunces Project Authors (github.com/undercasetype/Fraunces)
[OFL-1.1](https://openfontlicense.org)

## Fontsource Fraunces: Other Notes
Font version (provided by source): `v38`.

If you have any suggestions or ideas to improve the performance of font loading or expand the existing library, feel free to star and contribute to this repository. You can share your suggestions or ideas by creating an [issue](https://github.com/fontsource/fontsource/issues).


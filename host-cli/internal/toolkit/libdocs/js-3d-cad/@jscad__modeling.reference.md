# @jscad/modeling 2.13.0 reference

This is the project's own documentation, joined at the version on this drive. Here nothing is installed or downloaded: import the package by name in a module script and pack the page with `svalbard-drive html pack`. Code blocks marked `not-for-this-stick` and lines starting "Not for this drive:" show package managers, CDN links or downloads; this drive does not use them.

## @jscad/modeling

> Solid Modelling Library for 2D and 3D Geometries

[![NPM version](https://badge.fury.io/js/%40jscad%2Fmodeling.svg)](https://www.npmjs.com/package/@jscad/modeling)
Not for this drive: [![NPM downloads](https://img.shields.io/npm/dw/@jscad/modeling)](https://www.npmjs.com/package/@jscad/modeling)
[![Build Status](https://travis-ci.org/jscad/OpenJSCAD.org.svg?branch=master)](https://travis-ci.org/jscad/OpenJSCAD.org)
[![Stability](https://img.shields.io/badge/stability-stable-success)](https://github.com/emersion/stability-badges#stable)
[![License](https://img.shields.io/github/license/jscad/OpenJSCAD.org)](https://github.com/jscad/OpenJSCAD.org/blob/master/LICENSE)

[![User Group](https://img.shields.io/badge/maintained%20by-user%20group-blue)](https://openjscad.nodebb.com/)
[![Lerna](https://img.shields.io/badge/maintained%20with-lerna-blue)](https://lerna.js.org/)
[![JavaScript Style Guide](https://img.shields.io/badge/code_style-standard-blue)](https://standardjs.com)

[![Backers](https://img.shields.io/opencollective/backers/openjscad)](https://opencollective.com/openjscad)
[![Sponsors](https://img.shields.io/opencollective/sponsors/openjscad)](https://opencollective.com/openjscad)

Not for this drive: <a href="https://opencollective.com/openjscad"><img src="https://opencollective.com/openjscad/donate/button.png?color=blue" alt="Open Collective"></a>

## @jscad/modeling: Overview

This library contains boolean operations based on Constructive Solid Geometry (CSG). CSG is a modelling technique that uses boolean operations like union and intersection to combine 3D solids. This library implements CSG operations on meshes elegantly and concisely using BSP trees, and is meant to serve as an easily understandable implementation of the algorithm.

## @jscad/modeling: Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [Community](#community)
- [Copyrights](#copyrights)
- [License](#license)

## @jscad/modeling: Installation

This package is published and available for download via [NPM](https://www.npmjs.com/org/jscad)
```not-for-this-stick
npm install @jscad/modeling
```

In addition this package is hosted at [UNPKG](https://unpkg.com/), and can be imported directly into webpages.

```

```

## @jscad/modeling: Usage

As a Node module:

```
const {color, connectors, geometry, math, primitives, text, utils} = require('@jscad/modeling')
const {booleans, expansions, extrusions, hulls, measurements, transforms} = require('@jscad/modeling')
```

## @jscad/modeling: Documentation

- [API Reference](https://openjscad.xyz/docs/).
- [JSCAD User Guide](https://openjscad.xyz/guide.html)

## @jscad/modeling: Contributing

The various JSCAD packages and all source code are part of the JSCAD Organization, and maintained by a group of volunteers.
We welcome and encourage anyone to pitch in but please take a moment to read the following guidelines.

* If you want to submit a bug report please make sure to follow the [Reporting Issues](https://github.com/jscad/OpenJSCAD.org/wiki/Reporting-Issues) guide. Bug reports are accepted as [Issues](https://github.com/jscad/OpenJSCAD.org/issues/) via GitHub.

* If you want to submit a change or a patch, please read the [Contributing Guide](../../CONTRIBUTING.md). New contributions are accepted as [Pull Requests](https://github.com/jscad/OpenJSCAD.org/pulls/) via GitHub.

* We only accept bug reports and pull requests on **GitHub**.

* If you have a question about how to use JSCAD, then please start a conversation at the [JSCAD User Group](https://openjscad.xyz/forum.html). You might find the answer in the [JSCAD User Guide](https://openjscad.xyz/guide.html).

* If you have a change or new feature in mind, please start a conversation with the [Core Developers](https://openjscad.xyz/forum.html) and start contributing changes.

Small Note: If editing this README, please conform to the [standard-readme](https://github.com/RichardLitt/standard-readme) specification.

## @jscad/modeling: Community

Discuss designs or issues
* [JSCAD User Group](https://openjscad.xyz/forum.html)

Or chat with other users
* [Discord Community](https://openjscad.xyz/discord.html)

## @jscad/modeling: Copyrights

Some copyrights apply from integration of original libraries.

CSG Library : Copyright (c) 2012 Joost Nieuwenhuijse, under the MIT license. Copyright (c) 2011 Evan Wallace, under MIT license.

Portions of glMatrix Library: Copyright (c) 2015-2020, Brandon Jones, Colin MacKenzie IV, under MIT license.

Quickhull Library: Copyright (c) 2015 Mauricio Poppe, under the MIT license.

## @jscad/modeling: License

[The MIT License (MIT)](../../LICENSE)
(unless specified otherwise)


## Types

The package's type definitions, 372 files.

### src/colors/colorNameToRgb.d.ts

```ts
import { RGB } from './types'

export default colorNameToRgb

declare function colorNameToRgb(s: string): RGB
```

### src/colors/colorize.d.ts

```ts
import { Colored, Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

import { RGB, RGBA } from './types'

export default colorize

// Single Geom3 returns Colored Geom3
declare function colorize<T extends Geometry>(color: RGB | RGBA, object: T): T & Colored

// List of Geom3 returns list of Colored Geom3
declare function colorize<T extends Geometry>(color: RGB | RGBA, ...objects: RecursiveArray<T>): Array<T & Colored>
// List of mixed geometries returns list of colored geometries
declare function colorize(color: RGB | RGBA, ...objects: RecursiveArray<Geometry>): Array<Geometry & Colored>
```

### src/colors/cssColors.d.ts

```ts
import { RGB } from './types'

export const black: RGB
export const silver: RGB
export const gray: RGB
export const white: RGB
export const maroon: RGB
export const red: RGB
export const purple: RGB
export const fuchsia: RGB
export const green: RGB
export const lime: RGB
export const olive: RGB
export const yellow: RGB
export const navy: RGB
export const blue: RGB
export const teal: RGB
export const aqua: RGB
export const aliceblue: RGB
export const antiquewhite: RGB
export const aquamarine: RGB
export const azure: RGB
export const beige: RGB
export const bisque: RGB
export const blanchedalmond: RGB
export const blueviolet: RGB
export const brown: RGB
export const burlywood: RGB
export const cadetblue: RGB
export const chartreuse: RGB
export const chocolate: RGB
export const coral: RGB
export const cornflowerblue: RGB
export const cornsilk: RGB
export const crimson: RGB
export const cyan: RGB
export const darkblue: RGB
export const darkcyan: RGB
export const darkgoldenrod: RGB
export const darkgray: RGB
export const darkgreen: RGB
export const darkgrey: RGB
export const darkkhaki: RGB
export const darkmagenta: RGB
export const darkolivegreen: RGB
export const darkorange: RGB
export const darkorchid: RGB
export const darkred: RGB
export const darksalmon: RGB
export const darkseagreen: RGB
export const darkslateblue: RGB
export const darkslategray: RGB
export const darkslategrey: RGB
export const darkturquoise: RGB
export const darkviolet: RGB
export const deeppink: RGB
export const deepskyblue: RGB
export const dimgray: RGB
export const dimgrey: RGB
export const dodgerblue: RGB
export const firebrick: RGB
export const floralwhite: RGB
export const forestgreen: RGB
export const gainsboro: RGB
export const ghostwhite: RGB
export const gold: RGB
export const goldenrod: RGB
export const greenyellow: RGB
export const grey: RGB
export const honeydew: RGB
export const hotpink: RGB
export const indianred: RGB
export const indigo: RGB
export const ivory: RGB
export const khaki: RGB
export const lavender: RGB
export const lavenderblush: RGB
export const lawngreen: RGB
export const lemonchiffon: RGB
export const lightblue: RGB
export const lightcoral: RGB
export const lightcyan: RGB
export const lightgoldenrodyellow: RGB
export const lightgray: RGB
export const lightgreen: RGB
export const lightgrey: RGB
export const lightpink: RGB
export const lightsalmon: RGB
export const lightseagreen: RGB
export const lightskyblue: RGB
export const lightslategray: RGB
export const lightslategrey: RGB
export const lightsteelblue: RGB
export const lightyellow: RGB
export const limegreen: RGB
export const linen: RGB
export const magenta: RGB
export const mediumaquamarine: RGB
export const mediumblue: RGB
export const mediumorchid: RGB
export const mediumpurple: RGB
export const mediumseagreen: RGB
export const mediumslateblue: RGB
export const mediumspringgreen: RGB
export const mediumturquoise: RGB
export const mediumvioletred: RGB
export const midnightblue: RGB
export const mintcream: RGB
export const mistyrose: RGB
export const moccasin: RGB
export const navajowhite: RGB
export const oldlace: RGB
export const olivedrab: RGB
export const orange: RGB
export const orangered: RGB
export const orchid: RGB
export const palegoldenrod: RGB
export const palegreen: RGB
export const paleturquoise: RGB
export const palevioletred: RGB
export const papayawhip: RGB
export const peachpuff: RGB
export const peru: RGB
export const pink: RGB
export const plum: RGB
export const powderblue: RGB
export const rosybrown: RGB
export const royalblue: RGB
export const saddlebrown: RGB
export const salmon: RGB
export const sandybrown: RGB
export const seagreen: RGB
export const seashell: RGB
export const sienna: RGB
export const skyblue: RGB
export const slateblue: RGB
export const slategray: RGB
export const slategrey: RGB
export const snow: RGB
export const springgreen: RGB
export const steelblue: RGB
export const tan: RGB
export const thistle: RGB
export const tomato: RGB
export const turquoise: RGB
export const violet: RGB
export const wheat: RGB
export const whitesmoke: RGB
export const yellowgreen: RGB
```

### src/colors/hexToRgb.d.ts

```ts
import { RGB, RGBA } from './types'

export default hexToRgb

declare function hexToRgb(hex: string): RGB | RGBA
```

### src/colors/hslToRgb.d.ts

```ts
import { HSL, HSLA, RGB, RGBA } from './types'

export default hslToRgb

declare function hslToRgb(hsl:  HSL): RGB
declare function hslToRgb(hsl:  HSLA): RGBA
declare function hslToRgb(...hsl:  HSL): RGB
declare function hslToRgb(...hsl:  HSLA): RGBA
```

### src/colors/hsvToRgb.d.ts

```ts
import { HSV, HSVA, RGB, RGBA } from './types'

export default hsvToRgb

declare function hsvToRgb(hsv:  HSV): RGB
declare function hsvToRgb(hsv:  HSVA): RGBA
declare function hsvToRgb(...hsv:  HSV): RGB
declare function hsvToRgb(...hsv:  HSVA): RGBA
```

### src/colors/hueToColorComponent.d.ts

```ts
export default hueToColorComponent

declare function hueToColorComponent(p: number, q: number, t: number): number
```

### src/colors/index.d.ts

```ts
export { default as colorize } from './colorize'
export { default as colorNameToRgb } from './colorNameToRgb'
export * as cssColors from './cssColors'
export { default as hexToRgb } from './hexToRgb'
export { default as hslToRgb } from './hslToRgb'
export { default as hsvToRgb } from './hsvToRgb'
export { default as hueToColorComponent } from './hueToColorComponent'
export { default as rgbToHex } from './rgbToHex'
export { default as rgbToHsl } from './rgbToHsl'
export { default as rgbToHsv } from './rgbToHsv'

export * from './types'
export as namespace colors
```

### src/colors/rgbToHex.d.ts

```ts
import { RGB, RGBA } from './types'

export default rgbToHex

declare function rgbToHex(rgb: RGB | RGBA): string
declare function rgbToHex(...rgb: RGB | RGBA): string
```

### src/colors/rgbToHsl.d.ts

```ts
import { HSL, HSLA, RGB, RGBA } from './types'

export default rgbToHsl

declare function rgbToHsl(rgb: RGB): HSL
declare function rgbToHsl(rgb: RGBA): HSLA
declare function rgbToHsl(...rgb: RGB): HSL
declare function rgbToHsl(...rgb: RGBA): HSLA
```

### src/colors/rgbToHsv.d.ts

```ts
import { HSV, HSVA, RGB, RGBA } from './types'

export default rgbToHsv

declare function rgbToHsv(rgb: RGB): HSV
declare function rgbToHsv(rgb: RGBA): HSVA
declare function rgbToHsv(...rgb: RGB): HSV
declare function rgbToHsv(...rgb: RGBA): HSVA
```

### src/colors/types.d.ts

```ts
export type RGB = [number, number, number]
export type RGBA = [number, number, number, number]
export type HSL = [number, number, number]
export type HSLA = [number, number, number, number]
export type HSV = [number, number, number]
export type HSVA = [number, number, number, number]
```

### src/curves/bezier/arcLengthToT.d.ts

```ts
import Bezier from './type'

export default arcLengthToT

export interface ArcLengthToTOptions {
  distance?: Number
  segments?: Number
}

declare function arcLengthToT(options: ArcLengthToTOptions, bezier: Bezier): number
```

### src/curves/bezier/create.d.ts

```ts
import Bezier from './type'

export default create

declare function create(points: Array<number> | Array<Array<number>>): Bezier
```

### src/curves/bezier/index.d.ts

```ts
export { default as create } from './create'
export { default as tangentAt } from './tangentAt'
export { default as valueAt } from './valueAt'
export { default as lengths } from './lengths'
export { default as length } from './length'
export { default as arcLengthToT } from './arcLengthToT'

export { default as Bezier } from './type'
export as namespace bezier
```

### src/curves/bezier/length.d.ts

```ts
import Bezier from './type'

export default length

declare function length(segments: number, bezier: Bezier): number
```

### src/curves/bezier/lengths.d.ts

```ts
import Bezier from './type'

export default lengths

declare function lengths(segments: number, bezier: Bezier): Array<number>
```

### src/curves/bezier/tangentAt.d.ts

```ts
import Bezier from './type'

export default tangentAt

declare function tangentAt(t: number, bezier: Bezier): Array<number> | number
```

### src/curves/bezier/type.d.ts

```ts
export default Bezier

declare interface Bezier {
  points: Array<number> | Array<Array<number>>
  pointType: string
  dimensions: number
  permutations: Array<number>
  tangentPermutations: Array<number>
}
```

### src/curves/bezier/valueAt.d.ts

```ts
import Bezier from './type'

export default valueAt

declare function valueAt(t: number, bezier: Bezier): Array<number> | number
```

### src/curves/index.d.ts

```ts
export * as bezier from './bezier'

export as namespace curves
```

### src/geometries/geom2/clone.d.ts

```ts
import Geom2 from './type'

export default clone

declare function clone(geometry: Geom2): Geom2
```

### src/geometries/geom2/create.d.ts

```ts
import Geom2 from './type'
import Vec2 from '../../maths/vec2/type'

export default create

declare function create(sides?: Array<[Vec2, Vec2]>): Geom2
```

### src/geometries/geom2/fromCompactBinary.d.ts

```ts
import Geom2 from './type'

export default fromCompactBinary

declare function fromCompactBinary(data: Array<number> | Float32Array | Float64Array): Geom2
```

### src/geometries/geom2/fromPoints.d.ts

```ts
import Geom2 from './type'
import Vec2 from '../../maths/vec2/type'

export default fromPoints

declare function fromPoints(points: Array<Vec2>): Geom2
```

### src/geometries/geom2/index.d.ts

```ts
export { default as clone } from './clone'
export { default as create } from './create'
export { default as fromPoints } from './fromPoints'
export { default as fromCompactBinary } from './fromCompactBinary'
export { default as isA } from './isA'
export { default as reverse } from './reverse'
export { default as toOutlines } from './toOutlines'
export { default as toPoints } from './toPoints'
export { default as toSides } from './toSides'
export { default as toString } from './toString'
export { default as toCompactBinary } from './toCompactBinary'
export { default as transform } from './transform'
export { default as validate } from './validate'

export { default as Geom2 } from './type'
export as namespace geom2
```

### src/geometries/geom2/isA.d.ts

```ts
import Geom2 from './type'

export default isA

declare function isA(object: any): object is Geom2
```

### src/geometries/geom2/reverse.d.ts

```ts
import Geom2 from './type'

export default reverse

declare function reverse(geometry: Geom2): Geom2
```

### src/geometries/geom2/toCompactBinary.d.ts

```ts
import Geom2 from './type'

export default toCompactBinary

declare function toCompactBinary(geom: Geom2): Float32Array
```

### src/geometries/geom2/toOutlines.d.ts

```ts
import Geom2 from './type'
import Vec2 from '../../maths/vec2/type'

export default toOutlines

declare function toOutlines(geometry: Geom2): Array<Array<Vec2>>
```

### src/geometries/geom2/toPoints.d.ts

```ts
import Geom2 from './type'
import Vec2 from '../../maths/vec2/type'

export default toPoints

declare function toPoints(geometry: Geom2): Array<Vec2>
```

### src/geometries/geom2/toSides.d.ts

```ts
import Geom2 from './type'
import Vec2 from '../../maths/vec2/type'

export default toSides

declare function toSides(geometry: Geom2): Array<[Vec2, Vec2]>
```

### src/geometries/geom2/toString.d.ts

```ts
import Geom2 from './type'

export default toString

declare function toString(geometry: Geom2): string
```

### src/geometries/geom2/transform.d.ts

```ts
import Geom2 from './type'
import Mat4 from '../../maths/mat4/type'

export default transform

declare function transform(matrix: Mat4, geometry: Geom2): Geom2
```

### src/geometries/geom2/type.d.ts

```ts
import Vec2 from '../../maths/vec2/type'
import Mat4 from '../../maths/mat4/type'
import { Color } from '../types'

export default Geom2

declare interface Geom2 {
  sides: Array<[Vec2, Vec2]>
  transforms: Mat4
  color?: Color
}
```

### src/geometries/geom2/validate.d.ts

```ts
export default validate

declare function validate(object: any): void
```

### src/geometries/geom3/clone.d.ts

```ts
import Geom3 from './type'

export default clone

declare function clone(geometry: Geom3): Geom3
```

### src/geometries/geom3/create.d.ts

```ts
import Geom3 from './type'
import Poly3 from '../poly3/type'

export default create

declare function create(polygons?: Array<Poly3>): Geom3
```

### src/geometries/geom3/fromCompactBinary.d.ts

```ts
import Geom3 from './type'

export default fromCompactBinary

declare function fromCompactBinary(data: Array<number> | Float32Array | Float64Array): Geom3
```

### src/geometries/geom3/fromPoints.d.ts

```ts
import Geom3 from './type'
import Vec3 from '../../maths/vec3/type'

export default fromPoints

declare function fromPoints(points: Array<Array<Vec3>>): Geom3
```

### src/geometries/geom3/fromPointsConvex.d.ts

```ts
import Geom3 from './type'
import Vec3 from '../../maths/vec3/type'

export default fromPointsConvex

declare function fromPointsConvex(points: Array<Array<Vec3>>): Geom3
```

### src/geometries/geom3/index.d.ts

```ts
export { default as clone } from './clone'
export { default as create } from './create'
export { default as fromPointsConvex } from './fromPointsConvex'
export { default as fromPoints } from './fromPoints'
export { default as fromCompactBinary } from './fromCompactBinary'
export { default as invert } from './invert'
export { default as isA } from './isA'
export { isConvex } from './isConvex'
export { default as toPoints } from './toPoints'
export { default as toPolygons } from './toPolygons'
export { default as toString } from './toString'
export { default as toCompactBinary } from './toCompactBinary'
export { default as transform } from './transform'
export { default as validate } from './validate'

export { default as Geom3 } from './type'
export as namespace geom3
```

### src/geometries/geom3/invert.d.ts

```ts
import Geom3 from './type'

export default invert

declare function invert(geometry: Geom3): Geom3
```

### src/geometries/geom3/isA.d.ts

```ts
import Geom3 from './type'

export default isA

declare function isA(object: any): object is Geom3
```

### src/geometries/geom3/isConvex.d.ts

```ts
import type Geom3 from './type'

export function isConvex(geometry: Geom3): boolean
```

### src/geometries/geom3/toCompactBinary.d.ts

```ts
import Geom3 from './type'

export default toCompactBinary

declare function toCompactBinary(geom: Geom3): Float32Array
```

### src/geometries/geom3/toPoints.d.ts

```ts
import Geom3 from './type'
import Vec3 from '../../maths/vec3/type'

export default toPoints

declare function toPoints(geometry: Geom3): Array<Array<Vec3>>
```

### src/geometries/geom3/toPolygons.d.ts

```ts
import Geom3 from './type'
import Poly3 from '../poly3/type'

export default toPolygons

declare function toPolygons(geometry: Geom3): Array<Poly3>
```

### src/geometries/geom3/toString.d.ts

```ts
import Geom3 from './type'

export default toString

declare function toString(geometry: Geom3): string
```

### src/geometries/geom3/transform.d.ts

```ts
import Geom3 from './type'
import Mat4 from '../../maths/mat4/type'

export default transform

declare function transform(matrix: Mat4, geometry: Geom3): Geom3
```

### src/geometries/geom3/type.d.ts

```ts
import Poly3 from '../poly3/type'
import Mat4 from '../../maths/mat4/type'
import { Color } from '../types'

export default Geom3

declare interface Geom3 {
  polygons: Array<Poly3>
  transforms: Mat4
  color?: Color
}
```

### src/geometries/geom3/validate.d.ts

```ts
export default validate

declare function validate(object: any): void
```

### src/geometries/index.d.ts

```ts
export * as geom2 from './geom2'
export * as geom3 from './geom3'
export * as path2 from './path2'
export * as poly2 from './poly2'
export * as poly3 from './poly3'

export as namespace geometries
```

### src/geometries/path2/appendArc.d.ts

```ts
import Path2 from './type'
import Vec2 from '../../maths/vec2/type'

export default appendArc

export interface AppendArcOptions {
  endpoint: Vec2
  radius?: Vec2
  xaxisrotation?: number
  clockwise?: boolean
  large?: boolean
  segments?: number
}

declare function appendArc(options: AppendArcOptions, geometry: Path2): Path2
```

### src/geometries/path2/appendBezier.d.ts

```ts
import Path2 from './type'
import Vec2 from '../../maths/vec2/type'

export default appendBezier

export interface AppendBezierOptions {
  controlPoints: Array<Vec2 | null>
  segments?: number
}

declare function appendBezier(options: AppendBezierOptions, geometry: Path2): Path2
```

### src/geometries/path2/appendPoints.d.ts

```ts
import Path2 from './type'
import Vec2 from '../../maths/vec2/type'

export default appendPoints

declare function appendPoints(points: Array<Vec2>, geometry: Path2): Path2
```

### src/geometries/path2/clone.d.ts

```ts
import Path2 from './type'

export default clone

declare function clone(geometry: Path2): Path2
```

### src/geometries/path2/close.d.ts

```ts
import Path2 from './type'

export default close

declare function close(geometry: Path2): Path2
```

### src/geometries/path2/concat.d.ts

```ts
import Path2 from './type'

export default concat

declare function concat(...paths: Array<Path2>): Path2
```

### src/geometries/path2/create.d.ts

```ts
import Path2 from './type'
import Vec2 from '../../maths/vec2/type'

export default create

declare function create(points?: Array<Vec2>): Path2
```

### src/geometries/path2/equals.d.ts

```ts
import Path2 from './type'

export default equals

declare function equals(a: Path2, b: Path2): boolean
```

### src/geometries/path2/fromCompactBinary.d.ts

```ts
import Path2 from './type'

export default fromCompactBinary

declare function fromCompactBinary(data: Array<number> | Float32Array | Float64Array): Path2
```

### src/geometries/path2/fromPoints.d.ts

```ts
import Path2 from './type'
import Vec2 from '../../maths/vec2/type'

export default fromPoints

export interface FromPointsOptions {
  closed?: boolean
}

declare function fromPoints(options: FromPointsOptions, points: Array<Vec2>): Path2
```

### src/geometries/path2/index.d.ts

```ts
export { default as appendArc, AppendArcOptions } from './appendArc'
export { default as appendBezier, AppendBezierOptions } from './appendBezier'
export { default as appendPoints } from './appendPoints'
export { default as clone } from './clone'
export { default as close } from './close'
export { default as concat } from './concat'
export { default as create } from './create'
export { default as equals } from './equals'
export { default as fromPoints, FromPointsOptions } from './fromPoints'
export { default as fromCompactBinary } from './fromCompactBinary'
export { default as isA } from './isA'
export { default as reverse } from './reverse'
export { default as toPoints } from './toPoints'
export { default as toString } from './toString'
export { default as toCompactBinary } from './toCompactBinary'
export { default as transform } from './transform'
export { default as validate } from './validate'

export { default as Path2 } from './type'
export as namespace path2
```

### src/geometries/path2/isA.d.ts

```ts
import Path2 from './type'

export default isA

declare function isA(object: any): object is Path2
```

### src/geometries/path2/reverse.d.ts

```ts
import Path2 from './type'

export default reverse

declare function reverse(path: Path2): Path2
```

### src/geometries/path2/toCompactBinary.d.ts

```ts
import Path2 from './type'

export default toCompactBinary

declare function toCompactBinary(geometry: Path2): Float32Array
```

### src/geometries/path2/toPoints.d.ts

```ts
import Path2 from './type'
import Vec2 from '../../maths/vec2/type'

export default toPoints

declare function toPoints(geometry: Path2): Array<Vec2>
```

### src/geometries/path2/toString.d.ts

```ts
import Path2 from './type'

export default toString

declare function toString(geometry: Path2): string
```

### src/geometries/path2/transform.d.ts

```ts
import Path2 from './type'
import Mat4 from '../../maths/mat4/type'

export default transform

declare function transform(matrix: Mat4, geometry: Path2): Path2
```

### src/geometries/path2/type.d.ts

```ts
import Vec2 from '../../maths/vec2/type'
import Mat4 from '../../maths/mat4/type'
import { Color } from '../types'

export default Path2

declare interface Path2 {
  points: Array<Vec2>
  isClosed: boolean
  transforms: Mat4
  color?: Color
}
```

### src/geometries/path2/validate.d.ts

```ts
export default validate

declare function validate(object: any): void
```

### src/geometries/poly2/arePointsInside.d.ts

```ts
import Poly2 from './type'
import Vec2 from '../../maths/vec2/type'

export default arePointsInside

declare function arePointsInside(points: Array<Vec2>, polygon: Poly2): number
```

### src/geometries/poly2/create.d.ts

```ts
import Poly2 from './type'
import Vec2 from '../../maths/vec2/type'

export default create

declare function create(vertices?: Array<Vec2>): Poly2
```

### src/geometries/poly2/flip.d.ts

```ts
import Poly2 from './type'

export default flip

declare function flip(polygon: Poly2): Poly2
```

### src/geometries/poly2/index.d.ts

```ts
export { default as arePointsInside } from './arePointsInside'
export { default as create } from './create'
export { default as flip } from './flip'
export { default as measureArea } from './measureArea'

export { default as Poly2 } from './type'
export as namespace poly2
```

### src/geometries/poly2/measureArea.d.ts

```ts
import Poly2 from './type'

export default measureArea

declare function measureArea(polygon: Poly2): number
```

### src/geometries/poly2/type.d.ts

```ts
import Vec2 from '../../maths/vec2/type'

export default Poly2

declare interface Poly2 {
  vertices: Array<Vec2>
}
```

### src/geometries/poly3/clone.d.ts

```ts
import Poly3 from './type'

export default clone

declare function clone(polygon: Poly3): Poly3
declare function clone(out: Poly3, polygon: Poly3): Poly3
```

### src/geometries/poly3/create.d.ts

```ts
import Poly3 from './type'
import Vec3 from '../../maths/vec3/type'

export default create

declare function create(vertices?: Array<Vec3>): Poly3
```

### src/geometries/poly3/fromPoints.d.ts

```ts
import Poly3 from './type'
import Vec3 from '../../maths/vec3/type'

export default fromPoints

declare function fromPoints(points: Array<Vec3>): Poly3
```

### src/geometries/poly3/fromPointsAndPlane.d.ts

```ts
import Poly3 from './type'
import Vec3 from '../../maths/vec3/type'
import Plane from '../../maths/plane/type'

export default fromPointsAndPlane

declare function fromPointsAndPlane(vertices: Array<Vec3>, plane: Plane): Poly3
```

### src/geometries/poly3/index.d.ts

```ts
export { default as clone } from './clone'
export { default as create } from './create'
export { default as fromPoints } from './fromPoints'
export { default as fromPointsAndPlane } from './fromPointsAndPlane'
export { default as invert } from './invert'
export { default as isA } from './isA'
export { default as isConvex } from './isConvex'
export { default as measureArea } from './measureArea'
export { default as measureBoundingBox } from './measureBoundingBox'
export { default as measureBoundingSphere } from './measureBoundingSphere'
export { default as measureSignedVolume } from './measureSignedVolume'
export { default as plane } from './plane'
export { default as toPoints } from './toPoints'
export { default as toString } from './toString'
export { default as transform } from './transform'
export { default as validate } from './validate'

export { default as Poly3 } from './type'
export as namespace poly3
```

### src/geometries/poly3/invert.d.ts

```ts
import Poly3 from './type'

export default invert

declare function invert(polygon: Poly3): Poly3
```

### src/geometries/poly3/isA.d.ts

```ts
import Poly3 from './type'

export default isA

declare function isA(object: any): object is Poly3
```

### src/geometries/poly3/isConvex.d.ts

```ts
import Poly3 from './type'

export default isConvex

declare function isConvex(polygon: Poly3): boolean
```

### src/geometries/poly3/measureArea.d.ts

```ts
import Poly3 from './type'

export default measureArea

declare function measureArea(polygon: Poly3): number
```

### src/geometries/poly3/measureBoundingBox.d.ts

```ts
import Poly3 from './type'
import Vec3 from '../../maths/vec3/type'

export default measureBoundingBox

declare function measureBoundingBox(polygon: Poly3): [Vec3, Vec3]
```

### src/geometries/poly3/measureBoundingSphere.d.ts

```ts
import Poly3 from './type'
import Vec4 from '../../maths/vec4/type'

export default measureBoundingSphere

declare function measureBoundingSphere(polygon: Poly3): Vec4
```

### src/geometries/poly3/measureSignedVolume.d.ts

```ts
import Poly3 from './type'

export default measureSignedVolume

declare function measureSignedVolume(polygon: Poly3): number
```

### src/geometries/poly3/plane.d.ts

```ts
import Poly3 from './type'
import Plane from '../../maths/plane/type'

export default plane

declare function plane(polygon: Poly3): Plane;
```

### src/geometries/poly3/toPoints.d.ts

```ts
import Poly3 from './type'
import Vec3 from '../../maths/vec3/type'

export default toPoints

declare function toPoints(polygon: Poly3): Array<Vec3>
```

### src/geometries/poly3/toString.d.ts

```ts
import Poly3 from './type'

export default toString

declare function toString(polygon: Poly3): string
```

### src/geometries/poly3/transform.d.ts

```ts
import Poly3 from './type'
import Mat4 from '../../maths/mat4/type'

export default transform

declare function transform(matrix: Mat4, polygon: Poly3): Poly3
```

### src/geometries/poly3/type.d.ts

```ts
import Plane from '../../maths/plane/type'
import Vec3 from '../../maths/vec3/type'
import { Color } from '../types'

export default Poly3

declare interface Poly3 {
  vertices: Array<Vec3>
  color?: Color
  plane?: Plane
}
```

### src/geometries/poly3/validate.d.ts

```ts
export default validate

declare function validate(object: any): void
```

### src/geometries/types.d.ts

```ts
import Geom2 from './geom2/type'
import Geom3 from './geom3/type'
import Poly3 from './poly3/type'
import Path2 from './path2/type'
import { RGB, RGBA } from '../colors'

// NOTE (@ahdinosaur): Poly2 is not included as not well-supported.
// see https://github.com/jscad/OpenJSCAD.org/pull/726#issuecomment-724575265
export type Geometry = Geom2 | Geom3 | Poly3 | Path2

export type Color = RGB | RGBA

export interface Colored {
  color: Color
}

export { default as Geom2 } from './geom2/type'
export { default as Geom3 } from './geom3/type'
export { default as Poly2 } from './poly2/type'
export { default as Poly3 } from './poly3/type'
export { default as Path2 } from './path2/type'
```

### src/index.d.ts

```ts
export * as colors from './colors'
export * as curves from './curves'
export * as geometries from './geometries'
export * as maths from './maths'
export * as measurements from './measurements'
export * as primitives from './primitives'
export * as text from './text'
export * as utils from './utils'

export * as booleans from './operations/booleans'
export * as expansions from './operations/expansions'
export * as extrusions from './operations/extrusions'
export * as hulls from './operations/hulls'
export * as minkowski from './operations/minkowski'
export * as modifiers from './operations/modifiers'
export * as transforms from './operations/transforms'

export as namespace modeling
```

### src/maths/constants.d.ts

```ts
export const EPS: number
export const NEPS: number
export const spatialResolution: number
```

### src/maths/index.d.ts

```ts
export * as constants from './constants'
export * as line2 from './line2'
export * as line3 from './line3'
export * as mat4 from './mat4'
export * as plane from './plane'
export * as utils from './utils'
export * as vec2 from './vec2'
export * as vec3 from './vec3'
export * as vec4 from './vec4'

export as namespace maths
```

### src/maths/line2/clone.d.ts

```ts
import Line2 from './type'

export default clone

declare function clone(line: Line2): Line2
```

### src/maths/line2/closestPoint.d.ts

```ts
import Line2 from './type'
import Vec2 from '../vec2/type'

export default closestPoint

declare function closestPoint(line: Line2, point: Vec2): Vec2
```

### src/maths/line2/copy.d.ts

```ts
import Line2 from './type'

export default copy

declare function copy(out: Line2, line: Line2): Line2
```

### src/maths/line2/create.d.ts

```ts
import Line2 from './type'

export default create

declare function create(): Line2
```

### src/maths/line2/direction.d.ts

```ts
import Line2 from './type'
import Vec2 from '../vec2/type'

export default direction

declare function direction(line: Line2): Vec2
```

### src/maths/line2/distanceToPoint.d.ts

```ts
import Line2 from './type'
import Vec2 from '../vec2/type'

export default distanceToPoint

declare function distanceToPoint(line: Line2, point: Vec2): number
```

### src/maths/line2/equals.d.ts

```ts
import Line2 from './type'

export default equals

declare function equals(a: Line2, b: Line2): boolean
```

### src/maths/line2/fromPoints.d.ts

```ts
import Line2 from './type'
import Vec2 from '../vec2/type'

export default fromPoints

declare function fromPoints(out: Line2, point1: Vec2, point2: Vec2): Line2
```

### src/maths/line2/fromValues.d.ts

```ts
import Line2 from './type'

export default fromValues

declare function fromValues(x: number, y: number, d: number): Line2
```

### src/maths/line2/index.d.ts

```ts
export { default as clone } from './clone'
export { default as closestPoint } from './closestPoint'
export { default as copy } from './copy'
export { default as create } from './create'
export { default as direction } from './direction'
export { default as distanceToPoint } from './distanceToPoint'
export { default as equals } from './equals'
export { default as fromPoints } from './fromPoints'
export { default as fromValues } from './fromValues'
export { default as intersectPointOfLines } from './intersectPointOfLines'
export { default as origin } from './origin'
export { default as reverse } from './reverse'
export { default as toString } from './toString'
export { default as transform } from './transform'
export { default as xAtY } from './xAtY'

export { default as Line2 } from './type'
export as namespace line2
```

### src/maths/line2/intersectPointOfLines.d.ts

```ts
import Line2 from './type'
import Vec2 from '../vec2/type'

export default intersectPointOfLines

declare function intersectPointOfLines(a: Line2, b: Line2): Vec2
```

### src/maths/line2/origin.d.ts

```ts
import Line2 from './type'
import Vec2 from '../vec2/type'

export default origin

declare function origin(line: Line2): Vec2
```

### src/maths/line2/reverse.d.ts

```ts
import Line2 from './type'

export default reverse

declare function reverse(out: Line2, line: Line2): Line2
```

### src/maths/line2/toString.d.ts

```ts
import Line2 from './type'

export default toString

declare function toString(line: Line2): string
```

### src/maths/line2/transform.d.ts

```ts
import Line2 from './type'
import Mat4 from '../mat4/type'

export default transform

declare function transform(out: Line2, line: Line2, matrix: Mat4): Line2
```

### src/maths/line2/type.d.ts

```ts
export default Line2

declare type Line2 = [number, number, number]
```

### src/maths/line2/xAtY.d.ts

```ts
import Line2 from './type'

export default xAtY

declare function xAtY(line: Line2, y: number): number
```

### src/maths/line3/clone.d.ts

```ts
import Line3 from './type'

export default clone

declare function clone(line: Line3): Line3
```

### src/maths/line3/closestPoint.d.ts

```ts
import Line3 from './type'
import Vec3 from '../vec3/type'

export default closestPoint

declare function closestPoint(line: Line3, point: Vec3): Vec3
```

### src/maths/line3/copy.d.ts

```ts
import Line3 from './type'

export default copy

declare function copy(out: Line3, line: Line3): Line3
```

### src/maths/line3/create.d.ts

```ts
import Line3 from './type'

export default create

declare function create(): Line3
```

### src/maths/line3/direction.d.ts

```ts
import Line3 from './type'
import Vec3 from '../vec3/type'

export default direction

declare function direction(line: Line3): Vec3
```

### src/maths/line3/distanceToPoint.d.ts

```ts
import Line3 from './type'
import Vec3 from '../vec3/type'

export default distanceToPoint

declare function distanceToPoint(line: Line3, point: Vec3): number
```

### src/maths/line3/equals.d.ts

```ts
import Line3 from './type'

export default equals

declare function equals(a: Line3, b: Line3): boolean
```

### src/maths/line3/fromPlanes.d.ts

```ts
import Line3 from './type'
import Plane from '../plane/type'

export default fromPlanes

declare function fromPlanes(out: Line3, a: Plane, b: Plane): Line3
```

### src/maths/line3/fromPointAndDirection.d.ts

```ts
import Line3 from './type'
import Vec3 from '../vec3/type'

export default fromPointAndDirection

declare function fromPointAndDirection(out: Line3, point: Vec3, direction: Vec3): Line3
```

### src/maths/line3/fromPoints.d.ts

```ts
import Line3 from './type'
import Vec3 from '../vec3/type'

export default fromPoints

declare function fromPoints(out: Line3, point1: Vec3, point2: Vec3): Line3
```

### src/maths/line3/index.d.ts

```ts
export { default as clone } from './clone'
export { default as closestPoint } from './closestPoint'
export { default as copy } from './copy'
export { default as create } from './create'
export { default as direction } from './direction'
export { default as distanceToPoint } from './distanceToPoint'
export { default as equals } from './equals'
export { default as fromPlanes } from './fromPlanes'
export { default as fromPointAndDirection } from './fromPointAndDirection'
export { default as fromPoints } from './fromPoints'
export { default as intersectPointOfLineAndPlane } from './intersectPointOfLineAndPlane'
export { default as origin } from './origin'
export { default as reverse } from './reverse'
export { default as toString } from './toString'
export { default as transform } from './transform'

export { default as Line3 } from './type'
export as namespace line3
```

### src/maths/line3/intersectPointOfLineAndPlane.d.ts

```ts
import Line3 from './type'
import Plane from '../plane/type'
import Vec3 from '../vec3/type'

export default intersectPointOfLineAndPlane

declare function intersectPointOfLineAndPlane(line: Line3, plane: Plane): Vec3
```

### src/maths/line3/origin.d.ts

```ts
import Line3 from './type'
import Vec3 from '../vec3/type'

export default origin

declare function origin(line: Line3): Vec3
```

### src/maths/line3/reverse.d.ts

```ts
import Line3 from './type'

export default reverse

declare function reverse(out: Line3, line: Line3): Line3
```

### src/maths/line3/toString.d.ts

```ts
import Line3 from './type'

export default toString

declare function toString(line: Line3): string
```

### src/maths/line3/transform.d.ts

```ts
import Line3 from './type'
import Mat4 from '../mat4/type'

export default transform

declare function transform(out: Line3, line: Line3, matrix: Mat4): Line3
```

### src/maths/line3/type.d.ts

```ts
import Vec3 from '../vec3/type'

export default Line3

declare type Line3 = [Vec3, Vec3]
```

### src/maths/mat4/add.d.ts

```ts
import Mat4 from './type'

export default add

declare function add(out: Mat4, a: Mat4, b: Mat4): Mat4
```

### src/maths/mat4/clone.d.ts

```ts
import Mat4 from './type'

export default clone

declare function clone(matrix: Mat4): Mat4
```

### src/maths/mat4/copy.d.ts

```ts
import Mat4 from './type'

export default copy

declare function copy(out: Mat4, matrix: Mat4): Mat4
```

### src/maths/mat4/create.d.ts

```ts
import Mat4 from './type'

export default create

declare function create(): Mat4
```

### src/maths/mat4/equals.d.ts

```ts
import Mat4 from './type'

export default equals

declare function equals(a: Mat4, b: Mat4): boolean
```

### src/maths/mat4/fromRotation.d.ts

```ts
import Mat4 from './type'
import Vec3 from '../vec3/type'

export default fromRotation

declare function fromRotation(out: Mat4, rad: number, axis: Vec3): Mat4
```

### src/maths/mat4/fromScaling.d.ts

```ts
import Mat4 from './type'
import Vec3 from '../vec3/type'

export default fromScaling

declare function fromScaling(out: Mat4, vector: Vec3): Mat4
```

### src/maths/mat4/fromTaitBryanRotation.d.ts

```ts
import Mat4 from './type'

export default fromTaitBryanRotation

declare function fromTaitBryanRotation(out: Mat4, yaw: number, pitch: number, roll: number): Mat4
```

### src/maths/mat4/fromTranslation.d.ts

```ts
import Mat4 from './type'
import Vec3 from '../vec3/type'

export default fromTranslation

declare function fromTranslation(out: Mat4, vector: Vec3): Mat4
```

### src/maths/mat4/fromValues.d.ts

```ts
import Mat4 from './type'

export default fromValues

declare function fromValues(m00: number, m01: number, m02: number, m03: number, m10: number, m11: number, m12: number, m13: number, m20: number, m21: number, m22: number, m23: number, m30: number, m31: number, m32: number, m33: number): Mat4
```

### src/maths/mat4/fromVectorRotation.d.ts

```ts
import Mat4 from './type'
import Vec3 from '../vec3/type'

export default fromVectorRotation

declare function fromVectorRotation(out: Mat4, source: Vec3, target: Vec3): Mat4
```

### src/maths/mat4/fromXRotation.d.ts

```ts
import Mat4 from './type'

export default fromXRotation

declare function fromXRotation(out: Mat4, radians: number): Mat4
```

### src/maths/mat4/fromYRotation.d.ts

```ts
import Mat4 from './type'

export default fromYRotation

declare function fromYRotation(out: Mat4, radians: number): Mat4
```

### src/maths/mat4/fromZRotation.d.ts

```ts
import Mat4 from './type'

export default fromZRotation

declare function fromZRotation(out: Mat4, radians: number): Mat4
```

### src/maths/mat4/identity.d.ts

```ts
import Mat4 from './type'

export default identity

declare function identity(out: Mat4): Mat4
```

### src/maths/mat4/index.d.ts

```ts
export { default as add } from './add'
export { default as clone } from './clone'
export { default as copy } from './copy'
export { default as create } from './create'
export { default as equals } from './equals'
export { default as fromRotation } from './fromRotation'
export { default as fromScaling } from './fromScaling'
export { default as fromTaitBryanRotation } from './fromTaitBryanRotation'
export { default as fromTranslation } from './fromTranslation'
export { default as fromValues } from './fromValues'
export { default as fromXRotation } from './fromXRotation'
export { default as fromYRotation } from './fromYRotation'
export { default as fromZRotation } from './fromZRotation'
export { default as identity } from './identity'
export { default as isIdentity } from './isIdentity'
export { default as isMirroring } from './isMirroring'
export { default as mirrorByPlane } from './mirrorByPlane'
export { default as multiply } from './multiply'
export { default as rotate } from './rotate'
export { default as rotateX } from './rotateX'
export { default as rotateY } from './rotateY'
export { default as rotateZ } from './rotateZ'
export { default as scale } from './scale'
export { default as subtract } from './subtract'
export { default as toString } from './toString'
export { default as translate } from './translate'

export { default as Mat4 } from './type'
export as namespace mat4
```

### src/maths/mat4/isIdentity.d.ts

```ts
import Mat4 from './type'

export default isIdentity

declare function isIdentity(matrix: Mat4): boolean
```

### src/maths/mat4/isMirroring.d.ts

```ts
import Mat4 from './type'

export default isMirroring

declare function isMirroring(matrix: Mat4): boolean
```

### src/maths/mat4/leftMultiplyVec2.d.ts

```ts
import Mat4 from './type'
import Vec2 from '../vec2/type'

export default leftMultiplyVec2

declare function leftMultiplyVec2(vector: Vec2, matrix: Mat4): Vec2
```

### src/maths/mat4/leftMultiplyVec3.d.ts

```ts
import Mat4 from './type'
import Vec3 from '../vec3/type'

export default leftMultiplyVec3

declare function leftMultiplyVec3(vector: Vec3, matrix: Mat4): Vec3
```

### src/maths/mat4/mirror.d.ts

```ts
import Mat4 from './type'
import Vec3 from '../vec3/type'

export default mirror

declare function mirror(out: Mat4, vec: Vec3, matrix: Mat4): Mat4
```

### src/maths/mat4/mirrorByPlane.d.ts

```ts
import Mat4 from './type'
import Plane from '../plane/type'

export default mirrorByPlane

declare function mirrorByPlane(out: Mat4, plane: Plane): Mat4
```

### src/maths/mat4/multiply.d.ts

```ts
import Mat4 from './type'

export default multiply

declare function multiply(out: Mat4, a: Mat4, b: Mat4): Mat4
```

### src/maths/mat4/rightMultiplyVec2.d.ts

```ts
import Mat4 from './type'
import Vec2 from '../vec2/type'

export default rightMultiplyVec2

declare function rightMultiplyVec2(vector: Vec2, matrix: Mat4): Vec2
```

### src/maths/mat4/rightMultiplyVec3.d.ts

```ts
import Mat4 from './type'
import Vec3 from '../vec3/type'

export default rightMultiplyVec3

declare function rightMultiplyVec3(vector: Vec3, matrix: Mat4): Vec3
```

### src/maths/mat4/rotate.d.ts

```ts
import Mat4 from './type'
import Vec3 from '../vec3/type'

export default rotate

declare function rotate(out: Mat4, matrix: Mat4, radians: number, axis: Vec3): Mat4
```

### src/maths/mat4/rotateX.d.ts

```ts
import Mat4 from './type'

export default rotateX

declare function rotateX(out: Mat4, matrix: Mat4, radians: number): Mat4
```

### src/maths/mat4/rotateY.d.ts

```ts
import Mat4 from './type'

export default rotateY

declare function rotateY(out: Mat4, matrix: Mat4, radians: number): Mat4
```

### src/maths/mat4/rotateZ.d.ts

```ts
import Mat4 from './type'

export default rotateZ

declare function rotateZ(out: Mat4, matrix: Mat4, radians: number): Mat4
```

### src/maths/mat4/scale.d.ts

```ts
import Mat4 from './type'
import Vec3 from '../vec3/type'

export default scale

declare function scale(out: Mat4, matrix: Mat4, dimensions: Vec3): Mat4
```

### src/maths/mat4/subtract.d.ts

```ts
import Mat4 from './type'

export default subtract

declare function subtract(out: Mat4, a: Mat4, b: Mat4): Mat4
```

### src/maths/mat4/toString.d.ts

```ts
import Mat4 from './type'

export default toString

declare function toString(matrix: Mat4): string
```

### src/maths/mat4/translate.d.ts

```ts
import Mat4 from './type'
import Vec3 from '../vec3/type'

export default translate

declare function translate(out: Mat4, matrix: Mat4, offsets: Vec3): Mat4
```

### src/maths/mat4/type.d.ts

```ts
export default Mat4

declare type Mat4 = [
  number, number, number, number,
  number, number, number, number,
  number, number, number, number,
  number, number, number, number,
]
```

### src/maths/plane/clone.d.ts

```ts
import Plane from './type'

export default clone

declare function clone(plane: Plane): Plane
```

### src/maths/plane/copy.d.ts

```ts
import Plane from './type'

export default copy

declare function copy(out: Plane, plane: Plane): Plane
```

### src/maths/plane/create.d.ts

```ts
import Plane from './type'

export default create

declare function create(): Plane
```

### src/maths/plane/equals.d.ts

```ts
import Plane from './type'

export default equals

declare function equals(a: Plane, b: Plane): boolean
```

### src/maths/plane/flip.d.ts

```ts
import Plane from './type'

export default flip

declare function flip(out: Plane, plane: Plane): Plane
```

### src/maths/plane/fromNoisyPoints.d.ts

```ts
import Plane from './type'
import Vec3 from '../vec3/type'

export default fromNoisyPoints

declare function fromNoisyPoints(out: Plane, ...vertices: Array<Vec3>): Plane
```

### src/maths/plane/fromNormalAndPoint.d.ts

```ts
import Plane from './type'
import Vec3 from '../vec3/type'

export default fromNormalAndPoint

declare function fromNormalAndPoint(out: Plane, normal: Vec3, point: Vec3): Plane
```

### src/maths/plane/fromPoints.d.ts

```ts
import Plane from './type'
import Vec3 from '../vec3/type'

export default fromPoints

declare function fromPoints(out: Plane, ...vertices: Array<Vec3>): Plane
```

### src/maths/plane/fromPointsRandom.d.ts

```ts
import Plane from './type'
import Vec3 from '../vec3/type'

export default fromPointsRandom

declare function fromPointsRandom(out: Plane, a: Vec3, b: Vec3, c: Vec3): Plane
```

### src/maths/plane/fromValues.d.ts

```ts
import Plane from './type'

export default fromValues

declare function fromValues(x: number, y: number, z: number, w: number): Plane
```

### src/maths/plane/index.d.ts

```ts
export { default as clone } from './clone'
export { default as copy } from './copy'
export { default as create } from './create'
export { default as equals } from './equals'
export { default as flip } from './flip'
export { default as fromNormalAndPoint } from './fromNormalAndPoint'
export { default as fromValues } from './fromValues'
export { default as fromNoisyPoints } from './fromNoisyPoints'
export { default as fromPoints } from './fromPoints'
export { default as fromPointsRandom } from './fromPointsRandom'
export { default as signedDistanceToPoint } from './signedDistanceToPoint'
export { default as projectionOfPoint } from './projectionOfPoint'
export { default as toString } from './toString'
export { default as transform } from './transform'

export { default as Plane } from './type'
export as namespace plane
```

### src/maths/plane/projectionOfPoint.d.ts

```ts
import Plane from './type'
import Vec3 from '../vec3/type'

export default projectionOfPoint

declare function projectionOfPoint(plane: Plane, point: Vec3): Vec3
```

### src/maths/plane/signedDistanceToPoint.d.ts

```ts
import Plane from './type'
import Vec3 from '../vec3/type'

export default signedDistanceToPoint

declare function signedDistanceToPoint(plane: Plane, vec: Vec3): number
```

### src/maths/plane/toString.d.ts

```ts
import Plane from './type'

export default toString

declare function toString(plane: Plane): string
```

### src/maths/plane/transform.d.ts

```ts
import Plane from './type'
import Mat4 from '../mat4/type'

export default transform

declare function transform(out: Plane, plane: Plane, matrix: Mat4): Plane
```

### src/maths/plane/type.d.ts

```ts
export default Plane

declare type Plane = [number, number, number, number]
```

### src/maths/types.d.ts

```ts
export { default as Line2 } from './line2/type'
export { default as Line3 } from './line3/type'
export { default as Mat4 } from './mat4/type'
export { default as Plane } from './plane/type'
export { default as Vec1 } from './vec1/type'
export { default as Vec2 } from './vec2/type'
export { default as Vec3 } from './vec3/type'
export { default as Vec4 } from './vec4/type'
```

### src/maths/utils/aboutEqualNormals.d.ts

```ts
import Vec3 from '../vec3/type'

export default aboutEqualNormals

declare function aboutEqualNormals(a: Vec3, b: Vec3): boolean
```

### src/maths/utils/area.d.ts

```ts
import Vec2 from '../vec2/type'

export default area

declare function area(points: Array<Vec2>): number
```

### src/maths/utils/index.d.ts

```ts
export { default as aboutEqualNormals } from './aboutEqualNormals'
export { default as area } from './area'
export { default as interpolateBetween2DPointsForY } from './interpolateBetween2DPointsForY'
export { default as intersect } from './intersect'
export { default as solve2Linear } from './solve2Linear'
export { sin, cos } from './trigonometry'

export as namespace utils
```

### src/maths/utils/interpolateBetween2DPointsForY.d.ts

```ts
import Vec2 from '../vec2/type'

export default interpolateBetween2DPointsForY

declare function interpolateBetween2DPointsForY(point1: Vec2, point2: Vec2, y: number): number
```

### src/maths/utils/intersect.d.ts

```ts
import Vec2 from '../vec2/type'

export default intersect

declare function intersect(p1: Vec2, p2: Vec2, p3: Vec2, p4: Vec2): Vec2
```

### src/maths/utils/solve2Linear.d.ts

```ts
import Vec2 from '../vec2/type'

export default solve2Linear

declare function solve2Linear(a: number, b: number, c: number, d: number, u: number, v: number): Vec2
```

### src/maths/utils/trigonometry.d.ts

```ts
export function cos(radians: number): number
export function sin(radians: number): number
```

### src/maths/vec1/type.d.ts

```ts
export default Vec1

declare type Vec1 = [number]
```

### src/maths/vec2/abs.d.ts

```ts
import Vec2 from './type'

export default abs

declare function abs(out: Vec2, vector: Vec2): Vec2
```

### src/maths/vec2/add.d.ts

```ts
import Vec2 from './type'

export default add

declare function add(out: Vec2, a: Vec2, b: Vec2): Vec2
```

### src/maths/vec2/angle.d.ts

```ts
import angleRadians from './angleRadians'

export default angleRadians
```

### src/maths/vec2/angleDegrees.d.ts

```ts
import Vec2 from './type'

export default angleDegrees

declare function angleDegrees(vector: Vec2): number
```

### src/maths/vec2/angleRadians.d.ts

```ts
import Vec2 from './type'

export default angleRadians

declare function angleRadians(vector: Vec2): number
```

### src/maths/vec2/clone.d.ts

```ts
import Vec2 from './type'

export default clone

declare function clone(vec: Vec2): Vec2
```

### src/maths/vec2/copy.d.ts

```ts
import Vec2 from './type'

export default copy

declare function copy(out: Vec2, vector: Vec2): Vec2
```

### src/maths/vec2/create.d.ts

```ts
import Vec2 from './type'

export default create

declare function create(): Vec2
```

### src/maths/vec2/cross.d.ts

```ts
import Vec2 from './type'
import Vec3 from '../vec3/type'

export default cross

declare function cross(out: Vec3, a: Vec2, b: Vec2): Vec3
```

### src/maths/vec2/distance.d.ts

```ts
import Vec2 from './type'

export default distance

declare function distance(a: Vec2, b: Vec2): number
```

### src/maths/vec2/divide.d.ts

```ts
import Vec2 from './type'

export default divide

declare function divide(out: Vec2, a: Vec2, b: Vec2): Vec2
```

### src/maths/vec2/dot.d.ts

```ts
import Vec2 from './type'

export default dot

declare function dot(a: Vec2, b: Vec2): number
```

### src/maths/vec2/equals.d.ts

```ts
import Vec2 from './type'

export default equals

declare function equals(a: Vec2, b: Vec2): boolean
```

### src/maths/vec2/fromAngleDegrees.d.ts

```ts
import Vec2 from './type'

export default fromAngleDegrees

declare function fromAngleDegrees(out: Vec2, degrees: number): Vec2
```

### src/maths/vec2/fromAngleRadians.d.ts

```ts
import Vec2 from './type'

export default fromAngleRadians

declare function fromAngleRadians(out: Vec2, radians: number): Vec2
```

### src/maths/vec2/fromScalar.d.ts

```ts
import Vec2 from './type'

export default fromScalar

declare function fromScalar(out: Vec2, scalar: number): Vec2
```

### src/maths/vec2/fromValues.d.ts

```ts
import Vec2 from './type'

export default fromValues

declare function fromValues(x: number, y: number): Vec2
```

### src/maths/vec2/index.d.ts

```ts
export { default as abs } from './abs'
export { default as add } from './add'
export { default as angle } from './angle'
export { default as angleDegrees } from './angleDegrees'
export { default as angleRadians } from './angleRadians'
export { default as clone } from './clone'
export { default as copy } from './copy'
export { default as create } from './create'
export { default as cross } from './cross'
export { default as distance } from './distance'
export { default as divide } from './divide'
export { default as dot } from './dot'
export { default as equals } from './equals'
export { default as fromAngleDegrees } from './fromAngleDegrees'
export { default as fromAngleRadians } from './fromAngleRadians'
export { default as fromScalar } from './fromScalar'
export { default as fromValues } from './fromValues'
export { default as length } from './length'
export { default as lerp } from './lerp'
export { default as max } from './max'
export { default as min } from './min'
export { default as multiply } from './multiply'
export { default as negate } from './negate'
export { default as normal } from './normal'
export { default as normalize } from './normalize'
export { default as rotate } from './rotate'
export { default as scale } from './scale'
export { default as snap } from './snap'
export { default as squaredDistance } from './squaredDistance'
export { default as squaredLength } from './squaredLength'
export { default as subtract } from './subtract'
export { default as toString } from './toString'
export { default as transform } from './transform'

export { default as Vec2 } from './type'
export as namespace vec2
```

### src/maths/vec2/length.d.ts

```ts
import Vec2 from './type'

export default length

declare function length(vector: Vec2): number
```

### src/maths/vec2/lerp.d.ts

```ts
import Vec2 from './type'

export default lerp

declare function lerp(out: Vec2, a: Vec2, b: Vec2, t: number): Vec2
```

### src/maths/vec2/max.d.ts

```ts
import Vec2 from './type'

export default max

declare function max(out: Vec2, a: Vec2, b: Vec2): Vec2
```

### src/maths/vec2/min.d.ts

```ts
import Vec2 from './type'

export default min

declare function min(out: Vec2, a: Vec2, b: Vec2): Vec2
```

### src/maths/vec2/multiply.d.ts

```ts
import Vec2 from './type'

export default multiply

declare function multiply(out: Vec2, a: Vec2, b: Vec2): Vec2
```

### src/maths/vec2/negate.d.ts

```ts
import Vec2 from './type'

export default negate

declare function negate(out: Vec2, vec: Vec2): Vec2
```

### src/maths/vec2/normal.d.ts

```ts
import Vec2 from './type'

export default normal

declare function normal(out: Vec2, vec:  Vec2): Vec2
```

### src/maths/vec2/normalize.d.ts

```ts
import Vec2 from './type'

export default normalize

declare function normalize(out: Vec2, vector: Vec2): Vec2
```

### src/maths/vec2/rotate.d.ts

```ts
import Vec2 from './type'

export default rotate

declare function rotate(out: Vec2, vector: Vec2, origin: Vec2, angle: number): Vec2
```

### src/maths/vec2/scale.d.ts

```ts
import Vec2 from './type'

export default scale

declare function scale(out: Vec2, vector: Vec2, amount: number): Vec2
```

### src/maths/vec2/snap.d.ts

```ts
import Vec2 from './type'

export default snap

declare function snap(out: Vec2, vector: Vec2, epsilon: number): Vec2
```

### src/maths/vec2/squaredDistance.d.ts

```ts
import Vec2 from './type'

export default squaredDistance

declare function squaredDistance(a: Vec2, b: Vec2): number
```

### src/maths/vec2/squaredLength.d.ts

```ts
import Vec2 from './type'

export default squaredLength

declare function squaredLength(vector: Vec2): number
```

### src/maths/vec2/subtract.d.ts

```ts
import Vec2 from './type'

export default subtract

declare function subtract(out: Vec2, a: Vec2, b: Vec2): Vec2
```

### src/maths/vec2/toString.d.ts

```ts
import Vec2 from './type'

export default toString

declare function toString(vec: Vec2): string
```

### src/maths/vec2/transform.d.ts

```ts
import Vec2 from './type'
import Mat4 from '../mat4/type'

export default transform

declare function transform(out: Vec2, vector: Vec2, matrix: Mat4): Vec2
```

### src/maths/vec2/type.d.ts

```ts
export default Vec2

declare type Vec2 = [number, number]
```

### src/maths/vec3/abs.d.ts

```ts
import Vec3 from './type'

export default abs

declare function abs(out: Vec3, vector: Vec3): Vec3
```

### src/maths/vec3/add.d.ts

```ts
import Vec3 from './type'

export default add

declare function add(out: Vec3, a: Vec3, b: Vec3): Vec3
```

### src/maths/vec3/angle.d.ts

```ts
import Vec3 from './type'

export default angle

declare function angle(a: Vec3, b: Vec3): number
```

### src/maths/vec3/clone.d.ts

```ts
import Vec3 from './type'

export default clone

declare function clone(vector: Vec3): Vec3
```

### src/maths/vec3/copy.d.ts

```ts
import Vec3 from './type'

export default copy

declare function copy(out: Vec3, vector: Vec3): Vec3
```

### src/maths/vec3/create.d.ts

```ts
import Vec3 from './type'

export default create

declare function create(): Vec3
```

### src/maths/vec3/cross.d.ts

```ts
import Vec3 from './type'

export default cross

declare function cross(out: Vec3, a: Vec3, b: Vec3): Vec3
```

### src/maths/vec3/distance.d.ts

```ts
import Vec3 from './type'

export default distance

declare function distance(a: Vec3, b: Vec3): number
```

### src/maths/vec3/divide.d.ts

```ts
import Vec3 from './type'

export default divide

declare function divide(out: Vec3, a: Vec3, b: Vec3): Vec3
```

### src/maths/vec3/dot.d.ts

```ts
import Vec3 from './type'

export default dot

declare function dot(a: Vec3, b: Vec3): number
```

### src/maths/vec3/equals.d.ts

```ts
import Vec3 from './type'

export default equals

declare function equals(a: Vec3, b: Vec3): boolean
```

### src/maths/vec3/fromScalar.d.ts

```ts
import Vec3 from './type'

export default fromScalar

declare function fromScalar(out: Vec3, scalar: number): Vec3
```

### src/maths/vec3/fromValues.d.ts

```ts
import Vec3 from './type'

export default fromValues

declare function fromValues(x: number, y: number, z: number): Vec3
```

### src/maths/vec3/fromVec2.d.ts

```ts
import Vec3 from './type'
import Vec2 from '../vec2/type'

export default fromVector2

declare function fromVector2(out: Vec3, vector: Vec2, z?: number): Vec3
```

### src/maths/vec3/index.d.ts

```ts
export { default as abs } from './abs'
export { default as add } from './add'
export { default as angle } from './angle'
export { default as clone } from './clone'
export { default as copy } from './copy'
export { default as create } from './create'
export { default as cross } from './cross'
export { default as distance } from './distance'
export { default as divide } from './divide'
export { default as dot } from './dot'
export { default as equals } from './equals'
export { default as fromScalar } from './fromScalar'
export { default as fromValues } from './fromValues'
export { default as fromVec2 } from './fromVec2'
export { default as length } from './length'
export { default as lerp } from './lerp'
export { default as max } from './max'
export { default as min } from './min'
export { default as multiply } from './multiply'
export { default as negate } from './negate'
export { default as normalize } from './normalize'
export { default as orthogonal } from './orthogonal'
export { default as rotateX } from './rotateX'
export { default as rotateY } from './rotateY'
export { default as rotateZ } from './rotateZ'
export { default as scale } from './scale'
export { default as snap } from './snap'
export { default as squaredDistance } from './squaredDistance'
export { default as squaredLength } from './squaredLength'
export { default as subtract } from './subtract'
export { default as toString } from './toString'
export { default as transform } from './transform'

export { default as Vec3 } from './type'
export as namespace vec3
```

### src/maths/vec3/length.d.ts

```ts
import Vec3 from './type'

export default length

declare function length(vector: Vec3): number
```

### src/maths/vec3/lerp.d.ts

```ts
import Vec3 from './type'

export default lerp

declare function lerp(out: Vec3, a: Vec3, b: Vec3, t: number): Vec3
```

### src/maths/vec3/max.d.ts

```ts
import Vec3 from './type'

export default max

declare function max(out: Vec3, a: Vec3, b: Vec3): Vec3
```

### src/maths/vec3/min.d.ts

```ts
import Vec3 from './type'

export default min

declare function min(out: Vec3, a: Vec3, b: Vec3): Vec3
```

### src/maths/vec3/multiply.d.ts

```ts
import Vec3 from './type'

export default multiply

declare function multiply(out: Vec3, a: Vec3, b: Vec3): Vec3
```

### src/maths/vec3/negate.d.ts

```ts
import Vec3 from './type'

export default negate

declare function negate(out: Vec3, vector: Vec3): Vec3
```

### src/maths/vec3/normalize.d.ts

```ts
import Vec3 from './type'

export default normalize

declare function normalize(out: Vec3, vector: Vec3): Vec3
```

### src/maths/vec3/orthogonal.d.ts

```ts
import Vec3 from './type'

export default orthogonal

declare function orthogonal(out: Vec3, vec: Vec3): Vec3
```

### src/maths/vec3/rotateX.d.ts

```ts
import Vec3 from './type'

export default rotateX

declare function rotateX(out: Vec3, vector: Vec3, origin: Vec3, angle: number): Vec3
```

### src/maths/vec3/rotateY.d.ts

```ts
import Vec3 from './type'

export default rotateY

declare function rotateY(out: Vec3, vector: Vec3, origin: Vec3, angle: number): Vec3
```

### src/maths/vec3/rotateZ.d.ts

```ts
import Vec3 from './type'

export default rotateZ

declare function rotateZ(out: Vec3, vector: Vec3, origin: Vec3, angle: number): Vec3
```

### src/maths/vec3/scale.d.ts

```ts
import Vec3 from './type'

export default scale

declare function scale(out: Vec3, vector: Vec3, amount: number): Vec3
```

### src/maths/vec3/snap.d.ts

```ts
import Vec3 from './type'

export default snap

declare function snap(out: Vec3, vector: Vec3, epsilon: number): Vec3
```

### src/maths/vec3/squaredDistance.d.ts

```ts
import Vec3 from './type'

export default squaredDistance

declare function squaredDistance(a: Vec3, b: Vec3): number
```

### src/maths/vec3/squaredLength.d.ts

```ts
import Vec3 from './type'

export default squaredLength

declare function squaredLength(vector: Vec3): number
```

### src/maths/vec3/subtract.d.ts

```ts
import Vec3 from './type'

export default subtract

declare function subtract(out: Vec3, a: Vec3, b: Vec3): Vec3
```

### src/maths/vec3/toString.d.ts

```ts
import Vec3 from './type'

export default toString

declare function toString(vec: Vec3): string
```

### src/maths/vec3/transform.d.ts

```ts
import Vec3 from './type'
import Mat4 from '../mat4/type'

export default transform

declare function transform(out: Vec3, vector: Vec3, matrix: Mat4): Vec3
```

### src/maths/vec3/type.d.ts

```ts
export default Vec3

declare type Vec3 = [number, number, number]
```

### src/maths/vec4/clone.d.ts

```ts
import Vec4 from './type'

export default clone

declare function clone(vec: Vec4): Vec4
```

### src/maths/vec4/copy.d.ts

```ts
import Vec4 from './type'

export default copy

declare function copy(out: Vec4, vector: Vec4): Vec4
```

### src/maths/vec4/create.d.ts

```ts
import Vec4 from './type'

export default create

declare function create(): Vec4
```

### src/maths/vec4/dot.d.ts

```ts
import Vec4 from './type'

export default dot

declare function dot(a: Vec4, b: Vec4): number
```

### src/maths/vec4/equals.d.ts

```ts
import Vec4 from './type'

export default equals

declare function equals(a: Vec4, b: Vec4): boolean
```

### src/maths/vec4/fromScalar.d.ts

```ts
import Vec4 from './type'

export default fromScalar

declare function fromScalar(out: Vec4, scalar: number): Vec4
```

### src/maths/vec4/fromValues.d.ts

```ts
import Vec4 from './type'

export default fromValues

declare function fromValues(x: number, y: number, z: number, w: number): Vec4
```

### src/maths/vec4/index.d.ts

```ts
export { default as clone } from './clone'
export { default as copy } from './copy'
export { default as create } from './create'
export { default as dot } from './dot'
export { default as equals } from './equals'
export { default as fromScalar } from './fromScalar'
export { default as fromValues } from './fromValues'
export { default as toString } from './toString'
export { default as transform } from './transform'

export { default as Vec4 } from './type'
export as namespace vec4
```

### src/maths/vec4/toString.d.ts

```ts
import Vec4 from './type'

export default toString

declare function toString(vec: Vec4): string
```

### src/maths/vec4/transform.d.ts

```ts
import Vec4 from './type'
import Mat4 from '../mat4/type'

export default transform

declare function transform(out: Vec4, vector: Vec4, matrix: Mat4): Vec4
```

### src/maths/vec4/type.d.ts

```ts
export default Vec4

declare type Vec4 = [number, number, number, number]
```

### src/measurements/index.d.ts

```ts
export { default as measureAggregateArea } from './measureAggregateArea'
export { default as measureAggregateBoundingBox } from './measureAggregateBoundingBox'
export { default as measureAggregateEpsilon } from './measureAggregateEpsilon'
export { default as measureAggregateVolume } from './measureAggregateVolume'
export { default as measureArea } from './measureArea'
export { default as measureBoundingBox } from './measureBoundingBox'
export { default as measureBoundingSphere } from './measureBoundingSphere'
export { default as measureCenter } from './measureCenter'
export { default as measureCenterOfMass } from './measureCenterOfMass'
export { default as measureDimensions } from './measureDimensions'
export { default as measureEpsilon } from './measureEpsilon'
export { default as measureVolume } from './measureVolume'

export * from './types'
export as namespace measurements
```

### src/measurements/measureAggregateArea.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

export default measureAggregateArea

declare function measureAggregateArea(...geometries: RecursiveArray<Geometry>): number
```

### src/measurements/measureAggregateBoundingBox.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

import { BoundingBox } from './types'

export default measureAggregateBoundingBox

declare function measureAggregateBoundingBox(...geometries: RecursiveArray<Geometry>): BoundingBox
```

### src/measurements/measureAggregateEpsilon.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

export default measureAggregateEpsilon

declare function measureAggregateEpsilon(...geometries: RecursiveArray<Geometry>): number
```

### src/measurements/measureAggregateVolume.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

export default measureAggregateVolume

declare function measureAggregateVolume(...geometries: RecursiveArray<Geometry>): number
```

### src/measurements/measureArea.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

export default measureArea

declare function measureArea(geometry: Geometry): number
declare function measureArea(geometry: any): 0
declare function measureArea(...geometries: RecursiveArray<Geometry | any>): Array<number>
```

### src/measurements/measureBoundingBox.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

import { BoundingBox } from './types'

export default measureBoundingBox

declare function measureBoundingBox(geometry: Geometry): BoundingBox
declare function measureBoundingBox(geometry: any): [[0, 0, 0], [0, 0, 0]]
declare function measureBoundingBox(...geometries: RecursiveArray<Geometry | any>): Array<BoundingBox>
```

### src/measurements/measureBoundingSphere.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

type Centroid = [number, number, number]

export default measureBoundingSphere

declare function measureBoundingSphere(geometry: Geometry): [Centroid, number]
declare function measureBoundingSphere(...geometries: RecursiveArray<Geometry>): [Centroid, number][]
```

### src/measurements/measureCenter.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

export default measureCenter

declare function measureCenter(geometry: Geometry): [number, number, number]
declare function measureCenter(...geometries: RecursiveArray<Geometry>): [number, number, number][]
```

### src/measurements/measureCenterOfMass.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

export default measureCenterOfMass

declare function measureCenterOfMass(geometry: Geometry): [number, number, number]
declare function measureCenterOfMass(...geometries: RecursiveArray<Geometry>): [number, number, number][]
```

### src/measurements/measureDimensions.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

export default measureDimensions

declare function measureDimensions(geometry: Geometry): [number, number, number]
declare function measureDimensions(...geometries: RecursiveArray<Geometry>): [number, number, number][]
```

### src/measurements/measureEpsilon.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

export default measureEpsilon

declare function measureEpsilon(geometry: Geometry): number
declare function measureEpsilon(geometry: any): 0
declare function measureEpsilon(...geometries: RecursiveArray<Geometry | any>): Array<number>
```

### src/measurements/measureVolume.d.ts

```ts
import { Geometry } from '../geometries/types'
import RecursiveArray from '../utils/recursiveArray'

export default measureVolume

declare function measureVolume(geometry: Geometry): number
declare function measureVolume(geometry: any): 0
declare function measureVolume(...geometries: RecursiveArray<Geometry | any>): Array<number>
```

### src/measurements/types.d.ts

```ts
import Vec3 from '../maths/vec3/type'

export type BoundingBox = [Vec3, Vec3]
```

### src/operations/booleans/index.d.ts

```ts
export { default as intersect } from './intersect'
export { minkowskiSum as minkowski } from '../minkowski/minkowskiSum'
export { default as subtract } from './subtract'
export { default as union } from './union'
export { default as scission } from './scission'

export as namespace booleans
```

### src/operations/booleans/intersect.d.ts

```ts
import { Geom2, Geom3 } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

export default intersect

declare function intersect(...geometries: RecursiveArray<Geom2>): Geom2
declare function intersect(...geometries: RecursiveArray<Geom3>): Geom3
```

### src/operations/booleans/scission.d.ts

```ts
import { Geom2, Geom3 } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

export default scission

declare function scission(...geometries: RecursiveArray<Geom3>): Geom3[]
```

### src/operations/booleans/subtract.d.ts

```ts
import { Geom2, Geom3 } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

export default subtract

declare function subtract(...geometries: RecursiveArray<Geom2>): Geom2
declare function subtract(...geometries: RecursiveArray<Geom3>): Geom3
```

### src/operations/booleans/trees/splitPolygonByPlane.d.ts

```ts
import { Poly3 } from '../../../geometries/types';
import { Plane } from '../../../maths/types';

enum ResType
{
    coplanar_front = 0,
    coplanar_back = 1,
    front = 2,
    back = 3,
    spanning = 4,
}


interface SplitRes
{
    type: ResType,
    front: Poly3,
    back: Poly3;
}

// Returns object:
// .type:
//   0: coplanar-front
//   1: coplanar-back
//   2: front
//   3: back
//   4: spanning
// In case the polygon is spanning, returns:
// .front: a Polygon3 of the front part
// .back: a Polygon3 of the back part
declare function splitPolygonByPlane(plane: Plane, polygon: Poly3): SplitRes;

export default splitPolygonByPlane;
```

### src/operations/booleans/union.d.ts

```ts
import { Geom2, Geom3 } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

export default union

declare function union(...geometries: RecursiveArray<Geom2>): Geom2
declare function union(...geometries: RecursiveArray<Geom3>): Geom3
```

### src/operations/expansions/expand.d.ts

```ts
import { Path2, Geom2, Geom3 } from '../../geometries/types'
import { Corners } from '../../utils/corners'
import RecursiveArray from '../../utils/recursiveArray'

export default expand

export interface ExpandOptions {
  delta?: number
  corners?: Corners
  segments?: number
}

type Geom = Path2 | Geom2 | Geom3

declare function expand(options: ExpandOptions, geometry: Path2 | Geom2): Geom2
declare function expand(options: ExpandOptions, geometry: Geom3): Geom3
declare function expand<T extends Geom>(options?: ExpandOptions, ...geometries: RecursiveArray<T>): Array<T>
declare function expand(options?: ExpandOptions, ...geometries: RecursiveArray<Geom>): Array<Geom>
```

### src/operations/expansions/index.d.ts

```ts
export { default as expand, ExpandOptions } from './expand'
export { default as offset, OffsetOptions } from './offset'

export as namespace expansions
```

### src/operations/expansions/offset.d.ts

```ts
import { Path2, Geom2 } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

type Geometry = Path2 | Geom2

export default offset

export interface OffsetOptions {
  delta?: number
  corners?: 'edge' | 'chamfer' | 'round'
  segments?: number
}

declare function offset<T extends Geometry>(options: OffsetOptions, geometry: T): T
declare function offset(options?: OffsetOptions, ...geometries: RecursiveArray<Geometry>): Geometry
```

### src/operations/extrusions/extrudeFromSlices.d.ts

```ts
import Geom3 from '../../geometries/geom3/type'

import Slice from './slice/type'

export default extrudeFromSlices

export interface ExtrudeFromSlicesOptions<Base> {
  numberOfSlices?: number
  capStart?: boolean
  capEnd?: boolean
  close?: boolean
  callback?: (progress: number, index: number, base: Base) => Slice
}

declare function extrudeFromSlices<Base>(options: ExtrudeFromSlicesOptions<Base>, base: Base): Geom3
```

### src/operations/extrusions/extrudeHelical.d.ts

```ts
import { Geom2, Geom3 } from '../../geometries/types'

export default extrudeHelical

export interface ExtrudeHelicalOptions {
  angle?: number
  startAngle?: number
  pitch?: number
  height?: number
  endOffset?: number
  segmentsPerRotation?: number
}

declare function extrudeHelical(options: ExtrudeHelicalOptions, geometry: Geom2): Geom3
```

### src/operations/extrusions/extrudeLinear.d.ts

```ts
import { Path2, Geom2, Geom3 } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

export default extrudeLinear

export interface ExtrudeLinearOptions {
  height?: number
  twistAngle?: number
  twistSteps?: number
}

type Geometry = Path2 | Geom2

declare function extrudeLinear(options: ExtrudeLinearOptions, geometry: Geometry): Geom3
declare function extrudeLinear(options: ExtrudeLinearOptions, ...geometries: RecursiveArray<Geometry>): Geom3
```

### src/operations/extrusions/extrudeRectangular.d.ts

```ts
import { Path2, Geom2, Geom3 } from '../../geometries/types'
import { Corners } from '../../utils/corners'
import RecursiveArray from '../../utils/recursiveArray'

export default extrudeRectangular

export interface ExtrudeRectangularOptions {
  size?: number
  height?: number
  corners?: Corners
  segments?: number
}

type Geometry = Path2 | Geom2

declare function extrudeRectangular(options: ExtrudeRectangularOptions, geometry: Geometry): Geom3
declare function extrudeRectangular(options: ExtrudeRectangularOptions, ...geometries: RecursiveArray<Geometry>): Geom3
```

### src/operations/extrusions/extrudeRotate.d.ts

```ts
import { Geom2, Geom3 } from '../../geometries/types'

export default extrudeRotate

export interface ExtrudeRotateOptions {
  angle?: number
  startAngle?: number
  overflow?: 'cap'
  segments?: number;
}

declare function extrudeRotate(options: ExtrudeRotateOptions, geometry: Geom2): Geom3
```

### src/operations/extrusions/index.d.ts

```ts
export { default as extrudeFromSlices, ExtrudeFromSlicesOptions } from './extrudeFromSlices'
export { default as extrudeLinear, ExtrudeLinearOptions } from './extrudeLinear'
export { default as extrudeRectangular, ExtrudeRectangularOptions } from './extrudeRectangular'
export { default as extrudeRotate, ExtrudeRotateOptions } from './extrudeRotate'
export { default as extrudeHelical, ExtrudeHelicalOptions } from './extrudeHelical'
export { default as project, ProjectOptions } from './project'
export * as slice from './slice'

export as namespace extrusions
```

### src/operations/extrusions/project.d.ts

```ts
import { Geom2, Geom3 } from '../../geometries/types'
import { Vec3 } from '../../maths/vec3'
import RecursiveArray from '../../utils/recursiveArray'

export default project

export interface ProjectOptions {
  axis?: Vec3
  origin?: Vec3
}

declare function project(options: ProjectOptions, geometry: Geom3): Geom2
declare function project(options: ProjectOptions, ...geometries: RecursiveArray<Geom3>): Array<Geom2>
declare function project(options: ProjectOptions, ...geometries: RecursiveArray<any>): Array<any>
```

### src/operations/extrusions/slice/calculatePlane.d.ts

```ts
import Plane from '../../../maths/plane/type'

import Slice from './type'

export default calculatePlane

declare function calculatePlane(slice: Slice): Plane
```

### src/operations/extrusions/slice/clone.d.ts

```ts
import Slice from './type'

export default clone

declare function clone(slice: Slice): Slice
declare function clone(out: Slice, slice: Slice): Slice
```

### src/operations/extrusions/slice/create.d.ts

```ts
import Slice from './type'

export default create

declare function create(edges?: Slice['edges']): Slice
```

### src/operations/extrusions/slice/equals.d.ts

```ts
import Slice from './type'

export default equals

declare function equals(a: Slice, b: Slice): boolean
```

### src/operations/extrusions/slice/fromPoints.d.ts

```ts
import { Vec2, Vec3 } from '../../../maths/types'

import Slice from './type'

type Point = Vec2 | Vec3

export default fromPoints

declare function fromPoints(points: Array<Point>): Slice
```

### src/operations/extrusions/slice/fromSides.d.ts

```ts
import Geom2 from '../../../geometries/geom2/type'

import Slice from './type'

export default fromSides

declare function fromSides(sides: Geom2['sides']): Slice
```

### src/operations/extrusions/slice/index.d.ts

```ts
export { default as calculatePlane } from './calculatePlane'
export { default as clone } from './clone'
export { default as create } from './create'
export { default as equals } from './equals'
export { default as fromPoints } from './fromPoints'
export { default as fromSides } from './fromSides'
export { default as isA } from './isA'
export { default as reverse } from './reverse'
export { default as toEdges } from './toEdges'
export { default as toPolygons } from './toPolygons'
export { default as toString } from './toString'
export { default as transform } from './transform'

export { default as Slice } from './type'

export as namespace slice
```

### src/operations/extrusions/slice/isA.d.ts

```ts
import Slice from './type'

export default isA

declare function isA(object: any): object is Slice
```

### src/operations/extrusions/slice/reverse.d.ts

```ts
import Slice from './type'

export default reverse

declare function reverse(slice: Slice): Slice
declare function reverse(out: Slice, slice: Slice): Slice
```

### src/operations/extrusions/slice/toEdges.d.ts

```ts
import Slice from './type'

export default toEdges

declare function toEdges(slice: Slice): Slice['edges']
```

### src/operations/extrusions/slice/toPolygons.d.ts

```ts
import Poly3 from '../../../geometries/poly3/type'

import Slice from './type'

export default toPolygons

declare function toPolygons(slice: Slice): Array<Poly3>
```

### src/operations/extrusions/slice/toString.d.ts

```ts
import Slice from './type'

export default toString

declare function toString(slice: Slice): string
```

### src/operations/extrusions/slice/transform.d.ts

```ts
import Mat4 from '../../../maths/mat4/type'

import Slice from './type'

export default transform

declare function transform(matrix: Mat4, slice: Slice): Slice
```

### src/operations/extrusions/slice/type.d.ts

```ts
import Vec3 from '../../../maths/vec3/type'

interface Slice {
  edges: Array<[Vec3, Vec3]>
}

export default Slice
```

### src/operations/hulls/hull.d.ts

```ts
import { Geom2, Geom3, Path2 } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

export default hull

declare function hull(...geometries: RecursiveArray<Geom2>): Geom2
declare function hull(...geometries: RecursiveArray<Geom3>): Geom3
declare function hull(...geometries: RecursiveArray<Path2>): Path2
```

### src/operations/hulls/hullChain.d.ts

```ts
import { Geom2, Geom3, Path2 } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

export default hullChain

declare function hullChain(...geometries: RecursiveArray<Geom2>): Geom2
declare function hullChain(...geometries: RecursiveArray<Geom3>): Geom3
declare function hullChain(...geometries: RecursiveArray<Path2>): Path2
```

### src/operations/hulls/hullPoints2.d.ts

```ts
import Vec2 from '../../maths/vec2/type'

export default hullPoints2

declare function hullPoints2(uniquePoints: Array<Vec2>): Array<Vec2>
```

### src/operations/hulls/hullPoints3.d.ts

```ts
import Vec3 from '../../maths/vec3/type'
import Poly3 from '../../geometries/poly3/type'

export default hullPoints3

declare function hullPoints3(uniquePoints: Array<Vec3>): Array<Poly3>
```

### src/operations/hulls/index.d.ts

```ts
export { default as hull } from './hull'
export { default as hullChain } from './hullChain'
export { default as hullPoints2 } from './hullPoints2'
export { default as hullPoints3 } from './hullPoints3'

export as namespace hulls
```

### src/operations/minkowski/index.d.ts

```ts
export { minkowskiSum } from './minkowskiSum'
```

### src/operations/minkowski/minkowskiSum.d.ts

```ts
import type { Geom3 } from '../../geometries/types'

export function minkowskiSum(geometryA: Geom3, geometryB: Geom3): Geom3
export function minkowskiSum(...geometries: Geom3[]): Geom3
```

### src/operations/modifiers/generalize.d.ts

```ts
import { Geometry } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

export interface GeneralizeOptions {
  snap?: boolean
  simplify?: boolean
  triangulate?: boolean
}

export function generalize<T extends Geometry>(options: GeneralizeOptions, geometry: T): T
export function generalize<T extends Geometry>(options: GeneralizeOptions, ...geometries: RecursiveArray<T>): Array<T>
export function generalize(options: GeneralizeOptions, ...geometries: RecursiveArray<Geometry>): Array<Geometry>
```

### src/operations/modifiers/index.d.ts

```ts
export { default as generalize } from './generalize'
export { default as snap } from './snap'
export { default as retessellate } from './retessellate'
```

### src/operations/modifiers/retessellate.d.ts

```ts
import { Geom3 } from '../../geometries/types'

export default function retessellate(geometry: Geom3): Geom3
```

### src/operations/modifiers/snap.d.ts

```ts
import { Geometry } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

export function snap<T extends Geometry>(geometry: T): T
export function snap<T extends Geometry>(...geometries: RecursiveArray<T>): Array<T>
export function snap(...geometries: RecursiveArray<Geometry>): Array<Geometry>
```

### src/operations/transforms/align.d.ts

```ts
import { Geometry } from '../../geometries/types'
import RecursiveArray from '../../utils/recursiveArray'

export default align

type NullableNumber = null | number

export interface AlignOptions {
  modes?: Array<'center' | 'max' | 'min' | 'none'>
  relativeTo?: [NullableNumber] | [NullableNumber, NullableNumber] | [NullableNumber, NullableNumber, NullableNumber]
  grouped?: boolean
}

declare function align<T extends Geometry>(options: AlignOptions, geometry: T): T
declare function align<T extends Geometry>(options: AlignOptions, ...geometries: RecursiveArray<T>): Array<T>
declare function align(options: AlignOptions, ...geometries: RecursiveArray<Geometry>): Array<Geometry>
```

### src/operations/transforms/center.d.ts

```ts
import { Geometry } from '../../geometries/types'
import Vec3 from '../../maths/vec3/type'
import RecursiveArray from '../../utils/recursiveArray'

export interface CenterOptions {
  axes?: [boolean, boolean, boolean]
  relativeTo?: Vec3
}

export function center<T extends Geometry>(options: CenterOptions, geometry: T): T
export function center<T extends Geometry>(options: CenterOptions, ...geometries: RecursiveArray<T>): Array<T>
export function center(options: CenterOptions, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function centerX<T extends Geometry>(geometry: T): T
export function centerX<T extends Geometry>(...geometries: RecursiveArray<T>): Array<T>
export function centerX(...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function centerY<T extends Geometry>(geometry: T): T
export function centerY<T extends Geometry>(...geometries: RecursiveArray<T>): Array<T>
export function centerY(...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function centerZ<T extends Geometry>(geometry: T): T
export function centerZ<T extends Geometry>(...geometries: RecursiveArray<T>): Array<T>
export function centerZ(...geometries: RecursiveArray<Geometry>): Array<Geometry>
```

### src/operations/transforms/index.d.ts

```ts
export { default as align, AlignOptions } from './align'
export * from './center'
export * from './mirror'
export * from './rotate'
export * from './scale'
export * from './transform'
export * from './translate'

export as namespace transforms
```

### src/operations/transforms/mirror.d.ts

```ts
import { Geometry } from '../../geometries/types'
import { Vec3 } from '../../maths/types'
import RecursiveArray from '../../utils/recursiveArray'

export interface MirrorOptions {
  origin?: Vec3
  normal?: Vec3
}

export function mirror<T extends Geometry>(options: MirrorOptions, geometry: T): T
export function mirror<T extends Geometry>(options: MirrorOptions, ...geometries: RecursiveArray<T>): Array<T>
export function mirror(options: MirrorOptions, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function mirrorX<T extends Geometry>(geometry: T): T
export function mirrorX<T extends Geometry>(...geometries: RecursiveArray<T>): Array<T>
export function mirrorX(...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function mirrorY<T extends Geometry>(geometry: T): T
export function mirrorY<T extends Geometry>(...geometries: RecursiveArray<T>): Array<T>
export function mirrorY(...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function mirrorZ<T extends Geometry>(geometry: T): T
export function mirrorZ<T extends Geometry>(...geometries: RecursiveArray<T>): Array<T>
export function mirrorZ(...geometries: RecursiveArray<Geometry>): Array<Geometry>
```

### src/operations/transforms/rotate.d.ts

```ts
import { Geometry } from '../../geometries/types'
import { Vec1, Vec2, Vec3 } from '../../maths/types'
import RecursiveArray from '../../utils/recursiveArray'

export function rotate<T extends Geometry>(angles: Vec1 | Vec2 | Vec3, geometry: T): T
export function rotate<T extends Geometry>(angles: Vec1 | Vec2 | Vec3, ...geometries: RecursiveArray<T>): Array<T>
export function rotate(angles: Vec1 | Vec2 | Vec3, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function rotateX<T extends Geometry>(angle: number, geometry: T): T
export function rotateX<T extends Geometry>(angle: number, ...geometries: RecursiveArray<T>): Array<T>
export function rotateX(angle: number, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function rotateY<T extends Geometry>(angle: number, geometry: T): T
export function rotateY<T extends Geometry>(angle: number, ...geometries: RecursiveArray<T>): Array<T>
export function rotateY(angle: number, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function rotateZ<T extends Geometry>(angle: number, geometry: T): T
export function rotateZ<T extends Geometry>(angle: number, ...geometries: RecursiveArray<T>): Array<T>
export function rotateZ(angle: number, ...geometries: RecursiveArray<Geometry>): Array<Geometry>
```

### src/operations/transforms/scale.d.ts

```ts
import { Geometry } from '../../geometries/types'
import { Vec1, Vec2, Vec3 } from '../../maths/types'
import RecursiveArray from '../../utils/recursiveArray'

export function scale<T extends Geometry>(factors: Vec1 | Vec2 | Vec3, geometry: T): T
export function scale<T extends Geometry>(factors: Vec1 | Vec2 | Vec3, ...geometries: RecursiveArray<T>): Array<T>
export function scale(factors: Vec1 | Vec2 | Vec3, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function scaleX<T extends Geometry>(factor: number, geometry: T): T
export function scaleX<T extends Geometry>(factor: number, ...geometries: RecursiveArray<T>): Array<T>
export function scaleX(factor: number, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function scaleY<T extends Geometry>(factor: number, geometry: T): T
export function scaleY<T extends Geometry>(factor: number, ...geometries: RecursiveArray<T>): Array<T>
export function scaleY(factor: number, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function scaleZ<T extends Geometry>(factor: number, geometry: T): T
export function scaleZ<T extends Geometry>(factor: number, ...geometries: RecursiveArray<T>): Array<T>
export function scaleZ(factor: number, ...geometries: RecursiveArray<Geometry>): Array<Geometry>
```

### src/operations/transforms/transform.d.ts

```ts
import { Geometry } from '../../geometries/types'
import Mat4 from '../../maths/mat4/type'
import RecursiveArray from '../../utils/recursiveArray'

export function transform<T extends Geometry>(matrix: Mat4, geometry: T): T
export function transform<T extends Geometry>(matrix: Mat4, ...geometries: RecursiveArray<T>): Array<T>
export function transform(matrix: Mat4, ...geometries: RecursiveArray<Geometry>): Array<Geometry>
```

### src/operations/transforms/translate.d.ts

```ts
import { Geometry } from '../../geometries/types'
import { Vec1, Vec2, Vec3 } from '../../maths/types'
import RecursiveArray from '../../utils/recursiveArray'

type Vec = Vec1 | Vec2 | Vec3

export function translate<T extends Geometry>(offset: Vec, geometry: T): T
export function translate<T extends Geometry>(offset: Vec, ...geometries: RecursiveArray<T>): Array<T>
export function translate(offset: Vec, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function translateX<T extends Geometry>(offset: number, geometry: T): T
export function translateX<T extends Geometry>(offset: number, ...geometries: RecursiveArray<T>): Array<T>
export function translateX(offset: number, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function translateY<T extends Geometry>(offset: number, geometry: T): T
export function translateY<T extends Geometry>(offset: number, ...geometries: RecursiveArray<T>): Array<T>
export function translateY(offset: number, ...geometries: RecursiveArray<Geometry>): Array<Geometry>

export function translateZ<T extends Geometry>(offset: number, geometry: T): T
export function translateZ<T extends Geometry>(offset: number, ...geometries: RecursiveArray<T>): Array<T>
export function translateZ(offset: number, ...geometries: RecursiveArray<Geometry>): Array<Geometry>
```

### src/primitives/arc.d.ts

```ts
import Vec2 from '../maths/vec2/type'
import Path2 from '../geometries/path2/type'

export default arc

export interface ArcOptions {
  center?: Vec2
  radius?: number
  startAngle?: number
  endAngle?: number
  segments?: number
  makeTangent?: boolean
}

declare function arc(options?: ArcOptions): Path2
```

### src/primitives/circle.d.ts

```ts
import Vec2 from '../maths/vec2/type'
import Geom2 from '../geometries/geom2/type'

export default circle

export interface CircleOptions {
  center?: Vec2
  radius?: number
  startAngle?: number
  endAngle?: number
  segments?: number
}

declare function circle(options?: CircleOptions): Geom2
```

### src/primitives/cube.d.ts

```ts
import Vec3 from '../maths/vec3/type'
import Geom3 from '../geometries/geom3/type'

export default cube

export interface CubeOptions {
  center?: Vec3
  size?: number
}

declare function cube(options?: CubeOptions): Geom3
```

### src/primitives/cuboid.d.ts

```ts
import Vec3 from '../maths/vec3/type'
import Geom3 from '../geometries/geom3/type'

export default cuboid

export interface CuboidOptions {
  center?: Vec3
  size?: Vec3
}

declare function cuboid(options?: CuboidOptions): Geom3
```

### src/primitives/cylinder.d.ts

```ts
import Vec3 from '../maths/vec3/type'
import Geom3 from '../geometries/geom3/type'

export default cylinder

export interface CylinderOptions {
  center?: Vec3
  height?: number
  radius?: number
  segments?: number
}

declare function cylinder(options?: CylinderOptions): Geom3
```

### src/primitives/cylinderElliptic.d.ts

```ts
import Vec3 from '../maths/vec3/type'
import Geom3 from '../geometries/geom3/type'

export default cylinderElliptic

export interface CylinderEllipticOptions {
  center?: Vec3
  height?: number
  startRadius?: [number, number]
  startAngle?: number
  endRadius?: [number, number]
  endAngle?: number
  segments?: number
}

declare function cylinderElliptic(options?: CylinderEllipticOptions): Geom3
```

### src/primitives/ellipse.d.ts

```ts
import Vec2 from '../maths/vec2/type'
import Geom2 from '../geometries/geom2/type'

export default ellipse

export interface EllipseOptions {
  center?: Vec2
  radius?: Vec2
  startAngle?: number
  endAngle?: number
  segments?: number
}

declare function ellipse(options?: EllipseOptions): Geom2
```

### src/primitives/ellipsoid.d.ts

```ts
import Vec3 from '../maths/vec3/type'
import Geom3 from '../geometries/geom3/type'

export default ellipsoid

export interface EllipsoidOptions {
  center?: Vec3
  radius?: Vec3
  segments?: number
  axes?: Vec3
}

declare function ellipsoid(options?: EllipsoidOptions): Geom3
```

### src/primitives/geodesicSphere.d.ts

```ts
import Geom3 from '../geometries/geom3/type'

export default geodesicSphere

export interface GeodesicSphereOptions {
  radius?: number
  frequency?: number
}

declare function geodesicSphere(options?: GeodesicSphereOptions): Geom3
```

### src/primitives/index.d.ts

```ts
export { default as arc, ArcOptions } from './arc'
export { default as circle, CircleOptions } from './circle'
export { default as cube, CubeOptions } from './cube'
export { default as cuboid, CuboidOptions } from './cuboid'
export { default as cylinder, CylinderOptions } from './cylinder'
export { default as cylinderElliptic, CylinderEllipticOptions } from './cylinderElliptic'
export { default as ellipse, EllipseOptions } from './ellipse'
export { default as ellipsoid, EllipsoidOptions } from './ellipsoid'
export { default as geodesicSphere, GeodesicSphereOptions } from './geodesicSphere'
export { default as line } from './line'
export { default as polygon, PolygonOptions } from './polygon'
export { default as polyhedron, PolyhedronOptions } from './polyhedron'
export { default as rectangle, RectangleOptions } from './rectangle'
export { default as roundedCuboid, RoundedCuboidOptions } from './roundedCuboid'
export { default as roundedCylinder, RoundedCylinderOptions } from './roundedCylinder'
export { default as roundedRectangle, RoundedRectangleOptions } from './roundedRectangle'
export { default as sphere, SphereOptions } from './sphere'
export { default as square, SquareOptions } from './square'
export { default as star, StarOptions } from './star'
export { default as torus, TorusOptions } from './torus'
export { default as triangle, TriangleOptions } from './triangle'

export as namespace primitives
```

### src/primitives/line.d.ts

```ts
import Vec2 from '../maths/vec2/type'
import Path2 from '../geometries/path2/type'

export default line

declare function line(points: Array<Vec2>): Path2
```

### src/primitives/polygon.d.ts

```ts
import Vec2 from '../maths/vec2/type'
import Geom2 from '../geometries/geom2/type'

export default polygon

export interface PolygonOptions {
  points: Array<Vec2> | Array<Array<Vec2>>
  paths?: Array<number> | Array<Array<number>>
  orientation?: 'counterclockwise' | 'clockwise'
}

declare function polygon(options: PolygonOptions): Geom2
```

### src/primitives/polyhedron.d.ts

```ts
import Vec3 from '../maths/vec3/type'
import Geom3 from '../geometries/geom3/type'
import { RGB, RGBA } from '../colors/types'

export default polyhedron

export interface PolyhedronOptions {
  points: Array<Vec3>
  faces: Array<Array<number>>
  colors?: Array<RGB | RGBA>
  orientation?: 'outward' | 'inward'
}

declare function polyhedron(options: PolyhedronOptions): Geom3
```

### src/primitives/rectangle.d.ts

```ts
import Vec2 from '../maths/vec2/type'
import Geom2 from '../geometries/geom2/type'

export default rectangle

export interface RectangleOptions {
  center?: Vec2
  size?: Vec2
}

declare function rectangle(options?: RectangleOptions): Geom2
```

### src/primitives/roundedCuboid.d.ts

```ts
import Vec3 from '../maths/vec3/type'
import Geom3 from '../geometries/geom3/type'

export default roundedCuboid

export interface RoundedCuboidOptions {
  center?: Vec3
  size?: Vec3
  roundRadius?: number
  segments?: number
}

declare function roundedCuboid(options?: RoundedCuboidOptions): Geom3
```

### src/primitives/roundedCylinder.d.ts

```ts
import Vec3 from '../maths/vec3/type'
import Geom3 from '../geometries/geom3/type'

export default roundedCylinder

export interface RoundedCylinderOptions {
  center?: Vec3
  height?: number
  radius?: number
  roundRadius?: number
  segments?: number
}

declare function roundedCylinder(options?: RoundedCylinderOptions): Geom3
```

### src/primitives/roundedRectangle.d.ts

```ts
import Vec2 from '../maths/vec2/type'
import Geom2 from '../geometries/geom2/type'

export default roundedRectangle

export interface RoundedRectangleOptions {
  center?: Vec2
  size?: Vec2
  roundRadius?: number
  segments?: number
}

declare function roundedRectangle(options?: RoundedRectangleOptions): Geom2
```

### src/primitives/sphere.d.ts

```ts
import Vec3 from '../maths/vec3/type'
import Geom3 from '../geometries/geom3/type'

export default sphere

export interface SphereOptions {
  center?: Vec3
  radius?: number
  segments?: number
  axes?: Vec3
}

declare function sphere(options?: SphereOptions): Geom3
```

### src/primitives/square.d.ts

```ts
import Vec2 from '../maths/vec2/type'
import Geom2 from '../geometries/geom2/type'

export default square

export interface SquareOptions {
  center?: Vec2
  size?: number
}

declare function square(options?: SquareOptions): Geom2
```

### src/primitives/star.d.ts

```ts
import Vec2 from '../maths/vec2/type'
import Geom2 from '../geometries/geom2/type'

export default star

export interface StarOptions {
  center?: Vec2
  vertices?: number
  density?: number
  outerRadius?: number
  innerRadius?: number
  startAngle?: number
}

declare function star(options?: StarOptions): Geom2
```

### src/primitives/torus.d.ts

```ts
import Geom3 from '../geometries/geom3/type'

export default torus

export interface TorusOptions {
  innerRadius?: number
  outerRadius?: number
  innerSegments?: number
  outerSegments?: number
  innerRotation?: number
  outerRotation?: number
  startAngle?: number
}

declare function torus(options?: TorusOptions): Geom3
```

### src/primitives/triangle.d.ts

```ts
import Geom2 from '../geometries/geom2/type'

export default triangle

export interface TriangleOptions {
  type?: 'AAA' | 'AAS' | 'ASA' | 'SAS' | 'SSA' | 'SSS'
  values?: [number, number, number]
}

declare function triangle(options?: TriangleOptions): Geom2
```

### src/text/index.d.ts

```ts
export { default as vectorChar, VectorChar, VectorCharOptions } from './vectorChar'
export { default as vectorText, VectorText, VectorTextOptions } from './vectorText'

export as namespace text
```

### src/text/vectorChar.d.ts

```ts
import Vec2 from '../maths/vec2/type'

export default vectorChar

export interface VectorChar {
  width: number
  height: number
  segments: Array<Array<Vec2>>
}

export interface VectorCharOptions {
  xOffset?: number
  yOffset?: number
  height?: number
  extrudeOffset?: number
  input?: string
}

declare function vectorChar(): VectorChar
declare function vectorChar(char: string): VectorChar
declare function vectorChar(options: VectorCharOptions): VectorChar
declare function vectorChar(options: Omit<VectorCharOptions, 'input'>, char: string): VectorChar
```

### src/text/vectorText.d.ts

```ts
import Vec2 from '../maths/vec2/type'

export default vectorText

export interface VectorText extends Array<Array<Vec2>> {}

export interface VectorTextOptions {
  xOffset?: number
  yOffset?: number
  height?: number
  lineSpacing?: number
  letterSpacing?: number
  align?: 'left' | 'center' | 'right'
  extrudeOffset?: number
  input?: string
}

declare function vectorText(): VectorText
declare function vectorText(text: string): VectorText
declare function vectorText(options: VectorTextOptions): VectorText
declare function vectorText(options: Omit<VectorTextOptions, 'input'>, text: string): VectorText
```

### src/utils/areAllShapesTheSameType.d.ts

```ts
import { Geometry } from '../geometries/types'

export default areAllShapesTheSameType

declare function areAllShapesTheSameType(shapes: Array<Geometry>): boolean
```

### src/utils/corners.d.ts

```ts
export type Corners = 'edge' | 'chamfer' | 'round'
```

### src/utils/degToRad.d.ts

```ts
export default degToRad

declare function degToRad(degrees: number): number
```

### src/utils/flatten.d.ts

```ts
import RecursiveArray from './recursiveArray'

export default flatten

declare function flatten<T>(arr: RecursiveArray<T>): Array<T>
```

### src/utils/fnNumberSort.d.ts

```ts
export default fnNumberSort

declare function fnNumberSort(a: number, b: number): number
```

### src/utils/index.d.ts

```ts
export { default as areAllShapesTheSameType } from './areAllShapesTheSameType'
export { default as degToRad } from './degToRad'
export { default as flatten } from './flatten'
export { default as fnNumberSort } from './fnNumberSort'
export { default as insertSorted } from './insertSorted'
export { default as radiusToSegments } from './radiusToSegments'
export { default as radToDeg } from './radToDeg'

export as namespace utils
```

### src/utils/insertSorted.d.ts

```ts
export default insertSorted

declare function insertSorted<T>(array: Array<T>, element: T, comparefunc: (a: T, b: T) => number): void
```

### src/utils/padArrayToLength.d.ts

```ts
export default padArrayToLength

declare function padArrayToLength<T = number>(anArray: Array<T>, padding: T, targetLength: number): Array<T>
```

### src/utils/radToDeg.d.ts

```ts
export default radToDeg

declare function radToDeg(radians: number): number
```

### src/utils/radiusToSegments.d.ts

```ts
export default radiusToSegments

declare function radiusToSegments(radius: number, minimumLength?: number, minimumAngle?: number): number
```

### src/utils/recursiveArray.d.ts

```ts
export default RecursiveArray

declare interface RecursiveArray<T> extends Array<T | RecursiveArray<T>> {}
```


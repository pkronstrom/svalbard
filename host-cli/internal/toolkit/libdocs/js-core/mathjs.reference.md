# mathjs 15.3.0 reference

This is the project's own documentation, joined at the version on this drive. Here nothing is installed or downloaded: import the package by name in a module script and pack the page with `svalbard-drive html pack`. Code blocks marked `not-for-this-stick` and lines starting "Not for this drive:" show package managers, CDN links or downloads; this drive does not use them.

## README

![math.js](https://raw.githubusercontent.com/josdejong/mathjs/develop/misc/img/mathjs.svg)

[https://mathjs.org](https://mathjs.org)

Math.js is an extensive math library for JavaScript and Node.js. It features a flexible expression parser with support for symbolic computation, comes with a large set of built-in functions and constants, and offers an integrated solution to work with different data types like numbers, big numbers, complex numbers, fractions, units, and matrices. Powerful and easy to use.

Not for this drive: [![Version](https://img.shields.io/npm/v/mathjs.svg)](https://www.npmjs.com/package/mathjs)
Not for this drive: [![Downloads](https://img.shields.io/npm/dm/mathjs.svg)](https://www.npmjs.com/package/mathjs)
[![Build Status](https://github.com/josdejong/mathjs/workflows/Node.js%20CI/badge.svg)](https://github.com/josdejong/mathjs/actions)
[![Maintenance](https://img.shields.io/maintenance/yes/2026.svg)](https://github.com/josdejong/mathjs/graphs/commit-activity)
[![License](https://img.shields.io/github/license/josdejong/mathjs.svg)](https://github.com/josdejong/mathjs/blob/master/LICENSE)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fjosdejong%2Fmathjs.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fjosdejong%2Fmathjs?ref=badge_shield)
[![Codecov](https://codecov.io/gh/josdejong/mathjs/branch/develop/graph/badge.svg)](https://codecov.io/gh/josdejong/mathjs)
[![Github Sponsor](https://img.shields.io/github/sponsors/josdejong
)](https://github.com/sponsors/josdejong)

## README: Features

- Supports numbers, bignumbers, bigints, complex numbers, fractions, units, strings, arrays, and matrices.
- Is compatible with JavaScript's built-in Math library.
- Contains a flexible expression parser.
- Does symbolic computation.
- Comes with a large set of built-in functions and constants.
- Can be used as a command line application as well.
- Runs on any JavaScript engine.
- Is easily extensible.
- Open source.

## README: Usage

Math.js can be used in both node.js and in the browser.

Not for this drive: Install math.js using [npm](https://www.npmjs.com/package/mathjs):

Not for this drive:     npm install mathjs

Or download mathjs via one of the CDN's listed on the downloads page:

&nbsp;&nbsp;&nbsp;&nbsp;[https://mathjs.org/download.html](https://mathjs.org/download.html#download)

Math.js can be used similar to JavaScript's built-in Math library. Besides that,
math.js can evaluate
[expressions](https://mathjs.org/docs/expressions/index.html)
and supports
[chained operations](https://mathjs.org/docs/core/chaining.html).

```js
import {
  atan2, chain, derivative, e, evaluate, log, pi, pow, round, sqrt
} from 'mathjs'

// functions and constants
round(e, 3)                    // 2.718
atan2(3, -3) / pi              // 0.75
log(10000, 10)                 // 4
sqrt(-4)                       // 2i
pow([[-1, 2], [3, 1]], 2)      // [[7, 0], [0, 7]]
derivative('x^2 + x', 'x')     // 2 * x + 1

// expressions
evaluate('12 / (2.3 + 0.7)')   // 4
evaluate('12.7 cm to inch')    // 5 inch
evaluate('sin(45 deg) ^ 2')    // 0.5
evaluate('9 / 3 + 2i')         // 3 + 2i
evaluate('det([-1, 2; 3, 1])') // -7

// chaining
chain(3)
    .add(4)
    .multiply(2)
    .done()  // 14
```

See the [Getting Started](https://mathjs.org/docs/getting_started.html) for a more detailed tutorial.


## README: Browser support

Math.js works on any [ES2020](https://262.ecma-international.org/11.0/) compatible JavaScript engine, including node.js, Chrome, Firefox, Safari, and Edge.


## README: Documentation

- [Getting Started](https://mathjs.org/docs/getting_started.html)
- [Examples](https://mathjs.org/examples/index.html)
- [Overview](https://mathjs.org/docs/index.html)
- [History](https://mathjs.org/history.html)


## README: Build

First clone the project from github:

Not for this drive:     git clone git@github.com:josdejong/mathjs.git
    cd mathjs

Install the project dependencies:

Not for this drive:     npm install

Not for this drive: Then, the project can be build by executing the build script via npm:

Not for this drive:     npm run build

This will build ESM output, CommonJS output, and the bundle math.js
from the source files and put them in the folder lib.


## README: Develop

When developing new features for mathjs, it is good to be aware of the following background information.

### Code

The code of `mathjs` is written in ES modules, and requires all files to have a real, relative path, meaning the files must have a `*.js` extension. Please configure adding file extensions on auto import in your IDE.

### Architecture

What mathjs tries to achieve is to offer an environment where you can do calculations with mixed data types,
like multiplying a regular `number` with a `Complex` number or a `BigNumber`, and work with all of those in matrices.
Mathjs also allows to add a new data type with little effort.

The solution that mathjs uses has two main ingredients:

- **Typed functions**. All functions are created using [`typed-function`](https://github.com/josdejong/typed-function/). This makes it easier to (dynamically) create and extend a single function with new data types, automatically do type conversions on function inputs, etc. So, if you create function multiply for two `number`s, you can extend it with support for multiplying your own data type, say `MyDecimal`. If you define a conversion from `MyDecimal` to `number`, the typed-function will automatically allow you to multiply a `MyDecimal` with a `number`.

- **Dependency injection**. When we have a function `multiply` with support for `MyDecimal`, thanks to the dependency injection, other functions using `multiply` under the hood, like `prod`, will automatically support `MyDecimal` too. This also works the other way around: if you don't need the heavyweight `multiply` (which supports BigNumbers, matrices, etc), and you just need a plain and simple number support, you can use a lightweight implementation of `multiply` just for numbers, and inject that in `prod` and other functions.

At the lowest level, mathjs has immutable "factory" functions which create immutable functions by gathering lower-level functions they depend on, and providing implementations of the new functions for different data types. The core function `math.create(...)` creates a new instance having functions created from all passed factory functions. A mathjs instance is a collection of created functions. It contains a function like `math.import` to allow extending the instance with new functions, which can then be used in the expression parser.

### Implementing a new function

A common case is to implement a new function. This involves the following steps:

- Implement the function in the right category, for example `./src/function/arithmetic/myNewFunction.js`, where you can replace `arithmetic` with the proper category, and `myNewFunction` with the name of the new function. The implementation will generally consist of a factory as described above. Add the new function to the index files `./src/factoriesAny.js` and possibly `./src/factoriesNumber.js` (see below).
- Write documentation on the function in the source code comment of `myNewFunction.js`. This documentation is used to auto generate documentation on the website. It should include a History section with one line, indicating the upcoming version number in which the function will be created.
- Write embedded documentation for the new function in `./src/expression/embeddedDocs/function/arithmetic/myNewFunction.js`. Add the new documentation to the index file `./src/expression/embeddedDocs/embeddedDocs.js`.
- Write unit tests for the function in `./test/unit-tests/function/arithmetic/myNewFunction.test.js`.
- Write the necessary TypeScript definitions for the new function in `./types/index.d.ts`, and write tests for it in `./test/typescript-tests/testTypes.ts`. This is described in [./types/EXPLANATION.md](./types/EXPLANATION.md) -- make sure to read that page, as Typescript definitions must be added in _multiple_ places in the code.
Not for this drive: - Ensure the code style is ok by running `npm run lint` (run `npm run format` to fix the code style automatically).


The Number bundle (`./src/factoriesNumber.js`) is intended to stay lightweight,
supporting JavaScript primitive numbers, booleans, and strings. Before adding a
function to this bundle:

- Avoid dependencies on heavyweight data types such as `DenseMatrix`,
  `SparseMatrix`, `Complex`, `BigNumber`, and `Unit`. A factory shared with the
  full bundle can use optional dependencies (prefixed with `?`) for types that
  are unavailable in the Number bundle. Use a separate lightweight factory
  when the shared factory requires these types.
- Ensure all required dependencies, including their dependencies, are available
  in the Number bundle. The entry generator in `./tools/entryGenerator.js` checks
Not for this drive:   for missing required dependencies during `npm run build` (also part of
Not for this drive:   `npm run build-and-test`); no separate dependency check is needed.
- Consider array functions on a case-by-case basis: their inclusion depends
  on their size and dependencies, rather than on array support alone.


### Build scripts

The build script currently generates two types of output:

- **any**, generate entry points to create full versions of all functions
- **number**: generating and entry points to create lightweight functions just supporting `number`

For each function, an object is generated containing the factory functions of all dependencies of the function. This allows to just load a specific set of functions, and not load or bundle any other functionality. So for example, to just create function `add` you can do `math.create(addDependencies)`.


## README: Test

To execute tests for the library, install the project dependencies once:

Not for this drive:     npm install

Then, the tests can be executed:

Not for this drive:     npm test

To test the type definitions:

Not for this drive:     npm run test:types

Additionally, the tests can be run on FireFox using [headless mode](https://hacks.mozilla.org/2017/12/using-headless-mode-in-firefox/):

Not for this drive:     npm run test:browser

To run the tests remotely on LambdaTest, first set the environment variables `LT_USERNAME` and `LT_ACCESS_KEY` with your username and access key and then execute:

Not for this drive:     npm run test:lambdatest

Not for this drive: You can separately run the code linter, though it is also executed with `npm test`:

Not for this drive:     npm run lint

To automatically fix linting issue, run:

Not for this drive:     npm run format

To test code coverage of the tests:

Not for this drive:     npm run coverage

To see the coverage results, open the generated report in your browser:

    ./coverage/lcov-report/index.html


### Continuous integration testing

Continuous integration tests are run on [GitHub Actions](https://github.com/josdejong/mathjs/actions) and [TestMu AI](https://www.testmuai.com/?utm_medium=sponsor&utm_source=jos-dejong) (formerly LambdaTest) every time a commit is pushed to GitHub. GitHub Actions runs the tests for different versions of node.js, and TestMu AI runs the tests on all major browsers.

[![TestMu AI](https://raw.github.com/josdejong/mathjs/develop/misc/TestMu-AI-Black-Logo-512px.svg)](https://www.testmuai.com/?utm_medium=sponsor&utm_source=jos-de-jong)

Thanks, GitHub Actions and TestMu AI for the generous free hosting of this open source project!

## README: License

mathjs is published under the Apache 2.0 license:

```
Copyright (C) 2013-2026 Jos de Jong <wjosdejong@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

mathjs contains a JavaScript port of the [CSparse](https://github.com/DrTimothyAldenDavis/SuiteSparse/tree/dev/CSparse/Source) library, published under the LGPL-2.1+ license:

```
CSparse: a Concise Sparse matrix package.
Copyright (c) 2006, Timothy A. Davis.
http://www.suitesparse.com

--------------------------------------------------------------------------------

CSparse is free software; you can redistribute it and/or
modify it under the terms of the GNU Lesser General Public
License as published by the Free Software Foundation; either
version 2.1 of the License, or (at your option) any later version.

CSparse is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Lesser General Public License for more details.

You should have received a copy of the GNU Lesser General Public
License along with this Module; if not, write to the Free Software
Foundation, Inc., 51 Franklin St, Fifth Floor, Boston, MA  02110-1301  USA
```


## Command Line Interface (CLI)

Not for this drive: When math.js is installed globally using npm, its expression parser can be used
from the command line. To install math.js globally:

```not-for-this-stick
$ npm install -g mathjs
```

Normally, a global installation must be run with admin rights (precede the
command with `sudo`). After installation, the application `mathjs` is available
via the command line:

```bash
$ mathjs
> 12 / (2.3 + 0.7)
4
> 12.7 cm to inch
5 inch
> sin(45 deg) ^ 2
0.5
> 9 / 3 + 2i
3 + 2i
> det([-1, 2; 3, 1])
-7
```

The command line interface can be used to open a prompt, to execute a script,
or to pipe input and output streams:

```bash
$ mathjs                                 # Open a command prompt
$ mathjs script.txt                      # Run a script file, output to console
$ mathjs script1.txt script2.txt         # Run two script files
$ mathjs script.txt > results.txt        # Run a script file, output to file
$ cat script.txt | mathjs                # Run input stream, output to console
$ cat script.txt | mathjs > results.txt  # Run input stream, output to file
```

You can also use it to create LaTeX from or sanitize your expressions using the
`--tex` and `--string` options:

```bash
$ mathjs --tex
> 1/2
\frac{1}{2}
```

```bash
$ mathjs --string
> (1+1+1)
(1 + 1 + 1)
```

To change the parenthesis option use the `--parenthesis=` flag:

```bash
$ mathjs --string --parenthesis=auto
> (1+1+1)
1 + 1 + 1
```

```bash
$ mathjs --string --parenthesis=all
> (1+1+1)
(1 + 1) + 1
```

## Command Line Interface (CLI): Command line debugging (REPL)

The library also provides a REPL (Read Evaluate Print Loop) via `bin/repl.js` which
loads mathjs in a Node.js command line environment.
You can either start it directly (`./bin/repl.js`) or via node (`node bin/repl.js`).

You can exit using either [ctrl]-[C] or [ctrl]-[D].

```bash
$ ./bin/repl.js 
> math.parse('1+1')
{ op: '+',
  fn: 'add',
  args: 
   [ { value: '1', valueType: 'number' },
     { value: '1', valueType: 'number' } ] }
> 
```


## Chaining

Math.js supports chaining operations by wrapping a value into a `Chain`.
A chain can be created with the function `math.chain(value)`
(formerly `math.select(value)`).
All functions available in the math namespace can be executed via the chain.
The functions will be executed with the chain's value as the first argument,
followed by extra arguments provided by the function call itself.

```js
math.chain(3)
    .add(4)
    .subtract(2)
    .done() // 5

math.chain( [[1, 2], [3, 4]] )
    .subset(math.index(0, 0), 8)
    .multiply(3)
    .done() // [[24, 6], [9, 12]]
```

### API

A `Chain` is constructed as:

```js
math.chain()
math.chain(value)
```

The `Chain` has all functions available in the `math` namespace, and has
a number of special functions:

 - `done()`
   Finalize the chain and return the chain's value.
 - `valueOf()`
   The same as `done()`, returns the chain's value.
 - `toString()`
   Executes `math.format(value)` onto the chain's value, returning
   a string representation of the value.

Note that a "rest" or "..." parameter may not be broken across the value
in the chain and a function call. For example

```js
math.chain(3).median(4,5).done() // throws error
```

does not compute the median of 3, 4, and 5.


## Configuration

Math.js contains a number of configuration options.
These options can be applied on a created mathjs instance and changed afterwards.

```js
import { create, all } from 'mathjs'

// create a mathjs instance with configuration
const config = {
  relTol: 1e-12,
  absTol: 1e-15,
  matrix: 'Matrix',
  number: 'number',
  numberFallback: 'number',
  precision: 64,
  predictable: false,
  randomSeed: null,
  legacySubset: false
}
const math = create(all, config)

// read the applied configuration
console.log(math.config())

// change the configuration
math.config({
  number: 'BigNumber'
})
```

The following configuration options are available:

- `relTol`. The minimum relative difference used to test equality between two
  compared values. This value is used by all relational functions.
  Default value is `1e-12`.

- `absTol`. The minimum absolute difference used to test equality between two
  compared values. This value is used by all relational functions.
  Default value is `1e-15`.

- `matrix`. The default type of matrix output for functions.
  Available values are: `'Matrix'` (default) or `'Array'`.
  Where possible, the type of matrix output from functions is determined from
  the function input: An array as input will return an Array, a Matrix as input
  will return a Matrix. In case of no matrix as input, the type of output is
  determined by the option `matrix`. In case of mixed matrix
  inputs, a matrix will be returned always.

- `number`. The type used to parse strings into a numeric value or create a new
  numeric value internally.

  For most functions, the type of output is determined from the input: 
  a number as input will return a number as output, a BigNumber as input 
  returns a BigNumber as output. But for example the functions 
  `math.evaluate('2+3')`, `math.parse('2+3')`, `math.range('1:10')`, 
  and `math.unit('5cm')` use the `number` configuration setting. 

  Note that `math.sqrt(4)` will always return the number `2` regardless of 
  the `number` configuration, because the numeric type can be determined from 
  the input value.

  Available values are: `'number'` (default), `'BigNumber'`, `'bigint'`, or `'Fraction'`.
  [BigNumbers](../datatypes/bignumbers.md) have higher precision than the default numbers of JavaScript, 
  [bigint](../datatypes/bigints.md) can represent large integer numbers, 
  and [`Fractions`](../datatypes/fractions.md) store values in terms of a numerator and 
  denominator.

- `numberFallback`. When `number` is configured for example with value `'bigint'`,
  and a value cannot be represented as `bigint` like in `math.evaluate('2.3')`, 
  the value will be parsed in the type configured with `numberFallback`. 
  Available values: `'number'` (default) or `'BigNumber'`.

- `precision`. The maximum number of significant digits for BigNumbers.
  This setting only applies to BigNumbers, not to numbers.
  Default value is `64`.

- `predictable`. Predictable output type of functions. When true, output type
  depends only on the input types. When false (default), output type can vary
  depending on input values. For example `math.sqrt(-4)` returns `complex('2i')` when
  predictable is false, and returns `NaN` when true.
  Predictable output can be needed when programmatically handling the result of
  a calculation, but can be inconvenient for users when evaluating dynamic
  equations.

- `randomSeed`. Set this option to seed pseudo random number generation, making it deterministic. The pseudo random number generator is reset with the seed provided each time this option is set. For example, setting it to `'a'` will cause `math.random()` to return `0.43449421599986604` upon the first call after setting the option every time. Set to `null` to seed the pseudo random number generator with a random seed. Default value is `null`.

- `legacySubset`. When set to `true`, the `subset` function behaves as in earlier versions of math.js: retrieving a subset where the index size contains only one element, returns the value itself. When set to `false` (default), `subset` eliminates dimensions from the result only if the index dimension is a scalar; if the index dimension is a range, matrix, or array, the dimensions are preserved. This option is helpful for maintaining compatibility with legacy code that depends on the previous behavior.


## Configuration: Examples

This section shows a number of configuration examples.

### node.js

```js
import { create, all } from 'mathjs'

const config = {
  matrix: 'Array' // Choose 'Matrix' (default) or 'Array'
}
const math = create(all, config)

// range will output an Array
math.range(0, 4) // Array [0, 1, 2, 3]

// change the configuration from Arrays to Matrices
math.config({
  matrix: 'Matrix' // Choose 'Matrix' (default) or 'Array'
})

// range will output a Matrix
math.range(0, 4) // Matrix [0, 1, 2, 3]

// create an instance of math.js with BigNumber configuration
const bigmath = create(all, {
  number: 'BigNumber', // Choose 'number' (default), 'BigNumber', or 'Fraction'
  precision: 32        // 64 by default, only applicable for BigNumbers
})

// parser will parse numbers as BigNumber now:
bigmath.evaluate('1 / 3') // BigNumber, 0.33333333333333333333333333333333
```

### browser


```html
<!DOCTYPE HTML>
<html>
<head>
  
</head>
<body>
  
</body>
</html>
```


## Extension

The library can easily be extended with functions and variables using the
[`import`](../reference/functions/import.md) function. The `import` function is available on a mathjs instance, which can be created using the `create` function.

```js
import { create, all } from 'mathjs'

const math = create(all)

math.import(/* ... */)
```

The function `import` accepts an object with functions and variables, or an array with factory functions. It has the following syntax:

```js
math.import(functions: Object [, options: Object])
```

Where:

- `functions` is an object or array containing the functions and/or values to be
  imported. `import` support regular values and functions, typed functions
  (see section [Typed functions](#typed-functions)), and factory functions
  (see section [Factory functions](#factory-functions)).
  An array is only applicable when it contains factory functions.

- `options` is an optional second argument with options.
  The following options are available:

    - `{boolean} override`
      If `true`, existing functions will be overwritten. The default value is `false`.
    - `{boolean} silent`
      If `true`, the function will not throw errors on duplicates or invalid
      types. Default value is `false`.
    - `{boolean} wrap`
      If `true`, the functions will be wrapped in a wrapper function which
      converts data types like Matrix to primitive data types like Array.
      The wrapper is needed when extending math.js with libraries which do not
      support the math.js data types. The default value is `false`.

The following code example shows how to import a function and a value into math.js:

```js
// define new functions and variables
math.import({
  myvalue: 42,
  hello: function (name) {
    return 'hello, ' + name + '!'
  }
})

// defined functions can be used in both JavaScript as well as the parser
math.myvalue * 2                 // 84
math.hello('user')               // 'hello, user!'

const parser = math.parser()
parser.evaluate('myvalue + 10')  // 52
parser.evaluate('hello("user")') // 'hello, user!'
```

## Extension: Import external libraries

External libraries like
[numbers.js](https://github.com/sjkaliski/numbers.js) and
[numeric.js](https://github.com/sloisel/numeric) can be imported as follows.
Not for this drive: The libraries must be installed using npm:

Not for this drive:     $ npm install numbers
Not for this drive:     $ npm install numeric

The libraries can be easily imported into math.js using `import`.
In order to convert math.js specific data types like `Matrix` to primitive types
like `Array`, the imported functions can be wrapped by enabling `{wrap: true}`.

```js
import { create, all } from 'mathjs'
import * as numbers from 'numbers'
import * as numeric from 'numeric'

// create a mathjs instance and import the numbers.js and numeric.js libraries
const math = create(all)
math.import(numbers, {wrap: true, silent: true})
math.import(numeric, {wrap: true, silent: true})

// use functions from numbers.js
math.fibonacci(7)                           // 13
math.evaluate('fibonacci(7)')               // 13

// use functions from numeric.js
math.evaluate('eig([1, 2; 4, 3])').lambda.x // [5, -1]
```


## Extension: Typed functions

Typed functions can be created using `math.typed`. A typed function is a function
which does type checking on the input arguments. It can have multiple signatures.
And can automatically convert input types where needed.

A typed function can be created like:

```js
const max = typed('max', {
  'number, number': function (a, b) {
    return Math.max(a, b)
  },

  'BigNumber, BigNumber': function (a, b) {
    return a.greaterThan(b) ? a : b
  }
})
```

Typed functions can be merged as long as there are no conflicts in the signatures.
This allows for extending existing functions in math.js with support for new
data types.

```js
// create a new data type
function MyType (value) {
  this.value = value
}
MyType.prototype.isMyType = true
MyType.prototype.toString = function () {
  return 'MyType:' + this.value
}

// define a new datatype
math.typed.addType({
  name: 'MyType',
  test: function (x) {
    // test whether x is of type MyType
    return x && x.isMyType
  }
})

// use the type in a new typed function
const add = typed('add', {
  'MyType, MyType': function (a, b) {
    return new MyType(a.value + b.value)
  }
})

// import in math.js, extend the existing function `add` with support for MyType
math.import({add: add})

// use the new type
const ans = math.add(new MyType(2), new MyType(3)) // returns MyType(5)
console.log(ans)                                 // outputs 'MyType:5'
```

Detailed information on typed functions is available here:
[https://github.com/josdejong/typed-function](https://github.com/josdejong/typed-function)




## Extension: Factory functions

Regular JavaScript functions can be imported in math.js using `math.import`:

```js
math.import({
  myFunction: function (a, b) {
     // ...
  }
})
```

The function can be stored in a separate file:

```js
export function myFunction (a, b) {
  // ...
}
```

Which can be imported like:

```js
import { myFunction } from './myFunction.js'

math.import({
  myFunction
})
```

An issue arises when `myFunction` needs functionality from math.js:
it doesn't have access to the current instance of math.js when in a separate file.
Factory functions can be used to solve this issue. A factory function allows to inject dependencies into a function when creating it.

A syntax of factory function is:

```js
factory(name: string, dependencies: string[], create: function, meta?: Object): function
```

where:

-   `name` is the name of the created function.
-   `dependencies` is an array with names of the dependent functions.
-   `create` is a function which creates the function.
    An object with the dependencies is passed as first argument.
-   `meta` An optional object which can contain any meta data you want.
    This will be attached as a property `meta` on the created function.
    Known meta data properties used by the mathjs instance are:
    -   `isClass: boolean`  If true, the created function is supposed to be a
        class, and for example will not be exposed in the expression parser
        for security reasons.
    -   `lazy: boolean`.  By default, everything is imported lazily by `import`.
        only as soon as the imported function or constant is actually used, it
        will be constructed. A function can be forced to be created immediately
        by setting `lazy: false` in the meta data.
    -   `isTransformFunction: boolean`. If true, the created function is imported
        as a transform function. It will not be imported in `math` itself, only
        in the internal `mathWithTransform` namespace that is used by the
        expression parser.
    -   `recreateOnConfigChange: boolean`. If true, the imported factory will be
        created again when there is a change in the configuration. This is for
        example used for the constants like `pi`, which is different depending
        on the configsetting `number` which can be numbers or BigNumbers.
    -   `formerly: string`. If present, the created function will also be
        accessible on the instance under the name given by the value of
        `formerly` as a (deprecated) synonym for the specified `name`. This
        facility should only be used when a function is renamed, to allow
        temporary use of the previous name, for backward compatibility.

Here an example of a factory function which depends on `multiply`:

```js
import { factory, create, all } from 'mathjs'

// create a factory function
const name = 'negativeSquare'
const dependencies = ['multiply', 'unaryMinus']
const createNegativeSquare = factory(name, dependencies, function ({ multiply, unaryMinus }) {
    return function negativeSquare (x) {
      return unaryMinus(multiply(x, x))
    }
  })

// create an instance of the function yourself:
const multiply = (a, b) => a * b
const unaryMinus = (a) => -a
const negativeSquare = createNegativeSquare({ multiply, unaryMinus })
console.log(negativeSquare(3)) // -9

// or import the factory in a mathjs instance and use it there
const math = create(all)
math.import(createNegativeSquare)
console.log(math.negativeSquare(4)) // -16
console.log(math.evaluate('negativeSquare(5)')) // -25
```

You may wonder why you would inject functions `multiply` and `unaryMinus`
instead of just doing these calculations inside the function itself. The
reason is that this makes the factory function `negativeSquare` work for
different implementations: numbers, BigNumbers, units, etc.

```js
import { Decimal } from 'decimal.js'

// create an instance of our negativeSquare supporting BigNumbers instead of numbers
const multiply = (a, b) => a.mul(b)
const unaryMinus = (a) => new Decimal(0).minus(a)
const negativeSquare = createNegativeSquare({ multiply, unaryMinus })
```

## Core

## Core: Usage

The core of math.js is the `math` namespace containing all functions and constants. There are three ways to do calculations in math.js:

- Doing regular function calls like `math.add(math.sqrt(4), 2)`.
- Evaluating expressions like `math.evaluate('sqrt(4) + 2')`
- Chaining operations like `math.chain(4).sqrt().add(2)`.

## Core: Configuration

math.js can be configured using the `math.config()`, see page [Configuration](configuration.md).

## Core: Extension

math.js can be extended with new functions and constants using the function `math.import()`, see page [Extension](extension.md).

## Core: Serialization

To persist or exchange data structures like matrices and units, the data types of math.js can be stringified as JSON. This is explained on the page [Serialization](serialization.md).


## Serialization

Math.js has a number of data types like `Matrix`, `Complex`, and `Unit`. These
types are instantiated JavaScript objects. To be able to store these data types
or send them between processes, they must be serialized. The data types of
math.js can be serialized to JSON. Use cases:

- Store data in a database or on disk.
- Interchange of data between a server and a client.
- Interchange of data between a web worker and the browser.

Math.js types can be serialized using JavaScript's built-in `JSON.stringify`
function:

```js
const x = math.complex('2 + 3i')
const str = JSON.stringify(x, math.replacer)
console.log(str)
// outputs a string '{"mathjs":"Complex","re":2,"im":3}'
```

> IMPORTANT: in most cases works, serialization correctly without
> passing the `math.replacer` function as second argument. This is because
> in most cases we can rely on the default behavior of JSON.stringify, which 
> uses the `.toJSON` method on classes like `Unit` and `Complex` to correctly 
> serialize them. However, there are a few special cases like the 
> number `Infinity` which does require the replacer function in order to be 
> serialized without losing information: without it, `Infinity` will be 
> serialized as `"null"` and cannot be deserialized correctly.
>
> So, it's best to always pass the `math.replacer` function to prevent 
> weird edge cases.

In order to deserialize a string, containing math.js data types, `JSON.parse`
can be used. In order to recognize the data types of math.js, `JSON.parse` must
be called with the reviver function of math.js:

```js
const json = '{"mathjs":"Unit","value":5,"unit":"cm","fixPrefix":false}'
const x = JSON.parse(json, math.reviver)   // Unit 5 cm
```

Note that if math.js is used in conjunction with other data types, it is
possible to use multiple reviver functions at the same time by cascading them:

```js
const reviver = function (key, value) {
  return reviver1(key, reviver2(key, value))
}
```


## Custom bundling

Math.js is a large library containing many data types and functions.
It is well possible that you only need a small portion of the library.
Math.js allows for picking just the functions and data types you need.
This gives faster load times and smaller browser bundles. Math.js uses
ES modules, and creating small bundles using tree-shaking works out of
the box when using Webpack for example.

This page describes:

- How to use just a few functions for faster load times and smaller bundles.
- How to use light-weight, number only implementations of functions.
- What to expect from bundle sizes when using tree-shaking.

## Custom bundling: Using just a few functions

Using the function `create`, a mathjs instance can be created.
The `all` object contains all functionality available in mathjs,
and a mathjs instance containing everything can be created like:

```js
import { create, all } from 'mathjs'

const math = create(all)
```

To create an instance with just a few functions, you have to pass the
factory functions of the functions you need, and all their dependencies.
For example the function `add` depends on the functions `addScalar`,
`equalScalar`, classes `DenseMatrix` and `SparseMatrix`, and more.
Because it is hard to figure out what the dependencies of a function are,
and the dependencies of the dependencies, mathjs provides ready made
collections of all dependencies for every function. For example all
factory functions of function `add` and its dependencies are available
as `addDependencies`.

Here is a full example of loading just a few functions in a mathjs instance:

```js
// file: custom_loading.js

import {
  create,
  fractionDependencies,
  addDependencies,
  divideDependencies,
  formatDependencies
} from 'mathjs'

const config = {
  // optionally, you can specify configuration
}

// Create just the functions we need
const { fraction, add, divide, format } = create({
  fractionDependencies,
  addDependencies,
  divideDependencies,
  formatDependencies
}, config)

// Use the created functions
const a = fraction(1, 3)
const b = fraction(3, 7)
const c = add(a, b)
const d = divide(a, b)
console.log('c =', format(c)) // outputs "c = 16/21"
console.log('d =', format(d)) // outputs "d = 7/9"
```

This example can be bundled using for example Webpack:

```not-for-this-stick
npx webpack custom_loading.js -o custom_loading.bundle.js --mode=production
```

Only the used parts of mathjs will be bundled thanks to tree-shaking.


## Custom bundling: Numbers only

The functions of mathjs support multiple data types out of the box, like
numbers, bignumbers, complex numbers, units, and matrices. Quite commonly however,
only support for numbers is needed and the other data-types are overkill.

To accomodate for this use case where just the JavaScript `number` type
suffices, mathjs offers light-weight,
number-only implementations of all relevant functions. These are available by
importing from `'mathjs/number'` instead of `'mathjs'`:

```js
// use light-weight, number-only implementations of functions
import { create, all } from 'mathjs/number'

const math = create(all)
console.log(math.add(2, 3)) // 5
```

## Custom bundling: Bundle size

When using just a few functions of mathjs instead of the whole library,
you may expect the size of the bundle to be just a small fraction of the
complete library. However, to create the function `add` supporting all data
types, all these data types must be included: Unit, BigNumber, Complex,
DenseMatrix, SparseMatrix, etc. A rough idea of the size of different parts of
mathjs:

- About 5% is coming from core functionality like `create`, `import`, `factory`,
  `typed-function`, etc.
- About 30% of the bundle size comes from the data classes `Complex`, `BigNumber`, `Fraction`, `Unit`, `SparseMatrix`, `DenseMatrix`.
- About 25%  of the bundle size comes from the expression parser.
  Half of this comes from the embedded docs.
- About 40% comes from the about 200 built-in functions and some constants.

To get a better insight in what is in your JavaScript bundle, you can use
a tool like [source-map-explorer](https://github.com/danvk/source-map-explorer).


## BigInts

For calculations with large integer numbers, math.js supports the built-in `bigint` data type.

## BigInts: Usage

A bigint can be created either by adding the suffix `n` to a `number`, using the `BigInt` constructor function, or using the util function `math.bigint`:

```js
42n
BigInt('42')
math.bigint('42')
```

Most functions can determine the type of output from the type of input:
a `number` as input will return a `number` as output, a `bigint` as input returns
a `bigint` as output. Functions which cannot determine the type of output
from the input (for example `math.evaluate`) use the default number type `number`,
which can be configured when instantiating math.js. To configure the use of
`bigint` instead of [numbers](numbers.md) by default, configure math.js like:

```js
math.config({
  number: 'bigint'
})

// use math
math.evaluate('70000000000000000123')  // bigint 70000000000000000123n
```

## BigInts: Support

All basic arithmetic functions in math.js support `bigint`. Since `bigint` can only hold integer values, it is not applicable to for example trigonometric functions. When using a `bigint` in a function that does not support it, like `sqrt`, it will convert the `bigint` into a regular `number` and then execute the function:

```js
math.sin(2n) // number 0.9092974268256817
```

## BigInts: Conversion

There are utility functions to convert a `bigint` into a `number` or `BigNumber`:

```js
// convert a number to bigint or BigNumber
math.bigint(42)                    // bigint, 42n
math.bignumber(42)                 // BigNumber, 42

// convert a bigint to a number or BigNumber
math.number(42n)                   // number, 42
math.bignumber(42n)                // BigNumber, 42

// losing digits when converting to number
math.number(70000000000000000123n) // number, 7e+19
```


## BigNumbers

For calculations with an arbitrary precision, math.js supports a `BigNumber`
datatype. BigNumber support is powered by
[decimal.js](https://github.com/MikeMcl/decimal.js/).

## BigNumbers: Usage

A BigNumber can be created using the function `bignumber`:

```js
math.bignumber('2.3e+500') // BigNumber, 2.3e+500
```

Most functions can determine the type of output from the type of input:
a number as input will return a number as output, a BigNumber as input returns
a BigNumber as output. Functions which cannot determine the type of output
from the input (for example `math.evaluate`) use the default number type `number`,
which can be configured when instantiating math.js. To configure the use of
BigNumbers instead of [numbers](numbers.md) by default, configure math.js like:

```js
math.config({
  number: 'BigNumber',      // Default type of number:
                            // 'number' (default), 'BigNumber', or 'Fraction'
  precision: 64,            // Number of significant digits for BigNumbers
  relTol: 1e-60,
  absTol: 1e-63
})

// use math
math.evaluate('0.1 + 0.2')  // BigNumber, 0.3
```

The default precision for BigNumber is 64 digits, and can be configured with
the option `precision`. 

Note that we also change the configuration of `relTol` and `absTol` 
to be close to the precision limit of our BigNumbers. `relTol` and `absTol` are used for 
example in relational and rounding functions (`equal`, `larger`, `smaller`, 
`round`, `floor`, etc) to determine when a value is nearly equal, 
see [Equality](numbers.md#equality). If we would leave `relTol` and `absTol`  unchanged, 
having the default value of `1e-12` and `1e-15` respectively, we could get inaccurate and misleading 
results since we're now working with a higher precision.


## BigNumbers: Support

Most functions in math.js support BigNumbers, but not all of them.
For example the function `random` doesn't support BigNumbers.


## BigNumbers: Round-off errors

Calculations with BigNumber are much slower than calculations with Number,
but they can be executed with an arbitrary precision. By using a higher
precision, it is less likely that round-off errors occur:

```js
// round-off errors with numbers
math.add(0.1, 0.2)                                     // Number, 0.30000000000000004
math.divide(0.3, 0.2)                                  // Number, 1.4999999999999998

// no round-off errors with BigNumbers :)
math.add(math.bignumber(0.1), math.bignumber(0.2))     // BigNumber, 0.3
math.divide(math.bignumber(0.3), math.bignumber(0.2))  // BigNumber, 1.5
```


## BigNumbers: Limitations

It's important to realize that BigNumbers do not solve *all* problems related
to precision and round-off errors. Numbers with an infinite number of digits
cannot be represented with a regular number nor a BigNumber. Though a BigNumber
can store a much larger number of digits, the amount of digits remains limited
if only to keep calculations fast enough to remain practical.

```js
const one = math.bignumber(1)
const three = math.bignumber(3)
const third = math.divide(one, three)
console.log(third.toString())
// outputs 0.3333333333333333333333333333333333333333333333333333333333333333

const ans = math.multiply(third, three)
console.log(ans.toString())
// outputs 0.9999999999999999999999999999999999999999999999999999999999999999
// this should be 1 again, but `third` is rounded to a limited number of digits 3
```


## BigNumbers: Conversion

BigNumbers can be converted to numbers and vice versa using the functions
`number` and `bignumber`. When converting a BigNumber to a number, the high
precision of the BigNumber will be lost. When a BigNumber is too large to be represented
as Number, it will be initialized as `Infinity`.

```js
// converting numbers and BigNumbers
const a = math.number(0.3)                         // number, 0.3
const b = math.bignumber(a)                        // BigNumber, 0.3
const c = math.number(b)                           // number, 0.3

// exceeding the maximum of a number
const d = math.bignumber('1.2e500')                // BigNumber, 1.2e+500
const e = math.number(d)                           // number, Infinity

// loosing precision when converting to number
const f = math.bignumber('0.2222222222222222222')  // BigNumber, 0.2222222222222222222
const g = math.number(f)                           // number,    0.2222222222222222
```


## Booleans

Math.js uses the JavaScript boolean values `true` and `false`. For scalar inputs,
relational functions such as `math.larger` and `math.equal` return booleans:

```js
math.typeOf(true)     // 'boolean'
math.larger(3, 2)     // true
math.equal(2, 3)      // false
```

## Booleans: Conversion

Use [`math.boolean`](../reference/functions/boolean.md) to convert a value.
For numbers, zero and `NaN` become `false`; other numbers become `true`.

```js
math.boolean(0)       // false
math.boolean(-3)      // true
```

For strings, `math.boolean` first compares the whole string with `true` and
`false`, ignoring case. Other nonempty strings are converted to a number first,
then to a boolean using the numeric rules above. An empty string or a string
that cannot be converted to a number throws an error.

```js
math.boolean('false')   // false
math.boolean('TRUE')    // true
math.boolean('0')       // false
math.boolean('')        // throws an error
math.boolean('yes')     // throws an error
```

**Note:** This string conversion differs from JavaScript's `Boolean('false')`,
which returns `true`.

Arrays and matrices are converted element by element, including mixed types:

```js
math.boolean([true, 'false', 0, '2']) // [true, false, false, true]
```

To convert booleans to other types, use [`math.number`](../reference/functions/number.md)
or [`math.string`](../reference/functions/string.md):

```js
math.number(true)     // 1
math.number(false)    // 0
math.string(false)    // 'false'
```

## Booleans: Logical operations

Use [`math.and`](../reference/functions/and.md),
[`math.or`](../reference/functions/or.md),
[`math.xor`](../reference/functions/xor.md), and
[`math.not`](../reference/functions/not.md) for logical operations.
The expression parser uses the operators `and`, `or`, `xor`, and `not`:

```js
math.and(true, false)              // false
math.or(true, false)               // true
math.xor(true, true)               // false
math.not(false)                   // true
math.evaluate('true and false')    // false
math.evaluate('true or false')     // true
math.evaluate('true xor true')     // false
math.evaluate('not false')         // true
```


## Complex Numbers

Math.js supports the creation, manipulation, and calculations with complex numbers.
Support of complex numbers is powered by the library [complex.js](https://github.com/infusion/Complex.js).

In mathematics, a complex number is an expression of the form `a + bi`,
where `a` and `b` are real numbers and `i` represents the imaginary number
defined as `i^2 = -1`. (In other words, `i` is the square root of `-1`.)
The real number `a` is called the real part of the complex number,
and the real number `b` is the imaginary part. For example, `3 + 2i` is a
complex number, having real part `3` and imaginary part `2`.
Complex numbers are often used in applied mathematics, control theory,
signal analysis, fluid dynamics and other fields.

## Complex Numbers: Usage

A complex number is created using the function `math.complex`. This function
accepts:

- two numbers representing the real and imaginary part of the value,
- a single string containing a complex value in the form `a + bi` where `a`
  and `b` respectively represent the real and imaginary part of the complex number.
- an object with either properties `re` and `im` for the real and imaginary
  part of the value, or two properties `r` and `phi` containing the polar
  coordinates of a complex value.
The function returns a `Complex` object.

Syntax:

```js
math.complex(re: number) : Complex
math.complex(re: number, im: number) : Complex
math.complex(complex: Complex) : Complex
math.complex({re: Number, im: Number}) : Complex
math.complex({r: number, phi: number}) : Complex
math.complex({abs: number, arg: number}) : Complex
math.complex(str: string) : Complex
```

Examples:

```js
const a = math.complex(2, 3)     // Complex 2 + 3i
a.re                             // Number 2
a.im                             // Number 3

const b = math.complex('4 - 2i') // Complex 4 - 2i
b.re = 5                         // Number 5
b                                // Complex 5 - 2i
```

## Complex Numbers: Calculations

Most functions of math.js support complex numbers. Complex and real numbers
can be used together.

```js
const a = math.complex(2, 3)     // Complex 2 + 3i
const b = math.complex('4 - 2i') // Complex 4 - 2i

math.re(a)                       // Number 2
math.im(a)                       // Number 3
math.conj(a)                     // Complex 2 - 3i

math.add(a, b)                   // Complex 6 + i
math.multiply(a, 2)              // Complex 4 + 6i
math.sqrt(-4)                    // Complex 2i
```

## Complex Numbers: API
A `Complex` object created by `math.complex` contains the following properties and functions:

### complex.re

A number containing the real part of the complex number. Can be read and replaced.

### complex.im

A number containing the imaginary part of the complex number. Can be read and replaced.

### complex.clone()

Create a clone of the complex number.

### complex.equals(other)

Test whether a complex number equals another complex value.

  Two complex numbers are equal when both their real and imaginary parts are
  equal.

### complex.neg()

Returns a complex number with a real part and an imaginary part equal in magnitude but opposite in sign to the current complex number.

### complex.conjugate()

Returns a complex number with an equal real part and an imaginary part equal in magnitude but opposite in sign to the current complex number.

### complex.inverse()

Returns a complex number that is inverse of the current complex number.

### complex.toVector()

Get the vector representation of the current complex number. Returns an array of size 2.

### complex.toJSON()

Returns a JSON representation of the complex number, with signature
  `{mathjs: 'Complex', re: number, im: number}`.
  Used when serializing a complex number, see [Serialization](../core/serialization.md).

### complex.toPolar()

Get the polar coordinates of the complex number, returns
  an object with properties `r` and `phi`.

### complex.toString()

Returns a string representation of the complex number, formatted
  as `a + bi` where `a` is the real part and `b` the imaginary part.


### complex.format([precision: number])

Get a string representation of the complex number,
  formatted as `a + bi` where `a` is the real part and `b` the imaginary part.
  If precision is defined, the units value will be rounded to the provided
  number of digits.

## Complex Numbers: Static methods
The following static methods can be accessed using `math.Complex`


### Complex.fromJSON(json)

Revive a complex number from a JSON object. Accepts
  An object `{mathjs: 'Complex', re: number, im: number}`, where the property
  `mathjs` is optional.
  Used when deserializing a complex number, see [Serialization](../core/serialization.md).

### Complex.fromPolar(r: number, phi: number)

Create a complex number from polar coordinates.


### Complex.compare(a: Complex, b: Complex)

Returns the comparision result of two complex number:

- Returns 1 when the real part of `a` is larger than the real part of `b`
- Returns -1 when the real part of `a` is smaller than the real part of `b`
- Returns 1 when the real parts are equal
  and the imaginary part of `a` is larger than the imaginary part of `b`
- Returns -1 when the real parts are equal
  and the imaginary part of `a` is smaller than the imaginary part of `b`
- Returns 0 when both real and imaginary parts are equal.

Example:
```js
const a = math.complex(2, 3)   // Complex 2 + 3i
const b = math.complex(2, 1)   // Complex 2 + 1i
math.Complex.compare(a,b) // returns 1

//create from json 
const c = math.Complex.fromJSON({mathjs: 'Complex', re: 4, im: 3})  // Complex 4 + 3i
```


## Fractions

For calculations with fractions, math.js supports a `Fraction` data type. 
Fraction support is powered by [fraction.js](https://github.com/rawify/Fraction.js).
Unlike [numbers](numbers.md) and [BigNumbers](./bignumbers.md), fractions can 
store numbers with infinitely repeating decimals, for example `1/3 = 0.3333333...`, 
which can be represented as `0.(3)`, or `2/7` which can be represented as `0.(285714)`.


## Fractions: Usage

A Fraction can be created using the function `fraction`:

```js
math.fraction('1/3')   // Fraction, 1/3
math.fraction(2, 3)    // Fraction, 2/3
math.fraction('0.(3)') // Fraction, 1/3
```

And can be used in functions like `add` and `multiply` like:

```js
math.add(math.fraction('1/3'), math.fraction('1/6'))      // Fraction, 1/2
math.multiply(math.fraction('1/4'), math.fraction('1/2')) // Fraction, 1/8
```

Note that not all functions support fractions. For example trigonometric 
functions doesn't support fractions. When not supported, the functions
will convert the input to numbers and return a number as result.

Most functions will determine the type of output from the type of input:
a number as input will return a number as output, a Fraction as input returns
a Fraction as output. Functions which cannot determine the type of output
from the input (for example `math.evaluate`) use the default number type `number`,
which can be configured when instantiating math.js. To configure the use of
fractions instead of [numbers](numbers.md) by default, configure math.js like:

```js
// Configure the default type of number: 'number' (default), 'BigNumber', or 'Fraction'
math.config({
  number: 'Fraction'
})

// use the expression parser
math.evaluate('0.32 + 0.08') // Fraction, 2/5
```

## Fractions: Support

The following functions support fractions:

- Arithmetic functions: `abs`, `add`, `ceil`, `cube`, `divide`, `dotDivide`, `dotMultiply`, `fix`, `floor`, `gcd`, `mod`, `multiply`, `round`, `sign`, `square`, `subtract`, `unaryMinus`, `unaryPlus`.
- Construction functions: `fraction`.
- Relational functions: `compare`, `deepEqual`, `equal`, `larger`, `largerEq`, `smaller`, `smallerEq`, `unequal`.
- Utils functions: `format`.


## Fractions: Conversion

Fractions can be converted to numbers and vice versa using the functions
`number` and `fraction`. When converting a Fraction to a number, precision
may be lost when the value cannot represented in 16 digits. 

```js
// converting numbers and fractions
const a = math.number(0.3)                       // number, 0.3
const b = math.fraction(a)                       // Fraction, 3/10
const c = math.number(b)                         // number, 0.3

// loosing precision when converting to number: a fraction can represent
// a number with an infinite number of repeating decimals, a number just
// stores about 16 digits and cuts consecutive digits.
const d = math.fraction('2/5')                   // Fraction, 2/5
const e = math.number(d)                         // number, 0.4
```


## Data Types

The functions of math.js support multiple data types, both native JavaScript
types as well as more advanced types implemented in math.js. The data types can
be mixed together in calculations, for example by adding a Number to a
Complex number or Array.

The supported data types are:

- [Boolean](booleans.md)
- [Number](numbers.md)
- [BigNumber](bignumbers.md)
- [Complex](complex_numbers.md)
- [Fraction](fractions.md)
- [Array](matrices.md)
- [Matrix](matrices.md)
- [Unit](units.md)
- [String](strings.md)

Function [`math.typeOf(x)`](../reference/functions/typeOf.md) can be used to get
the type of a variable.

Example usage:

```js
// use numbers
math.subtract(7.1, 2.3)          // 4.8
math.round(math.pi, 3)           // 3.142
math.sqrt(4.41e2)                // 21

// use BigNumbers
math.add(math.bignumber(0.1), math.bignumber(0.2)) // BigNumber, 0.3

// use bigint
math.add(300000000000000000n, 1n) // 300000000000000001n

// use Fractions
math.add(math.fraction(1), math.fraction(3)) // Fraction, 0.(3)

// use strings containing numbers
math.add('2', '3')               // 5
math.concat('2', '3')            // '23'
math.max('3', '2', '7')          // 7
math.compareNatural('10', '2')   // 1

// use complex numbers
const a = math.complex(2, 3)     // 2 + 3i
a.re                             // 2
a.im                             // 3
const b = math.complex('4 - 2i') // 4 - 2i
math.add(a, b)                   // 6 + i
math.sqrt(-4)                    // 2i

// use arrays
const array = [1, 2, 3, 4, 5]
math.factorial(array)            // Array,  [1, 2, 6, 24, 120]
math.add(array, 3)               // Array,  [3, 5, 6, 7, 8]

// use matrices
const matrix = math.matrix([1, 4, 9, 16, 25]) // Matrix, [1, 4, 9, 16, 25]
math.sqrt(matrix)                             // Matrix, [1, 2, 3, 4, 5]

// use units
const a = math.unit(55, 'cm')    // 550 mm
const b = math.unit('0.1m')      // 100 mm
math.add(a, b)                   // 0.65 m

// check the type of a variable
math.typeOf(2)                   // 'number'
math.typeOf(math.unit('2 inch')) // 'Unit'
math.typeOf(math.sqrt(-4))       // 'Complex'
```


## Matrices

Math.js supports multidimensional matrices and arrays. Matrices can be
created, manipulated, and used in calculations. Both regular JavaScript
arrays and the matrix type implemented by math.js can be used
interchangeably in all relevant math.js functions. math.js supports both
dense and sparse matrices.


## Matrices: Arrays and matrices

Math.js supports two types of matrices:

- `Array`, a regular JavaScript array. A multidimensional array can be created
  by nesting arrays. The following terminology will be used to define different kinds of nested arrays.
  - **Rectangular array**: All elements of an array are arrays of the same size. Like `[[1, 2], [3, 4]]`
  - **Jagged arrays**: All elements of an array are arrays but not of the same size. Like `[[1, 2], [3, 4, 5]]`
  - **Heterogeneous arrays**: If not all the elements inside an array are arrays. Like `[[1, 2], 3]`.
- `Matrix`, a matrix implementation by math.js. A `Matrix` is an object wrapped
  around a regular JavaScript `Array`, providing utility functions for easy
  matrix manipulation such as `subset`, `size`, `resize`, `clone`, and more. Nested arrays must be rectangular to be converted to a `Matrix`.

In most cases, the type of matrix output from functions is determined by the
function input: An `Array` as input will return an `Array`, a `Matrix` as input
will return a `Matrix`. In case of mixed input, a `Matrix` is returned.
For functions where the type of output cannot be determined from the
input, the output is determined by the configuration option `matrix`,
which can be a string `'Matrix'` (default) or `'Array'`. The function `size` is
an exception: `size` always returns an `Array` containing numbers. Having a 
consistent output type in this case is most practical since the size is often
used in JavaScript loops where the code can only use a flat Array with numbers.
This also makes the function `size` consistent with the matrix method 
`matrix.size()`.

```js
// create an array and a matrix
const array = [[2, 0], [-1, 3]]               // Array
const matrix = math.matrix([[7, 1], [-2, 3]]) // Matrix

// perform a calculation on an array and matrix
math.map(array, math.square)                  // Array,  [[4, 0], [1, 9]]
math.map(matrix, math.square)                 // Matrix, [[49, 1], [4, 9]]

// perform calculations with mixed array and matrix input
math.add(array, matrix)                       // Matrix, [[9, 1], [-3, 6]]
math.multiply(array, matrix)                  // Matrix, [[14, 2], [-13, 8]]

// create a matrix. Type of output of function ones is determined by the
// configuration option `matrix`
math.ones(2, 3)                               // Matrix, [[1, 1, 1], [1, 1, 1]]
```


## Matrices: Creation

A matrix can be created from an array using the function `math.matrix`. The
provided array can contain nested arrays in order to create a multidimensional matrix. When called without arguments, an empty matrix will be
created.

```js
// create matrices
math.matrix()                           // Matrix, size [0]
math.matrix([0, 1, 2])                  // Matrix, size [3]
math.matrix([[0, 1], [2, 3], [4, 5]])   // Matrix, size [3, 2]
```

Math.js supports regular Arrays. Multiple dimensions can be created
by nesting Arrays in each other.

```js
// create arrays
[]                                      // Array, size [0]
[0, 1, 2]                               // Array, size [3]
[[0, 1], [2, 3], [4, 5]]                // Array, size [3, 2]
```

Matrices can contain different types of values: numbers, complex numbers,
units, or strings. Different types can be mixed together in a single matrix.

```js
// create a matrix with mixed types
const a = math.matrix([2.3, 'hello', math.complex(3, -4), math.unit('5.2 mm')]) 
a.subset(math.index(1))  // 'hello'
```


There are a number of functions to create a matrix with a specific size and
content: `ones`, `zeros`, `identity`.

```js
// zeros creates a matrix filled with zeros
math.zeros(3)           // Matrix, size [3],    [0, 0, 0]
math.zeros(3, 2)        // Matrix, size [3, 2], [[0, 0], [0, 0], [0, 0]]
math.zeros(2, 2, 2)     // Matrix, size [2, 2, 2],
                        //   [[[0, 0], [0, 0]], [[0, 0], [0, 0]]]

// ones creates a matrix filled with ones
math.ones(3)                        // Matrix, size [3],    [1, 1, 1]
math.multiply(math.ones(2, 2), 5)   // Matrix, size [2, 2], [[5, 5], [5, 5]]

// identity creates an identity matrix
math.identity(3)      // Matrix, size [3, 3], [[1, 0, 0], [0, 1, 0], [0, 0, 1]]
math.identity(2, 3)   // Matrix, size [2, 3], [[1, 0, 0], [0, 1, 0]]
```


The functions `ones`, `zeros`, and `identity` also accept a single array
or matrix containing the dimensions for the matrix. When the input is an Array,
the functions will output an Array. When the input is a Matrix, the output will
be a Matrix. Note that in case of numbers as arguments, the output is
determined by the option `matrix` as discussed in section
[Arrays and matrices](#arrays-and-matrices).

```js
// Array as input gives Array as output
math.ones([2, 3])               // Array,  size [2, 3], [[1, 1, 1], [1, 1, 1]]
math.ones(math.matrix([2, 3]))  // Matrix, size [2, 3], [[1, 1, 1], [1, 1, 1]]
```

(Note the contrast with the function `size` described below, which always
returns an Array, even when its argument is a Matrix.)

Ranges can be created using the function `range`. The function `range` is
called with parameters start and end, and optionally a parameter step.
The start of the range is included, the end of the range is excluded.

```js
math.range(0, 4)        // [0, 1, 2, 3]
math.range(0, 8, 2)     // [0, 2, 4, 6]
math.range(3, -1, -1)   // [3, 2, 1, 0]
```


## Matrices: Calculations

Most functions of math.js support matrices and arrays. Unary functions can be applied element-wise using via `math.map(matrix, function)`.

```js
// perform an element-wise operation on a matrix using math.map
const a = math.matrix([1, 4, 9, 16, 25])  // Matrix, [1, 4, 9, 16, 25]
math.map(a, math.sqrt)                    // Matrix, [1, 2, 3, 4, 5]

// use a function that has built-in matrix and array support
const b = [1, 2, 3, 4, 5] 
math.factorial(b)                         // Array,  [1, 2, 6, 24, 120]

// multiply an array with a matrix
const c = [[2, 0], [-1, 3]]               // Array
const d = math.matrix([[7, 1], [-2, 3]])  // Matrix
math.multiply(c, d)                       // Matrix, [[14, 2], [-13, 8]]

// add a number to a matrix (see broadcasting)
math.add(c, 2)                            // Array, [[4, 2], [1, 5]]

// calculate the determinant of a matrix
math.det(c)                               // 6
math.det(d)                               // 23
```
## Matrices: Broadcasting

Functions that require two or more matrix like arguments that operate elementwise automatically operate as if the arguments were the same size.

```js
A = math.matrix([1, 2])       // Matrix, [1, 2]
math.add(A, 3)                // Matrix, [4, 5]

B = math.matrix([[3], [4]])   // Matrix, [[3], [4]]
math.add(A, B)                // Matrix, [[4, 5], [5, 6]]
```
Any index that is in one of the arguments, can be found as if it existed on the others when the size on that dimension is one or not existing. This is valid in N dimensions.

It's not possible to broadcast in cases where the size in that dimension is higher than one.

```js
math.add([1, 2], [3, 4, 5])
// Error: shape missmatch: missmatch is found in arg with shape (2) not possible to broadcast dimension 0 with size 2 to size 3

math.add([[1], [2], [3]], [[4], [5]])
// Error: shape missmatch: missmatch is found in arg with shape (2,1) not possible to broadcast dimension 0 with size 2 to size 3
```

## Matrices: Size and Dimensions

Math.js uses geometric dimensions:

- A scalar is zero-dimensional.
- A vector is one-dimensional.
- A matrix is two or multidimensional.

The size of a matrix can be calculated with the function `size`. This function
returns an `Array`, giving the length of its input (`Matrix` or `Array`) in
each dimension. You can also call `size()` as a method on a Matrix.

```js
// get the size of a scalar
math.size(2.4)                                // Array, []
math.size(math.complex(3, 2))                 // Array, []
math.size(math.unit('5.3 mm'))                // Array, []

// get the size of a one-dimensional matrix (a vector) and a string
math.size([0, 1, 2, 3])                       // Array, [4]
math.size('hello world')                      // Array, [11]

// get the size of a two-dimensional matrix
const a = [[0, 1, 2, 3]]                      // Array
const b = math.matrix([[0, 1, 2], [3, 4, 5]]) // Matrix
math.size(a)                                  // Array, [1, 4]
math.size(b)                                  // Array, [2, 3]

// matrices have a method size
b.size()                                      // Array, [2, 3]

// get the size of a multi-dimensional array
const c = [[[0, 1, 2], [3, 4, 5]], [[6, 7, 8], [9, 10, 11]]]
math.size(c)                                  // Array, [2, 2, 3]
```

Note that the dimensions themselves do not have a meaning attached.
When creating and printing a two-dimensional matrix, the first dimension is
normally rendered as the _row_, and the second dimension is rendered as
the _column_. For example:

```js
console.table(math.zeros([2, 4]))
// 0 0 0 0
// 0 0 0 0
```

If you have a matrix where the first dimension means `x` and the second
means `y`, this will look confusing since `x` is printed as _column_
(vertically) and `y` as _row_ (horizontally).

## Matrices: Non-rectangular arrays

By nesting arrays it is possible to have arrays that are not rectangular, for example.
```js
[[1, 2], [3, 4]]    // rectangular of size [2, 2]
[[1, 2], [3, 4, 5]] // jagged
[[1, 2], 3]         // heterogeneous
```

Jagged and heterogeneous arrays can't be converted to a matrix, but many operations are available for them.
```js
const A = [[1, 2], 3]
math.add(A, 1)
math.map(A, a => a+1)
math.forEach(A, a => console.log(a))
```
Some matrix functions expect a rectangular array and might provide unexpected results with non rectangular arrays, for example.
```js
math.size([[1, 2], [3]]) // [2, 2]
```
The process of validation for rectangularity is expensive and is mandatory to create a `Matrix`, thus there might be a performance benefit of not converting an `Array` to a `Matrix`.

## Matrices: Resizing

Matrices can be resized using their `resize` function. This function is called
with an Array with the new size as the first argument, and accepts an optional
default value. By default, new entries will be set to `0`, but it is possible
to pass a different default value like `null` to clearly indicate that
the entries haven't been explicitly set.

```js
const a = math.matrix() // Matrix, size [0],       []
a.resize([2, 3])        // Matrix, size [2, 3],    [[0, 0, 0], [0, 0, 0]]
a.resize([2, 2, 2])     // Matrix, size [2, 2, 2],
                        //   [[[0, 0], [0, 0]], [[0, 0], [0, 0]]]

const b = math.matrix()
b.resize([3], 7)        // Matrix, size [3],    [7, 7, 7]
b.resize([5], 9)        // Matrix, size [5],    [7, 7, 7, 9, 9]
b.resize([2])           // Matrix, size [2],    [7, 7]
```


Outer dimensions of a matrix can be squeezed using the function `squeeze`. When
getting or setting a single value in a matrix using `subset`, the value is automatically squeezed
or unsqueezed too.

```js
// squeeze a matrix
const a = [[[0, 1, 2]]]
math.squeeze(a)             // [0, 1, 2]
math.squeeze([[3]])         // 3

// when getting/setting a single value in a matrix using subset, 
// it automatically squeeze/unsqueeze the value
const b = math.matrix([[0, 1], [2, 3]])
b.subset(math.index(1, 0))  // 2 and not [[2]]
```


## Matrices: Getting or replacing subsets

Subsets of a matrix can be retrieved or replaced using the function `subset`.
Matrices have a `subset` function, which is applied to the matrix itself:
`Matrix.subset(index [, replacement])`. For both matrices and arrays,
the static function `subset(matrix, index [, replacement])` can be used.
When parameter `replacement` is provided, the function will replace a subset
in the matrix, and if not, a subset of the matrix will be returned.

A subset can be defined using an `Index`. An `Index` contains a single value
or a set of values for each dimension of a matrix. An `Index` can be
created using the function `index`. The way `subset` returns results depends on how you specify indices for each dimension:

- If you use a scalar (single number) as an index for a dimension, that dimension is removed from the result.
- If you use an array, matrix or range (even with just one element) as an index, that dimension is preserved in the result.

This means that scalar indices eliminate dimensions, while array, matrix or range indices retain them. See the section [Migrate to v15](#migrate-indexing-behavior-to-mathjs-v15) for more details and examples of this behavior.

For example:

```js
const m = [
  [10, 11, 12],
  [20, 21, 22]
]

// Scalar index eliminates the dimension:
math.subset(m, math.index(1, 2))           // 22 (both dimensions indexed by scalars, result is a value)
math.subset(m, math.index(1, [2]))         // [22] (row dimension eliminated, column dimension preserved as array)
math.subset(m, math.index([1], 2))         // [22] (column dimension eliminated, row dimension preserved as array)
math.subset(m, math.index([1], [2]))       // [[22]] (both dimensions preserved as arrays)

math.config({legacySubset: true}) // switch to legacy behavior
math.subset(m, math.index(1, 2))           // 22
math.subset(m, math.index(1, [2]))         // 22
math.subset(m, math.index([1], 2))         // 22
math.subset(m, math.index([1], [2]))       // 22

```

Matrix indexes in math.js are zero-based, like most programming languages
including JavaScript itself. Note that mathematical applications like Matlab 
and Octave work differently, as they use one-based indexes.

```js
// create some matrices
const a = [0, 1, 2, 3]
const b = [[0, 1], [2, 3]]
const c = math.zeros(2, 2)
const d = math.matrix([[0, 1, 2], [3, 4, 5], [6, 7, 8]])
const e = math.matrix()

// get a subset
math.subset(a, math.index(1))                 // 1
math.subset(a, math.index([2, 3]))            // Array, [2, 3]
math.subset(a, math.index(math.range(0,4)))   // Array, [0, 1, 2, 3]
math.subset(b, math.index(1, 0))              // 2
math.subset(b, math.index(1, [0, 1]))         // Array, [2, 3]
math.subset(b, math.index([0, 1], [0]))       // Matrix, [[0], [2]]

// get a subset
d.subset(math.index([1, 2], [0, 1]))          // Matrix, [[3, 4], [6, 7]]
d.subset(math.index(1, 2))                    // 5

// replace a subset. The subset will be applied to a clone of the matrix
math.subset(b, math.index(1, 0), 9)           // Array, [[0, 1], [9, 3]]
math.subset(b, math.index(2, [0, 1]), [4, 5]) // Array, [[0, 1], [2, 3], [4, 5]]

// replace a subset. The subset will be applied to the matrix itself
c.subset(math.index(0, 1),1)                  // Matrix, [[0, 1], [0, 0]]
c.subset(math.index(1, [0, 1]), [2, 3])       // Matrix, [[0, 1], [2, 3]]
e.resize([2, 3], 0)                           // Matrix, [[0, 0, 0], [0, 0, 0]]
e.subset(math.index(1, 2), 5)                 // Matrix, [[0, 0, 0], [0, 0, 5]]
```

## Matrices: Migrate indexing behavior to mathjs v15

With the release of math.js v15, the behavior of `subset` when indexing matrices and arrays has changed. If your code relies on the previous behavior (where indexing with an array or matrix of size 1 would always return the value itself), you may need to update your code or enable legacy mode.

To maintain the old indexing behavior without need for any code changes, use the configuration option `legacySubset`:

```js
math.config({ legacySubset: true })
```

To migrate your code, you'll have to change all matrix indexes from the old index notation to the new index notation. Basically: scalar indexes have to be wrapped in array brackets if you want an array as output. Here some examples:

```js
const m = math.matrix([[1, 2, 3], [4, 5, 6]])
```

| v14 code                                     | v15 equivalent code                       | Result             |
|----------------------------------------------|-------------------------------------------|--------------------|
| `math.subset(m, math.index([0, 1], [1, 2]))` | No change needed                          | `[[2, 3], [5, 6]]` |
| `math.subset(m, math.index(1, [1, 2]))`      | `math.subset(m, math.index([1], [1, 2]))` | `[[5, 6]]`         |
| `math.subset(m, math.index([0, 1], 2))`      | `math.subset(m, math.index([0, 1], [2]))` | `[[3], [6]]`       |
| `math.subset(m, math.index(1, 2))`           | No change needed                          | `6`               |


> **Tip:**  
> If you want to get a scalar value, use scalar indices.  
> If you want to preserve dimensions, use array, matrix or range indices.

## Matrices: Getting and setting a value in a matrix

There are two methods available on matrices that allow to get or set a single 
value inside a matrix. It is important to note that the `set` method will 
mutate the matrix.

```js
const p = math.matrix([[1, 2], [3, 4]])
p.set([0, 1], 5)
// p is now [[1, 5], [3, 4]]
p.get([1, 0]) // 3
```

When setting a value at a location outside the current matrix size using the
method `.set()`, the matrix will be resized. By default, new items will be 
initialized with zero, but it is possible to specify an alternative value using
the optional third argument `defaultValue`.

## Matrices: Advanced Indexing

Boolean array indexing is a technique that allows you to filter, replace, and set values in an array based on logical conditions. This can be done by creating a boolean array that represents the desired conditions, and then using that array as an index to select the elements of the original array that meet those conditions.

For example, a boolean array can be created to represent all the even numbers in an array, and then used to filter the original array to only include the even numbers. Alternatively, a boolean array can be created to represent all the elements of an array that are greater than a certain value, and then used to replace all the elements of the original array that are greater than that value with a new value.


```js
const q = [1, 2, 3, 4]
math.subset(q, math.index([true, false, true, false]))      // Array [1, 3]

// filtering
math.subset(q, math.index(math.larger(q, 2)))               // Array [3, 4]

// filtering with no matches
math.subset(q, math.index(math.larger(q, 5)))               // Array []

// setting specific values, please note that the replacement value is broadcasted
q = math.subset(q, math.index(math.smaller(q, 3)), 0)       // q = [0, 0, 3, 4]

// replacing specific values
math.subset(q, math.index(math.equal(q, 0)), [1, 2])        // q = [1, 2, 3, 4]
```

The same can be accomplished in the parser in a much more compact manner. Please note that everything after `#` are comments.
```js
math.evaluate(`
q = [1, 2, 3, 4]
q[[true, false, true, false]] # Matrix [1, 3]
q[q>2]                        # Matrix [3, 4]
q[q>5]                        # Matrix []
q[q <3] = 0                   # q = [0, 0, 3, 4]
q[q==0] = [1, 2]              # q = [1, 2, 3, 4]
`)
```
The expression inside the index can be as complex as needed as long it evaluates to an array of booleans of the same size.
```js
math.evaluate(`
q = [1, 2, 3, 4]
r = [6, 5, 4, 3]
q[q > 3 and r < 4]     # [4]
`)
```

## Matrices: Iterating

Matrices contain functions `map` and `forEach` to iterate over all elements of
the (multidimensional) matrix. The callback function of `map` and `forEach` has
three parameters: `value` (the value of the currently iterated element),
`index` (an array with the index value for each dimension), and `matrix` (the
matrix being iterated). This syntax is similar to the `map` and `forEach`
functions of native JavaScript Arrays, except that the index is no number but
an Array with numbers for each dimension.

```js
const a = math.matrix([[0, 1], [2, 3], [4, 5]])

// The iteration below will output the following in the console:
//    value: 0 index: [0, 0]
//    value: 1 index: [0, 1]
//    value: 2 index: [1, 0]
//    value: 3 index: [1, 1]
//    value: 4 index: [2, 0]
//    value: 5 index: [2, 1]
a.forEach(function (value, index, matrix) {
  console.log('value:', value, 'index:', index) 
}) 

// Apply a transformation on the matrix
const b = a.map(function (value, index, matrix) {
  return math.multiply(math.sin(value), math.exp(math.abs(value))) 
}) 
console.log(b.format(5))  // [[0, 2.2874], [6.7188, 2.8345], [-41.32, -142.32]]

// Create a matrix with the cumulative of all elements
let count = 0
const cum = a.map(function (value, index, matrix) {
  count += value 
  return count 
}) 
console.log(cum.toString())  // [[0, 1], [3, 6], [10, 15]]
```

### Iterating over multiple Matrices or Arrays

You can iterate over multiple matrices or arrays by using the `map` function. Mapping allows to perform element-wise operations on matrices by automatically adjusting their sizes to match each other.

To iterate over multiple matrices, you can use the `map` function. The `map` function applies a given function to each element of the matrices and returns a new matrix with the results.

Here's an example of iterating over two matrices and adding their corresponding elements:

```js
const a = math.matrix([[1, 2], [3, 4]]);
const b = math.matrix([[5, 6], [7, 8]]);

const result = math.map(a, b, (x, y) => x + y);

console.log(result); // [[6, 8], [10, 12]]
```

In this example, the `map` function takes matrices as the first two arguments and a callback function `(x, y) => x + y` as the third argument. The callback function is applied to each element of the matrices, where `x` represents the corresponding element from matrix `a` and `y` represents the corresponding element from matrix `b`. The result is a new matrix with the element-wise sum of the two matrices.

By using broadcasting and the `map` function, you can easily iterate over multiple matrices and perform element-wise operations.

```js
const a = math.matrix([10, 20])
const b = math.matrix([[3, 4], [5, 6]])

const result = math.map(a, b, (x, y) => x + y)
console.log(result); // [[13, 24], [15, 26]]
```

It's also possible to provide a callback with an index and the broadcasted arrays. Like `(valueA, valueB, index)` or even `(valueA, valueB, index, broadcastedMatrixA, broadcastedMatrixB)`. There is no specific limit for the number of matrices `N` that can be mapped. Thus, the callback can have `N` arguments, `N+1` arguments in the case of including the index, or `2N+1` arguments in the case of including the index and the broadcasted matrices in the callback. 

At this moment `forEach` doesn't include the same functionality.

## Matrices: Storage types

Math.js supports both dense matrices and sparse matrices. Sparse matrices are efficient for matrices largely containing zeros. In that case they save a lot of memory, and calculations can be much faster than for dense matrices.

Math.js supports two type of matrices:

- Dense matrix (`'dense'`, `default`) A regular, dense matrix, supporting multidimensional matrices. This is the default matrix type.
- Sparse matrix (`'sparse'`): A two dimensional sparse matrix implementation.

The type of matrix can be selected when creating a matrix using the construction functions `matrix`, `diag`, `identity`, `ones`, and `zeros`.

```js
// create sparse matrices
const m1 = math.matrix([[0, 1], [0, 0]], 'sparse')
const m2 = math.identity(1000, 1000, 'sparse')
```

You can also coerce an array or matrix into sparse storage format with the
`sparse` function.
```js
const md = math.matrix([[0, 1], [0,0]])  // dense
const ms = math.sparse(md)               // sparse
```

Caution: `sparse` called on a JavaScript array of _n_ plain numbers produces
a matrix with one column and _n_ rows -- in contrast to `matrix`, which
produces a 1-dimensional matrix object with _n_ entries, i.e., a vector
(_not_ a 1 by _n_ "row vector" nor an _n_ by 1 "column vector", but just a plain
vector of length _n_).
```js
const mv = math.matrix([0, 0, 1])  // Has size [3]
const mc = math.sparse([0, 0, 1])  // A "column vector," has size [3, 1]
```

## Matrices: API

All relevant functions in math.js support Matrices and Arrays. Functions like `math.add` and `math.subtract`, `math.sqrt` handle matrices element wise. There is a set of functions specifically for creating or manipulating matrices, such as:

- Functions like `math.matrix` and `math.sparse`, `math.ones`, `math.zeros`, and `math.identity` to create a matrix.
- Functions like `math.subset` and `math.index` to get or replace a part of a matrix
- Functions like `math.transpose` and `math.diag` to manipulate matrices.

A full list of matrix functions is available on the [functions reference page](../reference/functions.md#matrix-functions).

Two types of matrix classes are available in math.js, for storage of dense and sparse matrices. Although they contain public functions documented as follows, using the following API directly is *not* recommended. Prefer using the functions in the "math" namespace wherever possible.

- [DenseMatrix](../reference/classes/densematrix.md)
- [SparseMatrix](../reference/classes/sparsematrix.md)


## Numbers

Math.js supports three types of numbers:

- Number for fast floating point arithmetic, described on this page.
- BigNumber for arbitrary precision arithmetic, described on the page
  [BigNumbers](bignumbers.md).
- Fraction, which stores numbers in terms of a numerator and denominators, 
  described on the page [Fractions](fractions.md).


## Numbers: Configuration

Most functions can determine the type of output from the type of input:
a number as input will return a number as output, a BigNumber as input returns
a BigNumber as output. Functions which cannot determine the type of output
from the input (for example `math.evaluate`) use the default number type, which
can be configured when instantiating math.js:

```js
math.config({
  number: 'number' // Default type of number: 
                   // 'number' (default), 'BigNumber', or 'Fraction'
})
```

## Numbers: Round-off errors

Math.js uses the built-in JavaScript Number type. A Number is a floating point
number with a limited precision of 64 bits, about 16 digits. The largest integer
number which can be represented by a JavaScript Number
is `+/- 9007199254740992` (`+/- 2^53`). Because of the limited precision of
floating point numbers round-off errors can occur during calculations.
This can be easily demonstrated:

```js
// a round-off error
0.1 + 0.2            // 0.30000000000000004
math.add(0.1, 0.2)   // 0.30000000000000004
```

In most cases, round-off errors don't matter: they have no significant
impact on the results. However, it looks ugly when displaying output to a user.
A solution is to limit the precision just below the actual precision of 16
digits in the displayed output:

```js
// prevent round-off errors showing up in output
const ans = math.add(0.1, 0.2)     //  0.30000000000000004
math.format(ans, {precision: 14})  // '0.3'
```

Alternatives are to use [Fractions](fractions.md) which store a number as a numerator and denominator, [BigNumbers](bignumbers.md) which store a number with a higher precision, or [bigint](bigints.md) which can store larger integer numbers.


## Numbers: Minimum and maximum

A Number can store values between `5e-324` and `1.7976931348623157e+308`.
Values smaller than the minimum are stored as `0`, and values larger than the
maximum are stored as `+/- Infinity`.

```js
// exceeding the maximum and minimum number
console.log(1e309)   // Infinity
console.log(1e-324)  // 0
```

## Numbers: Equality

Because of rounding errors in calculations, it is unsafe to compare JavaScript
Numbers. For example executing `0.1 + 0.2 == 0.3` in JavaScript will return
false, as the addition `0.1 + 0.2` introduces a round-off error and does not
return exactly `0.3`.

To solve this problem, the relational functions of math.js check whether the
relative and absolute differences between the compared values is smaller than the configured
option `relTol` and `absTol`. In pseudo code (without exceptions for 0, Infinity and NaN):

    abs(a-b) <= max(relTol * max(abs(a), abs(b)), absTol)

where:

 - `relTol` is the relative tolerance between x and y and `absTol` the absolute tolerance. Relative tolerance and absolute tolerance are configurable and are `1e-12` and `1e-15` respectively by default. See [Configuration](../core/configuration.md).
 - `DBL_EPSILON` is the minimum positive floating point number such that
   `1.0 + DBL_EPSILON !== 1.0`. This is a constant with a value of approximately
   `2.2204460492503130808472633361816e-16`.

Note that the relational functions cannot be used to compare small values
(`< 2.22e-16`). These values are all considered equal to zero.

Examples:

```js
// compare values having a round-off error
console.log(0.1 + 0.2 === 0.3)           // false
console.log(math.equal(0.1 + 0.2, 0.3))  // true

// small values (< 2.22e-16) cannot be compared
console.log(3e-20 === 3.1e-20)           // false
console.log(math.equal(3e-20, 3.1e-20))  // true
```

The available relational functions are: `compare`, `equal`, `larger`,
`largerEq`, `smaller`, `smallerEq`, `unequal`.


## Strings

Math.js uses JavaScript strings, written with single or double quotes. Use
[`math.string`](../reference/functions/string.md) to convert a value to a string.
Arrays and matrices are converted element by element.

```js
math.typeOf('hello')       // 'string'
math.string(2.3)           // '2.3'
math.string([2, true])     // ['2', 'true']
```

## Strings: Numeric conversion

Numeric functions such as `math.add` and `math.max` attempt to convert string
arguments to numbers. Conversion fails if a string cannot be parsed as a number.
For example, `math.add('hello ', 'world')` throws an error.

```js
math.add('2', '3')         // 5
math.max('3', '2', '7')    // 7
math.number('2.3')         // 2.3
```

Functions with string signatures can handle strings directly. In particular,
use [`math.concat`](../reference/functions/concat.md) to concatenate text:

```js
math.concat('hello ', 'world') // 'hello world'
math.concat('2', '3')          // '23'
```

The expression parser uses `+` for arithmetic, including numeric conversion of
strings. JavaScript's `+` operator concatenates strings instead:

```js
'2' + '3'                                  // '23'
math.evaluate('"2" + "3"')                 // 5
math.evaluate('concat("hello ", "world")')  // 'hello world'
```

## Strings: Other operations

Some functions listed under Matrix also accept strings:
[`math.count`](../reference/functions/count.md),
[`math.size`](../reference/functions/size.md),
[`math.subset`](../reference/functions/subset.md), and
[`math.resize`](../reference/functions/resize.md).
The JavaScript API uses zero-based indices in `math.index`; the expression
parser uses one-based indices.

```js
math.count('hello')                      // 5
math.size('hello')                       // [5]
math.subset('hello', math.index(1))       // 'e'
math.evaluate('"hello"[2]')               // 'e'
math.resize('hello', [8], '!')            // 'hello!!!'
```

## Strings: Comparing and formatting strings

[`math.compareText`](../reference/functions/compareText.md) compares text
lexicographically. [`math.compareNatural`](../reference/functions/compareNatural.md)
uses natural ordering, comparing numeric parts by their value:

```js
math.compareText('10', '2')     // -1
math.compareNatural('10', '2')  // 1
math.compareText('file2part10', 'file2part2')     // -1
math.compareNatural('file2part10', 'file2part2')  // 1
```

Use [`math.format`](../reference/functions/format.md) to format values, or
[`math.print`](../reference/functions/print.md) to interpolate values into a
string template. See the [String functions](../reference/functions.md#string-functions)
reference for more functions.


## Units

Math.js supports units. Units can be used to do calculations and to perform
conversions.

## Units: Usage

Units can be created using the function `math.unit`. This function accepts
either a single string argument containing a value and unit, or two arguments,
the first being a numeric value and the second a string containing a unit.
Most units support prefixes like `k` or `kilo`, and many units have both a
full name and an abbreviation. The returned object is a `Unit`.

Syntax:

```js
math.unit(value: number, valuelessUnit: string) : Unit
math.unit(value: number, valuelessUnit: Unit) : Unit
math.unit(unit: string) : Unit
math.unit(unit: Unit) : Unit
```

Example usage:

```js
const a = math.unit(45, 'cm')             // Unit 45 cm
const b = math.unit('0.1 kilogram')       // Unit 0.1 kilogram
const c = math.unit('2 inch')             // Unit 2 inch
const d = math.unit('90 km/h')            // Unit 90 km/h
const e = math.unit('101325 kg/(m s^2)')  // Unit 101325 kg / (m s^2)

const f = c.to('cm')                      // Unit 5.08 cm
b.toNumber('gram')                        // Number 100
math.number(b, 'gram')                    // Number 100

c.equals(a)                               // false
c.equals(f)                               // true
c.equalBase(a)                            // true
c.equalBase(b)                            // false

f.toString()                              // String "5.08 cm"

const kph = math.unit('km/h')             // valueless Unit km/h
const mps = math.unit('m/s')              // valueless Unit m/s
const speed = math.unit(36, kph)          // Unit 36 km/h
speed.toNumber(mps)                       // Number 10
```

Use care when creating a unit with multiple terms in the denominator. Implicit multiplication has the same operator precedence as explicit multiplication and division, which means these three expressions are identical:

```js
// These three are identical
const correct1 = math.unit('8.314 m^3 Pa / mol / K')           // Unit 8.314 (m^3 Pa) / (mol K)
const correct2 = math.unit('8.314 (m^3 Pa) / (mol K)')         // Unit 8.314 (m^3 Pa) / (mol K)
const correct3 = math.unit('8.314 (m^3 * Pa) / (mol * K)')     // Unit 8.314 (m^3 Pa) / (mol K)
```
But this expression, which omits the second `/` between `mol` and `K`, results in the wrong value:

```js
// Missing the second '/' between 'mol' and 'K'
const incorrect = math.unit('8.314 m^3 Pa / mol K')            // Unit 8.314 (m^3 Pa K) / mol
```

The function `math.unit` has its own small parser. This parser differs a bit from the expression parser `math.evaluate`, and returns the expected result in this case:

```js
// using math.evaluate instead of math.unit
const correct4 = math.evaluate('8.314 (m^3 * Pa) / (mol * K)') // Unit 8.314 (m^3 Pa) / (mol K)
```

In summary: be careful with implicit multiplication. In case of doubt, always use an explicit `*` and parenthesis.

The unit parser accepts parentheses for grouping, but exponents must follow an individual unit, not a parenthesized group. For example, `math.unit('(m/s)^2')` throws a syntax error. Apply the exponent to each unit instead, or use the expression parser:

```js
math.unit('m^2/s^2')       // Unit m^2 / s^2
math.evaluate('(m/s)^2')   // Unit m^2 / s^2
```

In math.js expressions, as parsed by `math.evaluate` and related methods,
`min` and `sec` resolve by default to the minimum and secant functions, not the
minute and second units. Use `minute` or `second` with `math.evaluate`, or use
`math.unit('5 min')` and `math.unit('5 sec')` to parse those abbreviations. A
scope can explicitly override these symbols.


## Units: Calculations

The operations that support units are `add`, `subtract`, `multiply`, `divide`, `pow`, `abs`, `sqrt`, `square`, `cube`, and `sign`.
Trigonometric functions like `cos` are also supported when the argument is an angle.

```js
const a = math.unit(45, 'cm')       // Unit 45 cm
const b = math.unit('0.1m')         // Unit 0.1 m
math.add(a, b)                      // Unit 55 cm
math.multiply(b, 2)                 // Unit 0.2 m

const c = math.unit(45, 'deg')      // Unit 45 deg
math.cos(c)                         // Number 0.7071067811865476

// Kinetic energy of average sedan on highway
const d = math.unit('80 mi/h')      // Unit 80 mi/h
const e = math.unit('2 tonne')      // Unit 2 tonne
const f = math.multiply(0.5, math.multiply(math.pow(d, 2), e))
                                    // 1.2790064742399996 MJ
```

Operations with arrays are supported too:

```js
// Force on a charged particle moving through a magnetic field
const B = math.evaluate('[1, 0, 0] T')         // [1 T, 0 T, 0 T]
const v = math.evaluate('[0, 1, 0] m/s')       // [0 m / s, 1 m / s, 0 m / s]
const q = math.evaluate('1 C')                 // 1 C

const F = math.multiply(q, math.cross(v, B))   // [0 N, 0 N, -1 N]
```

All arithmetic operators act on the value of the unit as it is represented in SI units.
This may lead to surprising behavior when working with temperature scales like `celsius` (or `degC`) and `fahrenheit` (or `degF`).
In general, you should avoid calculations using `celsius` and `fahrenheit`. Rather, use `kelvin` (or `K`) and `rankine` (or `degR`) instead.
This example highlights some problems when using `celsius` and `fahrenheit` in calculations:

```js
const T_14F = math.unit('14 degF')            // Unit 14 degF (263.15 K)
const T_28F = math.multiply(T_14F, 2)         // Unit 28 degF (270.93 K), not 526.3 K

const Tnegative = math.unit(-13, 'degF')      // Unit -13 degF (248.15 K)
const Tpositive = math.abs(Tnegative)         // Unit -13 degF (248.15 K), not 13 degF
```

The expression parser supports units too. This is described in the section about
units on the page [Syntax](../expressions/syntax.md#units).

## Units: User-Defined Units

You can add your own units to Math.js using the `math.createUnit` function. The following example defines a new unit `furlong`, then uses the user-defined unit in a calculation:

```js
math.createUnit('furlong', '220 yards')
math.evaluate('1 mile to furlong')            // 8 furlong
```

If you cannot express the new unit in terms of any existing unit, then the second argument can be omitted. In this case, a new *base unit* is created:

```js
// A 'foo' cannot be expressed in terms of any other unit.
math.createUnit('foo')
math.evaluate('8 foo * 4 feet')               // 32 foo feet
```

The second argument to `createUnit` can also be a configuration object consisting of the following properties:

* **definition** A `string` or `Unit` which defines the user-defined unit in terms of existing built-in or user-defined units. If omitted, a new base unit is created.
* **prefixes** A `string` indicating which prefixes math.js should use with the new unit. Possible values are `'none'`, `'short'`, `'long'`, `'binary_short'`, or `'binary_long'`. Default is `'none'`.
* **offset** A value applied when converting to the unit. This is very helpful for temperature scales that do not share a zero with the absolute temperature scale. For example, if we were defining fahrenheit for the first time, we would use: `math.createUnit('fahrenheit', {definition: '0.555556 kelvin', offset: 459.67})`
* **aliases** An array of strings to alias the new unit. Example: `math.createUnit('knot', {definition: '0.514444 m/s', aliases: ['knots', 'kt', 'kts']})`
* **baseName** A `string` that specifies the name of the new dimension in case one needs to be created. Every unit in math.js has a dimension: length, time, velocity, etc. If the unit's `definition` doesn't match any existing dimension, or it is a new base unit, then `createUnit` will create a new dimension with the name `baseName` and assign it to the new unit. The default is to append `'_STUFF'` to the unit's name. If the unit already matches an existing dimension, this option has no effect.

An optional `options` object can also be supplied as the last argument to `createUnits`. Currently only the `override` option is supported:

```js
// Redefine the mile (would not be the first time in history)
math.createUnit('mile', '1609.347218694 m', {override: true})
```
Base units created without specifying a definition cannot be overridden.

### Create several units at once
Multiple units can defined using a single call to `createUnit` by passing an object map as the first argument, where each key in the object is the name of a new unit and the value is either a string defining the unit, or an object with the configuration properties listed above. If the value is an empty string or an object lacking a definition property, a new base unit is created.

For example:

```js
math.createUnit( {
  foo: {
    prefixes: 'long',
    baseName: 'essence-of-foo'
  },
  bar: '40 foo',
  baz: {
    definition: '1 bar/hour',
    prefixes: 'long'
  }
},
{
  override: true
})
math.evaluate('50000 kilofoo/s')  // 4.5 gigabaz
```

### Return Value
`createUnit` returns the created unit, or, when multiple units are created, the last unit created. Since `createUnit` is also compatible with the expression parser, this allows you to do things like this:

```js
math.evaluate('45 mile/hour to createUnit("knot", "0.514444m/s")')
// 39.103964668651976 knot
```

### Support of custom characters in unit names
Per default, the name of a new unit:
- should start by a latin (A-Z or a-z) character
- should contain only numeric (0-9) or latin characters

It is possible to allow the usage of special characters (such as Greek alphabet, cyrillic alphabet, any Unicode symbols, etc.) by overriding the `Unit.isValidAlpha` static method. For example:
```js
const isAlphaOriginal = math.Unit.isValidAlpha
const isGreekLowercaseChar = function (c) {
  const charCode = c.charCodeAt(0)
  return charCode > 944 && charCode < 970
}
math.Unit.isValidAlpha = function (c) {
  return isAlphaOriginal(c) || isGreekLowercaseChar(c)
}

math.createUnit('θ', '1 rad')
math.evaluate('1θ + 3 deg').toNumber('deg') // 60.29577951308232
```

## Units: Numeric type of the value of a unit

The built-in units are always created with a value being a `number`. To turn the value into for example a `BigNumber` or `Fraction`, you can convert the value using the function `math.fraction` and `math.bignumber`:

```js
math.unit(math.fraction(10), 'inch').toNumeric('cm')  // Fraction 127/5
math.fraction(math.unit(10, 'inch')).toNumeric('cm')  // Fraction 127/5

math.bignumber(math.unit(10, 'inch')).toNumeric('cm') // BigNumber 25.4
math.unit(math.bignumber(10), 'inch').toNumeric('cm') // BigNumber 25.4
```

When using the expression parser, it is possible to configure numeric values to be parsed as `Fraction` or `BigNumber`:

```js
math.config({ number: 'Fraction' })
math.evaluate('10 inch').toNumeric('cm') // Fraction 127/5
```


## Units: API
A `Unit` object contains the following functions:

### unit.clone()
Clone the unit, returns a new unit with the same parameters.

### unit.equalBase(unit)
Test whether a unit has the same base as an other unit:
length, mass, etc.

### unit.equals(unit)
Test whether a unit equals an other unit. Units are equal
when they have the same base and same value when normalized to SI units.

### unit.format([options])
Get a string representation of the unit. The function
will determine the best fitting prefix for the unit. See the [Format](../reference/functions/format.md)
page for available options.

### unit.toBest(unitList, options)
Converts a unit to the most appropriate display unit by choosing 
from the list of passed units (unitList) or doing it automatically if no list is passed.
It also accepts options. At the moment, the only available one is offset which is used for a better prefix calculation

### unit.fromJSON(json)
Revive a unit from a JSON object. Accepts
An object `{mathjs: 'Unit', value: number, unit: string, fixPrefix: boolean}`,
where the property `mathjs` and `fixPrefix` are optional.
Used when deserializing a unit, see [Serialization](../core/serialization.md).

### unit.splitUnit(parts)
Split a unit into the specified parts. For example:

```js
const u = math.unit(1, 'm')
u.splitUnit(['ft', 'in'])    // 3 feet,3.3700787401574765 inch
```

### unit.to(unitName)
Convert the unit to a specific unit name. Returns a clone of
the unit with a fixed prefix and unit.

### unit.toJSON()
Returns a JSON representation of the unit, with signature
`{mathjs: 'Unit', value: number, unit: string, fixPrefix: boolean}`.
Used when serializing a unit, see [Serialization](../core/serialization.md).

### unit.toNumber(unitName)
Get the value of a unit when converted to the
specified unit (a unit with optional prefix but without value).
The type of the returned value is always `number`.

### unit.toNumeric(unitName)
Get the value of a unit when converted to the
specified unit (a unit with optional prefix but without value).
The type of the returned value depends on how the unit was created and
can be `number`, `Fraction`, or `BigNumber`.

### unit.toSI()
Returns a clone of a unit represented in SI units. Works with units with or without a value.

### unit.toString()
Get a string representation of the unit. The function will
determine the best fitting prefix for the unit.

### unit.valType()
Get the string name of the current type of the value of this Unit object, e.g.
'number', 'BigNumber', etc.

## Units: Unit reference

This section lists all available units, prefixes, and physical constants. These can be used via the Unit object, or via `math.evaluate()`.

## Units: Reference

Math.js comes with the following built-in units.

Base                | Unit
------------------- | ---
Length              | meter (m), inch (in), foot (ft), yard (yd), mile (mi), link (li), rod (rd), chain (ch), angstrom, mil
Surface area        | m2, sqin, sqft, sqyd, sqmi, sqrd, sqch, sqmil, acre, hectare
Volume              | m3, litre (l, L, lt, liter), cc, cuin, cuft, cuyd, teaspoon, tablespoon
Liquid volume       | minim, fluiddram (fldr), fluidounce (floz), gill (gi), cup (cp), pint (pt), quart (qt), gallon (gal), beerbarrel (bbl), oilbarrel (obl), hogshead, drop (gtt)
Angles              | rad (radian), deg (degree), grad (gradian), cycle, arcsec (arcsecond), arcmin (arcminute)
Time                | second (s, sec, secs, seconds), minute (min, mins, minutes), hour (h, hr, hrs, hours), day (days), week (weeks), month (months), year (years), decade (decades), century (centuries), millennium (millennia)
Frequency           | hertz (Hz)
Mass                | gram(g), tonne, ton, grain (gr), dram (dr), ounce (oz), poundmass (lbm, lb, lbs), hundredweight (cwt), stick, stone
Electric current    | ampere (A)
Temperature         | kelvin (K), celsius (degC), fahrenheit (degF), rankine (degR)
Amount of substance | mole (mol)
Luminous intensity  | candela (cd)
Force               | newton (N), dyne (dyn), poundforce (lbf), kip
Energy              | joule (J), erg, Wh, BTU, electronvolt (eV)
Power               | watt (W), hp
Pressure            | Pa, psi, atm, torr, bar, mmHg, mmH2O, cmH2O
Electricity and magnetism | ampere (A), ampere-hour (Ah), coulomb (C), watt (W), volt (V), ohm, farad (F), weber (Wb), tesla (T), henry (H), siemens (S), electronvolt (eV)
Binary              | bits (b), bytes (B)

In math.js expressions, the `min` and `sec` abbreviations above refer to
functions by default; use `minute`, `second`, or `s` instead. See the discussion
of expression parsing above.

Note: all time units are based on the Julian year, with one month being 1/12th of a Julian year, a year being one Julian year, a decade being 10 Julian years, a century being 100, and a millennium being 1000.

Note that all relevant units can also be written in plural form, for example `5 meters` instead of `5 meter` or `10 seconds` instead of `10 second`.

Surface and volume units can alternatively be expressed in terms of length units raised to a power, for example `100 in^2` instead of `100 sqin`.

### Prefixes

The following decimal prefixes are available.

Name    | Abbreviation  | Value
------- | ------------- | -----
deca    | da            | 1e1
hecto   | h             | 1e2
kilo    | k             | 1e3
mega    | M             | 1e6
giga    | G             | 1e9
tera    | T             | 1e12
peta    | P             | 1e15
exa     | E             | 1e18
zetta   | Z             | 1e21
yotta   | Y             | 1e24
ronna   | R             | 1e27
quetta  | Q             | 1e30

Name    | Abbreviation  | Value
------  | ------------- | -----
deci    | d             | 1e-1
centi   | c             | 1e-2
milli   | m             | 1e-3
micro   | u             | 1e-6
nano    | n             | 1e-9
pico    | p             | 1e-12
femto   | f             | 1e-15
atto    | a             | 1e-18
zepto   | z             | 1e-21
yocto   | y             | 1e-24
ronto   | r             | 1e-27
quecto  | q             | 1e-30

The following binary prefixes are available.
They can be used with units `bits` (`b`) and `bytes` (`B`).

Name | Abbreviation | Value
---- | ------------ | -----
kibi | Ki           | 1024
mebi | Mi           | 1024^2
gibi | Gi           | 1024^3
tebi | Ti           | 1024^4
pebi | Pi           | 1024^5
exi  | Ei           | 1024^6
zebi | Zi           | 1024^7
yobi | Yi           | 1024^8

Name  | Abbreviation | Value
----- | ------------ | -----
kilo  | k            | 1e3
mega  | M            | 1e6
giga  | G            | 1e9
tera  | T            | 1e12
peta  | P            | 1e15
exa   | E            | 1e18
zetta | Z            | 1e21
yotta | Y            | 1e24


### Physical Constants

Math.js includes the following physical constants. See [Wikipedia](https://en.wikipedia.org/wiki/Physical_constants) for more information.


#### Universal constants

Name                  | Symbol                                                 | Value             | Unit
----------------------|--------------------------------------------------------|-------------------|-------------------------------------------------------
speedOfLight          | <i>c</i>                                               | 299792458         | m &#183; s<sup>-1</sup>
gravitationConstant   | <i>G</i>                                               | 6.67430e-11     | m<sup>3</sup> &#183; kg<sup>-1</sup> &#183; s<sup>-2</sup>
planckConstant        | <i>h</i>                                               | 6.62607015e-34   | J &#183; s
reducedPlanckConstant | <i><span style="text-decoration:overline">h</span></i> | 1.0545718176461565e-34 | J &#183; s


#### Electromagnetic constants

Name                      | Symbol                                           | Value                 | Unit
--------------------------|--------------------------------------------------|-----------------------|----------------------------------------
magneticConstant          | <i>&mu;<sub>0</sub></i>                          | 1.25663706212e-6       | N &#183; A<sup>-2</sup>
electricConstant          | <i>&epsilon;<sub>0</sub></i>                     | 8.8541878128e-12       | F &#183; m<sup>-1</sup>
vacuumImpedance           | <i>Z<sub>0</sub></i>                             | 376.730313667         | &ohm;
coulomb                   | <i>&kappa;</i>                                   | 8.987551792261171e9  | N &#183; m<sup>2</sup> &#183; C<sup>-2</sup>
elementaryCharge          | <i>e</i>                                         | 1.602176634e-19     | C
bohrMagneton              | <i>&mu;<sub>B</sub></i>                          | 9.2740100783e-24      | J &#183; T<sup>-1</sup>
conductanceQuantum        | <i>G<sub>0</sub></i>                             | 7.748091729863649e-5     | S
inverseConductanceQuantum | <i>G<sub>0</sub><sup>-1</sup></i>                | 12906.403729652257       | &ohm;
magneticFluxQuantum       | <i><font face="Symbol">f</font><sub>0</sub></i>  | 2.0678338484619295e-15     | Wb
nuclearMagneton           | <i>&mu;<sub>N</sub></i>                          | 5.0507837461e-27      | J &#183; T<sup>-1</sup>
klitzing                  | <i>R<sub>K</sub></i>                             | 25812.807459304513       | &ohm;




#### Atomic and nuclear constants

Name                    | Symbol                | Value                 | Unit
------------------------|------------------------------|-----------------------|----------------------------------
bohrRadius              | <i>a<sub>0</sub></i>         | 5.29177210903e-11    | m
classicalElectronRadius | <i>r<sub>e</sub></i>         | 2.8179403262e-15    | m
electronMass            | <i>m<sub>e</sub></i>         | 9.1093837015e-31      | kg
fermiCoupling           | <i>G<sub>F</sub></i>         | 1.1663787e-5          | GeV<sup>-2</sup>
fineStructure           | <i>&alpha;</i>               | 7.2973525693e-3     | -
hartreeEnergy           | <i>E<abbr>h</abbr> </i>      | 4.3597447222071e-18      | J
protonMass              | <i>m<sub>p</sub></i>         | 1.67262192369e-27     | kg
deuteronMass            | <i>m<sub>d</sub></i>         | 3.3435830926e-27      | kg
neutronMass             | <i>m<sub>n</sub></i>         | 1.6749271613e-27      | kg
quantumOfCirculation    | <i>h / (2m<sub>e</sub>)</i>  | 3.6369475516e-4     | m<sup>2</sup> &#183; s<sup>-1</sup>
rydberg                 | <i>R<sub>&infin;</sub></i>   | 10973731.568160     | m<sup>-1</sup>
thomsonCrossSection     |                              | 6.6524587321e-29     | m<sup>2</sup>
weakMixingAngle         |                              | 0.22290              | -
efimovFactor            |                              | 22.7                  | -


#### Physico-chemical constants

Name                | Symbol                       | Value               | Unit
--------------------|------------------------------|---------------------|--------------------------------------------
atomicMass          | <i>m<sub>u</sub></i>         | 1.66053906660e-27   | kg
avogadro            | <i>N<sub>A</sub></i>         | 6.02214076e23     | mol<sup>-1</sup>
boltzmann           | <i>k</i>                     | 1.380649e-23     | J &#183; K<sup>-1</sup>
faraday             | <i>F</i>                     | 96485.33212331001        | C &#183; mol<sup>-1</sup>
firstRadiation      | <i>c<sub>1</sub></i>         | 3.7417718521927573e-16    | W &#183; m<sup>2</sup>
loschmidt           | <i>n<sub>0</sub></i>         | 2.686780111798444e25      | m<sup>-3</sup>
gasConstant         | <i>R</i>                     | 8.31446261815324         | J &#183; K<sup>-1</sup> &#183; mol<sup>-1</sup>
molarPlanckConstant | <i>N<sub>A</sub> &#183; h</i>| 3.990312712893431e-10| J &#183; s &#183; mol<sup>-1</sup>
molarVolume         | <i>V<sub>m</sub></i>         | 0.022413969545014137     | m<sup>3</sup> &#183; mol<sup>-1</sup>
sackurTetrode       |                              | -1.16487052358        | -
secondRadiation     | <i>c<sub>2</sub></i>         | 0.014387768775039337      | m &#183; K
stefanBoltzmann     | <i>&sigma;</i>               | 5.67037441918443e-8       | W &#183; m<sup>-2</sup> &#183; K<sup>-4</sup>
wienDisplacement    | <i>b</i>                     | 2.897771955e-3      | m &#183; K



Note that the values of `loschmidt` and `molarVolume` are at `T = 273.15 K` and `p = 101.325 kPa`.
The value of `sackurTetrode` is at `T = 1 K` and `p = 101.325 kPa`.


#### Adopted values

Name          | Symbol                       | Value   | Unit
--------------|------------------------------|---------|-------------------------
molarMass     | <i>M<sub>u</sub></i>         | 0.99999999965e-3    | kg &#183; mol<sup>-1</sup>
molarMassC12  | <i>M(<sub>12</sub>C)</i>     | 11.9999999958e-3  | kg &#183; mol<sup>-1</sup>
gravity       | <i>g<sub>n</sub></i>         | 9.80665 | m &#183; s<sup>-2</sup>
atm           | <i>atm</i>                   | 101325  | Pa


#### Natural units

Name              | Symbol                | Value              | Unit
------------------|-----------------------|--------------------|-----
planckLength      | <i>l<sub>P</sub></i>  | 1.616255e-35     | m
planckMass        | <i>m<sub>P</sub></i>  | 2.176435e-8       | kg
planckTime        | <i>t<sub>P</sub></i>  | 5.391245e-44      | s
planckCharge      | <i>q<sub>P</sub></i>  | 1.87554603778e-18  | C
planckTemperature | <i>T<sub>P</sub></i>  | 1.416785e+32     | K


## Algebra (symbolic computation)

math.js has built-in support for symbolic computation ([CAS](https://www.wikiwand.com/en/Computer_algebra_system)). It can parse expressions into an expression tree and do algebraic operations like simplification and derivation on the tree.

> It's worth mentioning an excellent extension on math.js here: [mathsteps](https://github.com/socraticorg/mathsteps), a step-by-step math solver library that is focused on pedagogy (how best to teach). The math problems it focuses on are pre-algebra and algebra problems involving simplifying expressions.


## Algebra (symbolic computation): Simplify

The function [`math.simplify`](../reference/functions/simplify.md) simplifies an expression tree:

```js
// simplify an expression
console.log(math.simplify('3 + 2 / 4').toString())              // '7 / 2'
console.log(math.simplify('2x + 3x').toString())                // '5 * x'
console.log(math.simplify('x^2 + x + 3 + x^2').toString())      // '2 * x ^ 2 + x + 3'
console.log(math.simplify('x * y * -x / (x ^ 2)').toString())   // '-y'
```

The function accepts either a string or an expression tree (`Node`) as input, and outputs a simplified expression tree (`Node`). This node tree can be transformed and evaluated as described in detail on the page [Expression trees](expression_trees.md).

```js
// work with an expression tree, evaluate results
const f = math.parse('2x + x')
const simplified = math.simplify(f)
console.log(simplified.toString())       // '3 * x'
console.log(simplified.evaluate({x: 4})) // 12
```

Among its other actions, calling `simplify()` on an expression will convert
any functions that have operator equivalents to their operator form:
```js
console.log(math.simplify('multiply(x,3)').toString())  // '3 * x'
```

Note that `simplify` has an optional argument `scope` that allows the definitions of variables in the expression (as numeric values, or as further expressions) to be specified and used in the simplification, e.g. continuing the previous example,

```js
console.log(math.simplify(f, {x: 4}).toString()) // 12
console.log(math.simplify(f, {x: math.parse('y+z')}).toString()) // '3*(y+z)'
```

In general, simplification is an inherently dfficult problem; in fact, for certain classes of expressions and algebraic equivalences, it is undecidable whether a given expression is equivalent to zero. Moreover, simplification generally depends on the properties of the operations involved; since multiplication (for example) may have different properties (e.g., it might or might not be commutative) depending on the domain under consideration, different simplifications might be appropriate.

As a result, `simplify()` has an additional optional argument, `options`, which controls its behavior. This argument is an object specifying any of various properties concerning the simplification process. See the [detailed documentation](../reference/functions/simplify.md) for a complete list, but currently the two most important properties are as follows. Note that the `options` argument may only be specified if the `scope` is as well.

- `exactFractions` - a boolean which specifies whether non-integer numerical constants should be simplified to rational numbers when possible (true), or always converted to decimal notation (false).
- `context` - an object whose keys are the names of operations ('add', 'multiply', etc.) and whose values specify algebraic properties of the corresponding operation (currently any of 'total', 'trivial', 'commutative', and 'associative'). Simplifications will only be performed if the properties they rely on are true in the given context. For example,
```js
const expr = math.parse('x*y-y*x')
console.log(math.simplify(expr).toString())  // 0; * is commutative by default
console.log(math.simplify(expr, {}, {context: {multiply: {commutative: false}}}))
  // 'x*y-y*x'; the order of the right multiplication can't be reversed.
```

Note that the default context is very permissive (allows a lot of simplifications) but that there is also a `math.simplify.realContext` that only allows simplifications that are guaranteed to preserve the value of the expression on all real numbers:
```js
const rational = math.parse('(x-1)*x/(x-1)')
console.log(math.simplify(expr, {}, {context: math.simplify.realContext})
  // '(x-1)*x/(x-1)'; canceling the 'x-1' makes the expression defined at 1
```

For more details on the theory of expression simplification, see:

- [Strategies for simplifying math expressions (Stackoverflow)](https://stackoverflow.com/questions/7540227/strategies-for-simplifying-math-expressions)
- [Symbolic computation - Simplification (Wikipedia)](https://en.wikipedia.org/wiki/Symbolic_computation#Simplification)


## Algebra (symbolic computation): Derivative

The function [`math.derivative`](../reference/functions/derivative.md) finds the symbolic derivative of an expression:

```js
// calculate a derivative
console.log(math.derivative('2x^2 + 3x + 4', 'x').toString())   // '4 * x + 3'
console.log(math.derivative('sin(2x)', 'x').toString())         // '2 * cos(2 x)'
```

Similar to the function `math.simplify`, `math.derivative` accepts either a string or an expression tree (`Node`) as input, and outputs a simplified expression tree (`Node`).

```js
// work with an expression tree, evaluate results
const h = math.parse('x^2 + x')
const x = math.parse('x')
const dh = math.derivative(h, x)
console.log(dh.toString())        // '2 * x + 1'
console.log(dh.evaluate({x: 3}))  // '7'
```

The rules used by `math.derivative` can be found on Wikipedia:

- [Differentiation rules (Wikipedia)](https://en.wikipedia.org/wiki/Differentiation_rules)


## Algebra (symbolic computation): Rationalize

The function [`math.rationalize`](../reference/functions/rationalize.md) transforms a rationalizable expression in a rational fraction.
If rational fraction is one variable polynomial then converts the numerator and denominator in canonical form, with decreasing exponents, returning the coefficients of numerator.

```js

math.rationalize('2x/y - y/(x+1)')
              // (2*x^2-y^2+2*x)/(x*y+y)
math.rationalize('(2x+1)^6')
              // 64*x^6+192*x^5+240*x^4+160*x^3+60*x^2+12*x+1
math.rationalize('2x/( (2x-1) / (3x+2) ) - 5x/ ( (3x+4) / (2x^2-5) ) + 3')
              // -20*x^4+28*x^3+104*x^2+6*x-12)/(6*x^2+5*x-4)

math.rationalize('x+x+x+y',{y:1}) // 3*x+1
math.rationalize('x+x+x+y',{})    // 3*x+y

const ret = math.rationalize('x+x+x+y',{},true)
              // ret.expression=3*x+y, ret.variables = ["x","y"]
const ret = math.rationalize('-2+5x^2',{},true)
              // ret.expression=5*x^2-2, ret.variables = ["x"], ret.coefficients=[-2,0,5]
```


## Customization

Besides parsing and evaluating expressions, the expression parser supports
a number of features to customize processing and evaluation of expressions
and outputting expressions.

On this page:

- [Function transforms](#function-transforms)
- [Custom argument parsing](#custom-argument-parsing)
- [Custom LaTeX handlers](#custom-latex-handlers)
- [Custom HTML, LaTeX and string output](#custom-html-latex-and-string-output)
- [Customize supported characters](#customize-supported-characters)

## Customization: Function transforms

It is possible to preprocess function arguments and post process a functions
return value by writing a *transform* for the function. A transform is a
function wrapping around a function to be transformed or completely replaces
a function.

For example, the functions for math.js use zero-based matrix indices (as is
common in programing languages), but the expression parser uses one-based
indices. To enable this, all functions dealing with indices have a transform,
which changes input from one-based to zero-based, and transforms output (and
error message) from zero-based to one-based.

```js
// using plain JavaScript, indices are zero-based:
const a = [[1, 2], [3, 4]]       // a 2x2 matrix
math.subset(a, math.index(0, 1)) // returns 2

// using the expression parser, indices are transformed to one-based:
const a = [[1, 2], [3, 4]] // a 2x2 matrix
let scope = {
  a: a
}
math.evaluate('subset(a, index(1, 2))', scope) // returns 2
```

To create a transform for a function, the transform function must be attached
to the function as property `transform`:

```js
import { create, all } from 'mathjs'
const math = create(all)

// create a function
function addIt(a, b) {
  return a + b
}

// attach a transform function to the function addIt
addIt.transform = function (a, b) {
  console.log('input: a=' + a + ', b=' + b)
  // we can manipulate input here before executing addIt

  const res = addIt(a, b)

  console.log('result: ' + res)
  // we can manipulate result here before returning

  return res
}

// import the function into math.js
math.import({
  addIt: addIt
})

// use the function via the expression parser
console.log('Using expression parser:')
console.log('2+4=' + math.evaluate('addIt(2, 4)'))
// This will output:
//
//     input: a=2, b=4
//     result: 6
//     2+4=6

// when used via plain JavaScript, the transform is not invoked
console.log('')
console.log('Using plain JavaScript:')
console.log('2+4=' + math.addIt(2, 4))
// This will output:
//
//     6
```

Functions with a transform must be imported in the `math` namespace, as they
need to be processed at compile time. They are not supported when passed via a
scope at evaluation time.


## Customization: Custom argument parsing

The expression parser of math.js has support for letting functions
parse and evaluate arguments themselves, instead of calling them with
evaluated arguments. This is useful for example when creating a function
like `plot(f(x), x)` or `integrate(f(x), x, start, end)`, where some of the
arguments need to be processed in a special way. In these cases, the expression
`f(x)` will be evaluated repeatedly by the function, and `x` is not evaluated
but used to specify the variable looping over the function `f(x)`.

Functions having a property `rawArgs` with value `true` are treated in a special
way by the expression parser: they will be invoked with unevaluated arguments,
allowing the function to process the arguments in a customized way. Raw
functions are called as:

```
rawFunction(args: Node[], math: Object, scope: Map)
```

Where :

- `args` is an Array with nodes of the parsed arguments.
- `math` is the math namespace against which the expression was compiled.
- `scope` is a [`Map`](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Map) 
  interface containing the variables defined in the scope 
  passed via `evaluate(scope)`. The passed scope is always a `Map` interface,
  and normally a `PartitionedMap` is used to separate local function variables
  like `x` in a custom defined function `f(x) = rawFunction(x) ^ 2` from the 
  scope variables. Note that a `PartitionedMap` can recursively link to another
  `PartitionedMap`.

Raw functions must be imported in the `math` namespace, as they need to be
processed at compile time. They are not supported when passed via a scope
at evaluation time.

A simple example:

```js
function myFunction(args, math, scope) {
  // get string representation of the arguments
  const str = args.map(function (arg) {
    return arg.toString()
  })

  // evaluate the arguments
  const res = args.map(function (arg) {
    return arg.compile().evaluate(scope)
  })

  return 'arguments: ' + str.join(',') + ', evaluated: ' + res.join(',')
}

// mark the function as "rawArgs", so it will be called with unevaluated arguments
myFunction.rawArgs = true

// import the new function in the math namespace
math.import({
  myFunction: myFunction
})

// use the function
math.evaluate('myFunction(2 + 3, sqrt(4))')
// returns 'arguments: 2 + 3, sqrt(4), evaluated: 5, 2'
```

## Customization: Custom LaTeX handlers

You can attach a `toTex` property to your custom functions before importing them to define their LaTeX output. This
`toTex` property can be a handler in the format described in the next section 'Custom LaTeX and String conversion'
or a template string similar to ES6 templates.

### Template syntax

- `${name}`: Gets replaced by the name of the function
- `${args}`: Gets replaced by a comma separated list of the arguments of the function.
- `${args[0]}`: Gets replaced by the first argument of a function
- `$$`: Gets replaced by `$`

#### Example

```js
const customFunctions = {
  plus: function (a, b) {
    return a + b
  },
  minus: function (a, b) {
    return a - b
  },
  binom: function (n, k) {
    return 1
  }
}

customFunctions.plus.toTex = '${args[0]}+${args[1]}' //template string
customFunctions.binom.toTex = '\\mathrm{${name}}\\left(${args}\\right)' //template string
customFunctions.minus.toTex = function (node, options) { //handler function
  return node.args[0].toTex(options) + node.name + node.args[1].toTex(options)
}

math.import(customFunctions)

math.parse('plus(1,2)').toTex()    // '1+2'
math.parse('binom(1,2)').toTex()   // '\\mathrm{binom}\\left(1,2\\right)'
math.parse('minus(1,2)').toTex()   // '1minus2'
```

## Customization: Custom HTML, LaTeX and string output

All expression nodes have a method `toTex` and `toString` to output an expression respectively in HTML or LaTex format or as regular text .
The functions `toHTML`, `toTex` and `toString` accept an `options` argument to customise output. This object is of the following form:

```js
{
  parenthesis: 'keep',    // parenthesis option
  handler: someHandler,   // handler to change the output
  implicit: 'hide'        // how to treat implicit multiplication
}
```

### Parenthesis

The `parenthesis` option changes the way parentheses are used in the output. There are three options available:

- `keep` Keep the parentheses from the input and display them as is. This is the default.
- `auto` Only display parentheses that are necessary. Mathjs tries to get rid of as much parentheses as possible.
- `all` Display all parentheses that are given by the structure of the node tree. This makes the output precedence unambiguous.

There's two ways of passing callbacks:

1. Pass an object that maps function names to callbacks. Those callbacks will be used for FunctionNodes with
functions of that name.
2. Pass a function to `toTex`. This function will then be used for every node.

```js
const expression = math.parse('(1+1+1)')

expression.toString()                      // (1 + 1 + 1)
expression.toString({parenthesis: 'keep'}) // (1 + 1 + 1)
expression.toString({parenthesis: 'auto'}) // 1 + 1 + 1
expression.toString({parenthesis: 'all'})  // (1 + 1) + 1
```

### Handler

You can provide the `toTex` and `toString` functions of an expression with your own custom handlers that override the internal behaviour. This is especially useful to provide LaTeX/string output for your own custom functions. This can be done in two ways:

1. Pass an object that maps function names to callbacks. Those callbacks will be used for FunctionNodes that contain functions with that name.
2. Pass a callback directly. This callback will run for every node, so you can replace the output of anything you like.

A callback function has the following form:

```js
function callback (node, options) {
  ...
}
```
Where `options` is the object passed to `toHTML`/`toTex`/`toString`. Don't forget to pass this on to the child nodes, and `node` is a reference to the current node.

If a callback returns nothing, the standard output will be used. If your callback returns a string, this string will be used.

**Although the following examples use `toTex`, it works for `toString` and `toHTML` in the same way**

#### Examples for option 1

```js
const customFunctions = {
  binomial: function (n, k) {
    //calculate n choose k
    // (do some stuff)
    return result
  }
}

const customLaTeX = {
  'binomial': function (node, options) { //provide toTex for your own custom function
    return '\\binom{' + node.args[0].toTex(options) + '}{' + node.args[1].toTex(options) + '}'
  },
  'factorial': function (node, options) { //override toTex for builtin functions
  	return 'factorial\\left(' + node.args[0] + '\\right)'
  }
}
```

You can simply use your custom toTex functions by passing them to `toTex`:

```js
math.import(customFunctions)
const expression = math.parse('binomial(factorial(2),1)')
const latex = expression.toTex({handler: customLaTeX})
// latex now contains "\binom{factorial\\left(2\\right)}{1}"
```

#### Examples for option 2:

```js
function customLaTeX(node, options) {
  if ((node.type === 'OperatorNode') && (node.fn === 'add')) {
    //don't forget to pass the options to the toTex functions
    return node.args[0].toTex(options) + ' plus ' + node.args[1].toTex(options)
  }
  else if (node.type === 'ConstantNode') {
    if (node.value === 0) {
        return '\\mbox{zero}'
    }
    else if (node.value === 1) {
        return '\\mbox{one}'
    }
    else if (node.value === 2) {
        return '\\mbox{two}'
    }
    else {
        return node.value
    }
  }
}

const expression = math.parse('1+2')
const latex = expression.toTex({handler: customLaTeX})
// latex now contains '\mbox{one} plus \mbox{two}'
```

Another example in conjunction with custom functions:

```js
const customFunctions = {
  binomial: function (n, k) {
    //calculate n choose k
    // (do some stuff)
    return result
  }
}

function customLaTeX(node, options) {
  if ((node.type === 'FunctionNode') && (node.name === 'binomial')) {
      return '\\binom{' + node.args[0].toTex(options) + '}{' + node.args[1].toTex(options) + '}'
  }
}

math.import(customFunctions)
const expression = math.parse('binomial(2,1)')
const latex = expression.toTex({handler: customLaTeX})
// latex now contains "\binom{2}{1}"
```

### Implicit multiplication

You can change the way that implicit multiplication is converted to a string or LaTeX. The two options are `hide`, to not show a multiplication operator for implicit multiplication and `show` to show it.

Example:

```js
const node = math.parse('2a')

node.toString()                   // '2 a'
node.toString({implicit: 'hide'}) // '2 a'
node.toString({implicit: 'show'}) // '2 * a'

node.toTex()                      // '2~ a'
node.toTex({implicit: 'hide'})    // '2~ a'
node.toTex({implicit: 'show'})    // '2\\cdot a'
```


## Customization: Customize supported characters

It is possible to customize the characters allowed in symbols and digits.
The `parse` function exposes the following test functions:

- `math.parse.isAlpha(c, cPrev, cNext)`
- `math.parse.isWhitespace(c, nestingLevel)`
- `math.parse.isDecimalMark(c, cNext)`
- `math.parse.isDigitDot(c)`
- `math.parse.isDigit(c)`

The exact signature and implementation of these functions can be looked up in
the [source code of the parser](https://github.com/josdejong/mathjs/blob/master/src/expression/parse.js). The allowed alpha characters are described here: [Constants and variables](syntax.md#constants-and-variables).

For example, the phone character <code>&#9742;</code> is not supported by default. It can be enabled
by replacing the `isAlpha` function:

```js
const isAlphaOriginal = math.parse.isAlpha
math.parse.isAlpha = function (c, cPrev, cNext) {
  return isAlphaOriginal(c, cPrev, cNext) || (c === '\u260E')
}

// now we can use the \u260E (phone) character in expressions
const result = math.evaluate('\u260Efoo', {'\u260Efoo': 42}) // returns 42
console.log(result)
```


## Expression trees

When parsing an expression via `math.parse(expr)`, math.js generates an
expression tree and returns the root node of the tree. An expression tree can
be used to analyze, manipulate, and evaluate expressions.

Example:

```js
const node = math.parse('sqrt(2 + x)')
```

In this case, the expression `sqrt(2 + x)` is parsed as:

```
  FunctionNode    sqrt
                   |
  OperatorNode     +
                  / \
  ConstantNode   2   x   SymbolNode
```

Alternatively, this expression tree can be built by manually creating nodes:

```js
const node1 = new math.ConstantNode(2)
const node2 = new math.SymbolNode('x')
const node3 = new math.OperatorNode('+', 'add', [node1, node2])
const node4 = new math.FunctionNode('sqrt', [node3])
```

The resulting expression tree with root node `node4` is equal to the expression
tree generated by `math.parse('sqrt(2 + x)')`.


## Expression trees: API

### Methods

All nodes have the following methods:

-   `clone() : Node`

    Create a shallow clone of the node.
    The node itself is cloned, its childs are not cloned.

-   `cloneDeep() : Node`

    Create a deep clone of the node.
    Both the node as well as all its childs are cloned recursively.

-   `compile() : Object`

    Compile an expression into optimized JavaScript code. `compile` returns an
    object with a function `evaluate([scope])` to evaluate. Example:

    ```js
    const node = math.parse('2 + x')   // returns the root Node of an expression tree
    const code = node.compile()        // returns {evaluate: function (scope) {...}}
    const evaluate = code.evaluate({x: 3}) // returns 5
    ```

-   `evaluate([scope]) : Object`

    Compile and evaluate an expression, this is the equivalent of doing
    `node.compile().evaluate(scope)`. Example:

    ```js
    const node = math.parse('2 + x')   // returns the root Node of an expression tree
    const evaluate = node.evaluate({x: 3}) // returns 5
    ```

-   `equals(other: Node) : boolean`

    Test whether this node equals an other node. Does a deep comparison of the
    values of both nodes.

-   `filter(callback: function) : Node[]`

    Recursively filter nodes in an expression tree. The `callback` function is
    called as `callback(node: Node, path: string, parent: Node) : boolean` for
    every node in the tree, and must return a boolean. The function `filter`
    returns an array with nodes for which the test returned true.
    Parameter `path` is a string containing a relative JSON Path.

    Example:

    ```js
    const node = math.parse('x^2 + x/4 + 3*y')
    const filtered = node.filter(function (node) {
      return node.isSymbolNode && node.name === 'x'
    })
    // returns an array with two entries: two SymbolNodes 'x'
    ```

-   `forEach(callback: function) : void`

    Execute a callback for each of the child nodes of this node. The `callback`
    function is called as `callback(child: Node, path: string, parent: Node): void`.
    Parameter `path` is a string containing a relative JSON Path.

    See also `traverse`, which is a recursive version of `forEach`.

    Example:

    ```js
    const node = math.parse('3 * x + 2')
    node.forEach(function (node, path, parent) {
      switch (node.type) {
        case 'OperatorNode':
          console.log(node.type, node.op)
          break
        case 'ConstantNode':
          console.log(node.type, node.value)
          break
        case 'SymbolNode':
          console.log(node.type, node.name)
          break
        default:
          console.log(node.type)
      }
    })
    // outputs:
    //   OperatorNode *
    //   ConstantNode 2
    ```

-   `map(callback: function) : Node[]`

    Transform a node. Creates a new Node having it's childs be the results of
    calling the provided callback function for each of the childs of the original
    node. The `callback` function is called as `callback(child: Node, path: string,
    parent: Node)` and must return a Node. Parameter `path` is a string containing
    a relative JSON Path.

    See also `transform`, which is a recursive version of `map`.

-	`toHTML(options: object): string`

	Get a HTML representation of the parsed expression. Example:

    ```js
    const node = math.parse('sqrt(2/3)')
    node.toHTML()
	// returns
	// <span class="math-function">sqrt</span>
	// <span class="math-paranthesis math-round-parenthesis">(</span>
	// <span class="math-number">2</span>
	// <span class="math-operator math-binary-operator math-explicit-binary-operator">/</span>
	// <span class="math-number">3</span>
	// <span class="math-paranthesis math-round-parenthesis">)</span>
    ```

    Information about the available HTML classes in [HTML Classes](html_classes.md).
    Information about the options in [Customization](customization.md#custom-html-latex-and-string-output).

-   `toString(options: object) : string`

    Get a string representation of the parsed expression. This is not exactly
    the same as the original input. Example:

    ```js
    const node = math.parse('3+4*2')
    node.toString()                      // returns '3 + 4 * 2'
    node.toString({parenthesis: 'all'})  // returns '3 + (4 * 2)'
    ```

    Information about the options in [Customization](customization.md#custom-html-latex-and-string-output).

-   `toTex(options: object): string`

    Get a [LaTeX](https://en.wikipedia.org/wiki/LaTeX) representation of the
    expression. Example:

    ```js
    const node = math.parse('sqrt(2/3)')
    node.toTex() // returns '\sqrt{\frac{2}{3}}'
    ```

    Information about the options in [Customization](customization.md#custom-html-latex-and-string-output).

-   `transform(callback: function)`

    Recursively transform an expression tree via a transform function. Similar
    to `Array.map`, but recursively executed on all nodes in the expression tree.
    The callback function is a mapping function accepting a node, and returning
    a replacement for the node or the original node. Function `callback` is
    called as `callback(node: Node, path: string, parent: Node)` for every node
    in the tree, and must return a `Node`. Parameter `path` is a string containing
    a relative JSON Path.

    The transform function will stop iterating when a node is replaced by the
    callback function, it will not iterate over replaced nodes.

    For example, to replace all nodes of type `SymbolNode` having name 'x' with a
    ConstantNode with value `3`:

    ```js
    const node = math.parse('x^2 + 5*x')
    const transformed = node.transform(function (node, path, parent) {
      if (node.isSymbolNode && node.name === 'x') {
        return new math.ConstantNode(3)
      }
      else {
        return node
      }
    })
    transformed.toString() // returns '3 ^ 2 + 5 * 3'
    ```

-   `traverse(callback: function): void`

    Recursively traverse all nodes in a node tree. Executes given callback for
    this node and each of its child nodes. Similar to `Array.forEach`, except
    recursive.
    The callback function is a mapping function accepting a node, and returning
    nothing. Function `callback` is
    called as `callback(node: Node, path: string, parent: Node)` for every node
    in the tree. Parameter `path` is a string containing a relative JSON Path.
    Example:

    ```js
    const node = math.parse('3 * x + 2')
    node.traverse(function (node, path, parent) {
      switch (node.type) {
        case 'OperatorNode':
          console.log(node.type, node.op)
          break
        case 'ConstantNode':
          console.log(node.type, node.value)
          break
        case 'SymbolNode':
          console.log(node.type, node.name)
          break
        default:
          console.log(node.type)
      }
    })
    // outputs:
    //   OperatorNode +
    //   OperatorNode *
    //   ConstantNode 3
    //   SymbolNode x
    //   ConstantNode 2
    ```


### Properties

Each `Node` has the following properties:

-   `comment: string`

    A string holding a comment if there was any in the expression, or else the
    string will be empty string. A comment can be attached to the root node of
    an expression or to each of the childs nodes of a `BlockNode`.

-   `isNode: true`

    Is defined with value `true` on Nodes. Additionally, each type of node
    adds it's own flag, for example a `SymbolNode` as has a property
    `isSymbolNode: true`.

-   `type: string`

    The type of the node, for example `'SymbolNode'` in case of a `SymbolNode`.


## Expression trees: Nodes

math.js has the following types of nodes. All nodes are available at the
namespace `math`.


### AccessorNode

Construction:

```
new AccessorNode(object: Node, index: IndexNode)
new AccessorNode(object: Node, index: IndexNode, optionalChaining: boolean)
```

An optional property `optionalChaining` can be provided whether the accessor was 
written as optional-chaining using `a?.b`, or `a?.["b"]` with bracket notation. 
Default value is `false`. Forces evaluate to undefined if the given object is 
undefined or null.

Properties:

- `object: Node`
- `index: IndexNode`
- `name: string` (read-only) The function or method name. Returns an empty string when undefined.
- `optionalChaining: boolean`

Examples:

```js
const node1 = math.parse('a[3]')

const object = new math.SymbolNode('a')
const constant3 = new math.ConstantNode(3)
const index = new math.IndexNode([constant3])
const node2 = new math.AccessorNode(object, index)
```


### ArrayNode

Construction:

```
new ArrayNode(items: Node[])
```

Properties:

- `items: Node[]`

Examples:

```js
const node1 = math.parse('[1, 2, 3]')

const one    = new math.ConstantNode(1)
const two    = new math.ConstantNode(2)
const three  = new math.ConstantNode(3)
const node2  = new math.ArrayNode([one, two, three])
```


### AssignmentNode

Construction:

```
new AssignmentNode(object: SymbolNode, value: Node)
new AssignmentNode(object: SymbolNode | AccessorNode, index: IndexNode, value: Node)
```

Properties:

- `object: SymbolNode | AccessorNode`
- `index: IndexNode | null`
- `value: Node`
- `name: string` (read-only) The function or method name. Returns an empty string when undefined.

Examples:

```js
const node1 = math.parse('a = 3')

const object = new math.SymbolNode('a')
const value = new math.ConstantNode(3)
const node2 = new math.AssignmentNode(object, value)
```


### BlockNode

A `BlockNode` is created when parsing a multi line expression like `a=2;b=3` or
`a=2\nb=3`. Evaluating a `BlockNode` returns a `ResultSet`. The results can be
retrieved via `ResultSet.entries` or `ResultSet.valueOf()`, which contains
an `Array` with the results of the visible lines (i.e. lines not ending with
a semicolon).

Construction:

```
block = new BlockNode(Array.<{node: Node} | {node: Node, visible: boolean}>)
```

Properties:

- `blocks: Array.<{node: Node, visible: boolean}>`

Examples:

```js
const block1 = math.parse('a=1; b=2; c=3')

const a = new math.SymbolNode('a')
const one = new math.ConstantNode(1)
const ass1 = new math.AssignmentNode(a, one)

const b = new math.SymbolNode('b')
const two = new math.ConstantNode(2)
const ass2 = new math.AssignmentNode(b, two)

const c = new math.SymbolNode('c')
const three = new math.ConstantNode(3)
const ass3 = new math.AssignmentNode(c, three)

const block2 = new BlockNode([
  {node: ass1, visible: false},
  {node: ass2, visible: false},
  {node: ass3, visible: true}
])
```


### ConditionalNode

Construction:

```
new ConditionalNode(condition: Node, trueExpr: Node, falseExpr: Node)
```

Properties:

- `condition: Node`
- `trueExpr: Node`
- `falseExpr: Node`

Examples:

```js
const node1 = math.parse('a > 0 ? a : -a')

const a         = new math.SymbolNode('a')
const zero      = new math.ConstantNode(0)
const condition = new math.OperatorNode('>', 'larger', [a, zero])
const trueExpr  = a
const falseExpr = new math.OperatorNode('-', 'unaryMinus', [a])
const node2     = new math.ConditionalNode(condition, trueExpr, falseExpr)
```

### ConstantNode

Construction:

```
new ConstantNode(value: *)
```

Properties:

- `value: *`

Examples:

```js
const node1 = math.parse('2.4')

const node2 = new math.ConstantNode(2.4)
const node3 = new math.ConstantNode('foo')
```


### FunctionAssignmentNode

Construction:

```
new FunctionAssignmentNode(name: string, params: string[], expr: Node)
```

Properties:

- `name: string`
- `params: string[]`
- `expr: Node`

Examples:

```js
const node1 = math.parse('f(x) = x^2')

const x      = new math.SymbolNode('x')
const two    = new math.ConstantNode(2)
const expr   = new math.OperatorNode('^', 'pow', [x, 2])
const node2  = new math.FunctionAssignmentNode('f', ['x'], expr)
```


### FunctionNode

Construction:

```
new FunctionNode(fn: Node | string, args: Node[])
```

Properties:

- `fn: Node | string` (read-only) The object or function name which to invoke.
- `args: Node[]`

Static functions:

- `FunctionNode.onUndefinedFunction(name: string)`. This function is invoked when an undefined function is evaluated. By default, the function throws an exception "Undefined function x". The function can be overwritten to customize this behavior. See also `SymbolNode.onUndefinedSymbol`.


Examples:

```js
const node1 = math.parse('sqrt(4)')

const four  = new math.ConstantNode(4)
const node3 = new math.FunctionNode(new SymbolNode('sqrt'), [four])
```


### IndexNode

Construction:

```
new IndexNode(dimensions: Node[])
new IndexNode(dimensions: Node[], dotNotation: boolean)
```

Each dimension can be a single value, a range, or a property. The values of
indices are one-based, including range end.

An optional property `dotNotation` can be provided describing whether this index
was written using dot notation like `a.b`, or using bracket notation
like `a["b"]`. Default value is `false`. This information is used when
stringifying the IndexNode.

Properties:

- `dimensions: Node[]`
- `dotNotation: boolean`

Examples:

```js
const node1 = math.parse('A[1:3, 2]')

const A     = new math.SymbolNode('A')
const one   = new math.ConstantNode(1)
const two   = new math.ConstantNode(2)
const three = new math.ConstantNode(3)

const range = new math.RangeNode(one, three)
const index = new math.IndexNode([range, two])
const node2 = new math.AccessorNode(A, index)
```

### ObjectNode

Construction:

```
new ObjectNode(properties: Object.<string, Node>)
```

Properties:

- `properties: Object.<string, Node>`

Examples:

```js
const node1 = math.parse('{a: 1, b: 2, c: 3}')

const a = new math.ConstantNode(1)
const b = new math.ConstantNode(2)
const c = new math.ConstantNode(3)
const node2 = new math.ObjectNode({a: a, b: b, c: c})
```


### OperatorNode

Construction:

```
new OperatorNode(op: string, fn: string, args: Node[], implicit: boolean = false)
```

Additional methods:

-   `isUnary() : boolean`

    Returns true when the `OperatorNode` contains exactly one argument,
    like with a unary minus:

    ```js
    const a = new math.ConstantNode(2)
    const b = new math.OperatorNode('-', 'unaryMinus', [a])
    b.isUnary() // true
    ```

-   `isBinary() : boolean`

    Returns true when the `OperatorNode` contains exactly two arguments,
    like with most regular operators:

    ```js
    const a = new math.ConstantNode(2)
    const b = new math.ConstantNode(3)
    const c = new math.OperatorNode('+', 'add', [a, b])
    c.isBinary() // true
    ```

Properties:

- `op: string`
- `fn: string`
- `args: Node[]`
- `implicit: boolean` True in case of an implicit multiplication, false otherwise

Examples:

```js
const node1 = math.parse('2.3 + 5')

const a     = new math.ConstantNode(2.3)
const b     = new math.ConstantNode(5)
const node2 = new math.OperatorNode('+', 'add', [a, b])
```

### ParenthesisNode

Construction:

```
new ParenthesisNode(content: Node)
```

Properties:

- `content: Node`

Examples:

```js
const node1 = math.parse('(1)')

const a     = new math.ConstantNode(1)
const node2 = new math.ParenthesisNode(a)
```

### RangeNode

Construction:

```
new RangeNode(start: Node, end: Node [, step: Node])
```

Properties:

- `start: Node`
- `end: Node`
- `step: Node | null`

Examples:

```js
const node1 = math.parse('1:10')
const node2 = math.parse('0:2:10')

const zero = new math.ConstantNode(0)
const one = new math.ConstantNode(1)
const two = new math.ConstantNode(2)
const ten = new math.ConstantNode(10)

const node3 = new math.RangeNode(one, ten)
const node4 = new math.RangeNode(zero, ten, two)
```

### RelationalNode

Construction:

```
new RelationalNode(conditionals: string[], params: Node[])
```

`conditionals` is an array of strings, each of which may be 'smaller', 'larger', 'smallerEq', 'largerEq', 'equal', or 'unequal'. The `conditionals` array must contain exactly one fewer item than `params`.

Properties:

- `conditionals: string[]`
- `params: Node[]`

A `RelationalNode` efficiently represents a chained conditional expression with two or more comparison operators, such as `10 < x <= 50`. The expression is equivalent to `10 < x and x <= 50`, except that `x` is evaluated only once, and evaluation stops (is "short-circuited") once any condition tests false. Operators that are subject to chaining are `<`, `>`, `<=`, `>=`, `==`, and `!=`. For backward compatibility, `math.parse` will return an `OperatorNode` if only a single conditional is present (such as `x > 2`).

Examples:

```js

const ten = new math.ConstantNode(10)
const x = new math.SymbolNode('x')
const fifty = new math.ConstantNode(50)

const node1 = new math.RelationalNode(['smaller', 'smallerEq'], [ten, x, fifty])
const node2 = math.parse('10 < x <= 50')
```


### SymbolNode

Construction:

```
new SymbolNode(name: string)
```

Properties:

- `name: string`

Static functions:

- `SymbolNode.onUndefinedSymbol(name: string)`. This function is invoked when an undefined symbol is evaluated. By default, the function throws an exception "Undefined symbol x". The function can be overwritten to customize this behavior. See also `FunctionNode.onUndefinedFunction`.


Examples:

```js
const node = math.parse('x')

const x = new math.SymbolNode('x')
```


## HTML output

The expression parser can output a HTML string, where every `Node` is
transformed into a `<span>` element with semantic class names. Each class
name begins with the `math-` prefix. These class names can be used in CSS to
highlight the syntax or change the default layout (e.g. spaces around operators).

## HTML output: Available class names

- `math-number`
- `math-string`
- `math-boolean` (`true` and `false`)
- `math-undefined`
- `math-function` (function names)
- `math-parameter` (function parameters)
- `math-property` (object members)
- `math-symbol` (variables, units and built-in constants)
  - `math-null-symbol` (`null`)
  - `math-nan-symbol` (`NaN`)
  - `math-infinity-symbol` (`Infinity`)
  - `math-imaginary-symbol` (`i`)
- `math-operator`
  - `math-unary-operator`
    - `math-lefthand-unary-operator`
    - `math-righthand-unary-operator`
  - `math-binary-operator`
    - `math-explicit-binary-operator`
    - `math-implicit-binary-operator` (empty element)
  - `math-assignment-operator`
    - `math-variable-assignment-operator` (`=`)
    - `math-property-assignment-operator` (`:` in objects)
  - `math-accessor-operator` (`.` in objects)
  - `math-range-operator` (`:` in ranges)
- `math-parenthesis`
  -`math-round-parenthesis` (`(` and `)`)
  -`math-square-parenthesis` (`[` and `]`)
  -`math-curly-parenthesis` (`{` and `}`)
- `math-separator` (�,`, `;` and <code>&lt;br /&gt;</code>)

## Expressions

Math.js contains a flexible and easy to use expression parser.
The parser supports all data types, functions and constants available in math.js.

Whilst the math.js library is aimed at JavaScript developers, the expression
parser is aimed at end users: mathematicians, engineers, students, pupils.
The syntax of the expression parser differs from JavaScript and the low-level
math.js library.

This section is divided in the following pages:

- [Parsing and evaluation](parsing.md) describes how to parse and
  evaluate expressions with math.js.
- [Syntax](syntax.md) describes how to write expressions.
- [Expression trees](expression_trees.md) explains how to parse an expression into an
  expression tree, and use this to analyse and manipulate the expression.
- [Algebra](algebra.md) describing symbolic computation in math.js.
- [Customization](customization.md) describes how to customize processing and
  evaluation of expressions.
- [Security](security.md) about security risks of executing arbitrary expressions.


## Expression parsing and evaluation

Expressions can be parsed and evaluated in various ways:

- Using the function [`math.evaluate(expr [,scope])`](#evaluate).
- Using the function [`math.compile(expr)`](#compile).
- Using the function [`math.parse(expr)`](#parse).
- By creating a [parser](#parser), `math.parser()`, which contains a method
  `evaluate` and keeps a scope with assigned variables in memory.


## Expression parsing and evaluation: Evaluate

Math.js comes with a function `math.evaluate` to evaluate expressions. Syntax:

```js
math.evaluate(expr)
math.evaluate(expr, scope)
math.evaluate([expr1, expr2, expr3, ...])
math.evaluate([expr1, expr2, expr3, ...], scope)
```

Function `evaluate` accepts a single expression or an array with
expressions as the first argument and has an optional second argument
containing a `scope` with variables and functions. The scope can be a regular
JavaScript `Map` (recommended), a plain JavaScript `object`, or any custom 
class that implements the `Map` interface with methods `get`, `set`, `keys` 
and `has`. The scope will be used to resolve symbols, and to write assigned
variables and functions.

When an `Object` is used as scope, mathjs will internally wrap it in an 
`ObjectWrappingMap` interface since the internal functions can only use a `Map` 
interface. In case of custom defined functions like `f(x) = x^2`, the scope 
will be wrapped in a `PartitionedMap`, which reads and writes the function
variables (like `x` in this example) from a temporary map, and reads and writes
other variables from the original scope. The original scope is never copied, it
is only wrapped around when needed.

The following code demonstrates how to evaluate expressions.

```js
// evaluate expressions
math.evaluate('sqrt(3^2 + 4^2)')        // 5
math.evaluate('sqrt(-4)')               // 2i
math.evaluate('2 inch to cm')           // 5.08 cm
math.evaluate('cos(45 deg)')            // 0.7071067811865476

// provide a scope
let scope = {
    a: 3,
    b: 4
}
math.evaluate('a * b', scope)           // 12
math.evaluate('c = 2.3 + 4.5', scope)   // 6.8
scope.c                                 // 6.8
```


## Expression parsing and evaluation: Compile

Math.js contains a function `math.compile` which compiles expressions
into JavaScript code. This is a shortcut for first [parsing](#parse) and then
compiling an expression. The syntax is:

```js
math.compile(expr)
math.compile([expr1, expr2, expr3, ...])
```

Function `compile` accepts a single expression or an array with
expressions as the argument. Function `compile` returns an object with a function
`evaluate([scope])`, which can be executed to evaluate the expression against an
(optional) scope:

```js
const code = math.compile(expr)       // compile an expression
const result = code.evaluate([scope]) // evaluate the code with an optional scope
```

An expression needs to be compiled only once, after which the
expression can be evaluated repeatedly and against different scopes.
The optional scope is used to resolve symbols and to write assigned
variables or functions. Parameter [`scope`](#scope) can be a regular Object, or Map.

Example usage:

```js
// parse an expression into a node, and evaluate the node
const code1 = math.compile('sqrt(3^2 + 4^2)')
code1.evaluate()  // 5
```


## Expression parsing and evaluation: Parse

Math.js contains a function `math.parse` to parse expressions into an
[expression tree](expression_trees.md). The syntax is:

```js
math.parse(expr)
math.parse([expr1, expr2, expr3, ...])
```

Function `parse` accepts a single expression or an array with
expressions as the argument. Function `parse` returns the root node of the tree,
which can be successively compiled and evaluated:

```js
const node = math.parse(expr)         // parse expression into a node tree
const code = node.compile()           // compile the node tree
const result = code.evaluate([scope]) // evaluate the code with an optional scope
```

The API of nodes is described in detail on the page
[Expression trees](expression_trees.md).

An expression needs to be parsed and compiled only once, after which the
expression can be evaluated repeatedly. On evaluation, an optional scope
can be provided, which is used to resolve symbols and to write assigned
variables or functions. Parameter [`scope`](#scope) is a regular Object or Map.

Example usage:

```js
// parse an expression into a node, and evaluate the node
const node1 = math.parse('sqrt(3^2 + 4^2)')
const code1 = node1.compile()
code1.evaluate() // 5

// provide a scope
const node2 = math.parse('x^a')
const code2 = node2.compile()
let scope = {
    x: 3,
    a: 2
}
code2.evaluate(scope) // 9

// change a value in the scope and re-evaluate the node
scope.a = 3
code2.evaluate(scope) // 27
```

Parsed expressions can be exported to text using `node.toString()`, and can
be exported to LaTeX using `node.toTex()`. The LaTeX export can be used to
pretty print an expression in the browser with a library like
[MathJax](https://www.mathjax.org/). Example usage:

```js
// parse an expression
const node = math.parse('sqrt(x/x+1)')
node.toString()   // returns 'sqrt(x / x + 1)'
node.toTex()      // returns '\sqrt{\frac{ x}{ x}+1}'
```


## Expression parsing and evaluation: Parser

In addition to the static functions [`math.evaluate`](#evaluate) and
[`math.parse`](#parse), math.js contains a parser with functions `evaluate` and
`parse`, which automatically keeps a scope with assigned variables in memory.
The parser also contains some convenience functions to get, set, and remove
variables from memory.

A parser can be created by:

```js
const parser = math.parser()
```

The parser contains the following functions:

- `clear()`
  Completely clear the parser's scope.
- `evaluate(expr)`
  Evaluate an expression. Returns the result of the expression.
- `get(name)`
  Retrieve a variable or function from the parser's scope.
- `getAll()`
  Retrieve an object with all defined variables in the parser's scope.
- `getAllAsMap()`
  Retrieve a map with all defined variables in the parser's scope.
- `remove(name)`
  Remove a variable or function from the parser's scope.
- `set(name, value)`
  Set a variable or function in the parser's scope.

The following code shows how to create and use a parser.

```js
// create a parser
const parser = math.parser()

// evaluate expressions
parser.evaluate('sqrt(3^2 + 4^2)')      // 5
parser.evaluate('sqrt(-4)')             // 2i
parser.evaluate('2 inch to cm')         // 5.08 cm
parser.evaluate('cos(45 deg)')          // 0.7071067811865476

// define variables and functions
parser.evaluate('x = 7 / 2')            // 3.5
parser.evaluate('x + 3')                // 6.5
parser.evaluate('f(x, y) = x^y')        // f(x, y)
parser.evaluate('f(2, 3)')              // 8

// get and set variables and functions
const x = parser.get('x')               // x = 3.5
const f = parser.get('f')               // function
const g = f(3, 3)                       // g = 27
parser.set('h', 500)
parser.evaluate('h / 2')                // 250
parser.set('hello', function (name) {
    return 'hello, ' + name + '!'
})
parser.evaluate('hello("user")')        // "hello, user!"

// clear defined functions and variables
parser.clear()
```

## Expression parsing and evaluation: Scope

The scope is a data-structure used to store and lookup variables and functions defined and used by expressions.

It is passed to mathjs via calls to [`math.evaluate`](#evaluate) or `simplify`.

For ease of use, it can be a Plain Javascript Object; for safety it can be a plain `Map` and for flexibility, any object that has
the methods `get`/`set`/`has`/`keys`, seen on `Map`.

Some care is taken to mutate the same object that is passed into mathjs, so they can collect the definitions from mathjs scripts and expressions.

`evaluate` will fail if the expression uses a blacklisted symbol, preventing mathjs expressions to escape into Javascript. This is enforced by access to the scope.

For less reliance on this blacklist, scope can also be a `Map`, which allows mathjs expressions to define variables and functions of any name.

For more, see [examples of custom scopes](../../examples/advanced/custom_scope_objects.js).

## Expression parsing and evaluation: Serialization

All mathjs data types can be serialized. A scope containing variables can therefore be safely serialized too. However, in the expression parser it is possible to define functions, like:

```
f(x) = x^2
```

Such a custom function cannot be serialized on its own, since it may be bound to other variables in the scope.

A [`Parser`](#parser) can safely serialize all variables and functions evaluated via the expression parser:

```js
const parser = math.parser()

// evaluate some expressions
parser.evaluate('w = 2')
parser.evaluate('f(x) = x^w')
parser.evaluate('c = f(3)') // 9

// serialize the parser with its state
const str = JSON.stringify(parser)

// deserialize the parser again
const parser2 = JSON.parse(str, math.reviver)
parser.evaluate('f(4)') // 16
```


## Security

Executing arbitrary expressions like enabled by the expression parser of
mathjs involves a risk in general. When you're using mathjs to let users
execute arbitrary expressions, it's good to take a moment to think about
possible security and stability implications, especially when running
the code server side.

## Security: Security risks

A user could try to inject malicious JavaScript code via the expression
parser. The expression parser of mathjs offers a sandboxed environment
to execute expressions which should make this impossible. It's possible
though that there are unknown security vulnerabilities, so it's important
to be careful, especially when allowing server side execution of
arbitrary expressions.

The expression parser of mathjs parses the input in a controlled
way into an expression tree or abstract syntax tree (AST).
In a "compile" step, it does as much as possible preprocessing on the
static parts of the expression, and creates a fast performing function
which can be used to evaluate the expression repeatedly using a
dynamically passed scope.

The parser actively prevents access to JavaScripts internal `eval` and
`new Function` which are the main cause of security attacks. Mathjs
versions 4 and newer does not use JavaScript's `eval` under the hood.
Version 3 and older did use `eval` for the compile step. This is not
directly a security issue but results in a larger possible attack surface.

When running a node.js server, it's good to be aware of the different
types of security risks. The risk when running inside a browser may be
limited, though it's good to be aware of [Cross side scripting (XSS)](https://www.wikiwand.com/en/Cross-site_scripting) vulnerabilities. A nice overview of
the security risks of node.js servers is listed in the article [Node.js security checklist](https://blog.risingstack.com/node-js-security-checklist/) by Gergely Nemeth.

### Less vulnerable expression parser

There is a small number of functions which yield the biggest security
risk in the expression parser:

- `import` and `createUnit` which alter the built-in functionality and
  allow overriding existing functions and units, and `reviver` which parses 
  values into class instances.
- `evaluate`, `parse`, `simplify`, `derivative`, and `resolve` which parse 
  arbitrary input into a manipulable expression tree.

To make the expression parser less vulnerable whilst still supporting
most functionality, these functions can be disabled:

```js
import { create, all } from 'mathjs'

const math = create(all)
const limitedEvaluate = math.evaluate

math.import({
  // most important (hardly any functional impact)
  'import':     function () { throw new Error('Function import is disabled') },
  'createUnit': function () { throw new Error('Function createUnit is disabled') },
  'reviver':    function () { throw new Error('Function reviver is disabled') },

  // extra (has functional impact)
  'evaluate':   function () { throw new Error('Function evaluate is disabled') },
  'parse':      function () { throw new Error('Function parse is disabled') },
  'simplify':   function () { throw new Error('Function simplify is disabled') },
  'derivative': function () { throw new Error('Function derivative is disabled') },
  'resolve':    function () { throw new Error('Function resolve is disabled') },
}, { override: true })

console.log(limitedEvaluate('sqrt(16)'))     // Ok, 4
console.log(limitedEvaluate('parse("2+3")')) // Error: Function parse is disabled
```


### Found a security vulnerability? Please report in private!

You found a security vulnerability? Awesome! We hope you don't have bad
intentions and want to help fix the issue. Please report the
vulnerability in a private way by contacting one of the maintainers
via mail or another private channel. That way we can work together
on a fix before sharing the issue with everybody including the bad guys.

## Security: Stability risks

A user could accidentally or on purpose execute a
heavy expression like creating a huge matrix. That can let the
JavaScript engine run out of memory or freeze it when the CPU goes
to 100% for a long time.

To protect against this sort of issue, one can run the expression parser
in a separate Web Worker or child_process, so it can't affect the
main process. The workers can be killed when it runs for too
long or consumes too much memory. A useful library in this regard
is [workerpool](https://github.com/josdejong/workerpool), which makes
it easy to manage a pool of workers in both browser and node.js.


## Expression syntax

This page describes the syntax of the expression parser of math.js. It describes
how to work with the available data types, functions, operators, variables,
and more.

## Expression syntax: Differences from JavaScript

The expression parser of math.js is aimed at a mathematical audience,
not a programming audience. The syntax is similar to most calculators and
mathematical applications. This is close to JavaScript as well, though there
are a few important differences between the syntax of the expression parser and
the lower level syntax of math.js. Differences are:

- No need to prefix functions and constants with the `math.*` namespace,
  you can just enter `sin(pi / 4)`.
- Matrix indexes are one-based instead of zero-based.
- There are index and range operators which allow more conveniently getting
  and setting matrix indexes, like `A[2:4, 1]`.
- Both indexes and ranges and have the upper-bound included.
- There is a differing syntax for defining functions. Example: `f(x) = x^2`.
- There are custom operators like `x + y` instead of `add(x, y)`.
- Some operators are different. For example  `^` is used for exponentiation,
  not bitwise xor.
- Implicit multiplication, like `2 pi`, is supported and has special rules.
- Relational operators (`<`, `>`, `<=`, `>=`, `==`, and `!=`) are chained, so the expression `5 < x < 10` is equivalent to `5 < x and x < 10`.
- The precedence of some operators is different; see a complete list at the precedence table below.
- Multi-expression constructs like `a = 1; b = 2; a + b` or
  `"a = 1;\n cos(a)\n sin(a)"` (where `\n` denotes newline)
  produce a collection ("ResultSet") of values. Those expressions
  terminated by `;` are evaluated for side effect only and their values
  are suppressed from the result.

## Expression syntax: Operators

The expression parser has operators for all common arithmetic operations such
as addition and multiplication. The expression parser uses conventional infix
notation for operators: an operator is placed between its arguments.
Round parentheses can be used to override the default precedence of operators.

```js
// use operators
math.evaluate('2 + 3')       // 5
math.evaluate('2 * 3')       // 6

// use parentheses to override the default precedence
math.evaluate('2 + 3 * 4')   // 14
math.evaluate('(2 + 3) * 4') // 20
```

The following operators are available. Note that almost every operator listed
also has a function form with identical meaning that can be used
interchangeably. For example, `x+y` will always evaluate identically to
`add(x,y)`. For a full list of the equivalences, see the section on
Functions below.


Operator    | Name                        | Syntax      | Associativity | Example               | Result
----------- |-----------------------------|-------------| ------------- |-----------------------| ---------------
`(`, `)`    | Grouping                    | `(x)`       | None          | `2 * (3 + 4)`         | `14`
`[`, `]`    | Matrix, Index               | `[...]`     | None          | `[[1,2],[3,4]]`       | `[[1,2],[3,4]]`
`{`, `}`    | Object                      | `{...}`     | None          | `{a: 1, b: 2}`        | `{a: 1, b: 2}`
`,`         | Parameter separator         | `x, y`      | Left to right | `max(2, 1, 5)`        | `5`
`.`         | Property accessor           | `obj.prop`  | Left to right | `obj={a: 12}; obj.a`  | `12`
`;`         | Statement separator         | `x; y`      | Left to right | `a=2; b=3; a*b`       | `[6]`
`;`         | Row separator               | `[x; y]`    | Left to right | `[1,2;3,4]`           | `[[1,2],[3,4]]`
`\n`        | Statement separator         | `x \n y`    | Left to right | `a=2 \n b=3 \n a*b`   | `[2,3,6]`
`+`         | Add                         | `x + y`     | Left to right | `4 + 5`               | `9`
`+`         | Unary plus                  | `+y`        | Right to left | `+4`                  | `4`
`-`         | Subtract                    | `x - y`     | Left to right | `7 - 3`               | `4`
`-`         | Unary minus                 | `-y`        | Right to left | `-4`                  | `-4`
`*`         | Multiply                    | `x * y`     | Left to right | `2 * 3`               | `6`
`.*`        | Element-wise multiply       | `x .* y`    | Left to right | `[1,2,3] .* [1,2,3]`  | `[1,4,9]`
`/`         | Divide                      | `x / y`     | Left to right | `6 / 2`               | `3`
`./`        | Element-wise divide         | `x ./ y`    | Left to right | `[9,6,4] ./ [3,2,2]`  | `[3,3,2]`
`%`         | Percentage                  | `x%`        | None          | `8%`                  | `0.08`
`%`         | Addition with Percentage    | `x + y%`    | Left to right | `100 + 3%`            | `103`
`%`         | Subtraction with Percentage | `x - y%`    | Left to right | `100 - 3%`            | `97`
`%` `mod`   | Modulus                     | `x % y`     | Left to right | `8 % 3`               | `2`
`^`         | Power                       | `x ^ y`     | Right to left | `2 ^ 3`               | `8`
`.^`        | Element-wise power          | `x .^ y`    | Right to left | `[2,3] .^ [3,3]`      | `[8,27]`
`'`         | Transpose                   | `y'`        | Left to right | `[[1,2],[3,4]]'`      | `[[1,3],[2,4]]`
`!`         | Factorial                   | `y!`        | Left to right | `5!`                  | `120`
`&`         | Bitwise and                 | `x & y`     | Left to right | `5 & 3`               | `1`
`~`         | Bitwise not                 | `~x`        | Right to left | `~2`                  | `-3`
<code>&#124;</code>  | Bitwise or    | <code>x &#124; y</code> | Left to right | <code>5 &#124; 3</code> | `7`
<code>^&#124;</code> | Bitwise xor   | <code>x ^&#124; y</code> | Left to right | <code>5 ^&#124; 2</code> | `7`
`<<`        | Left shift                  | `x << y`    | Left to right | `4 << 1`              | `8`
`>>`        | Right arithmetic shift      | `x >> y`    | Left to right | `8 >> 1`              | `4`
`>>>`       | Right logical shift         | `x >>> y`   | Left to right | `-8 >>> 1`            | `2147483644`
`and`       | Logical and                 | `x and y`   | Left to right | `true and false`      | `false`
`not`       | Logical not                 | `not y`     | Right to left | `not true`            | `false`
`or`        | Logical or                  | `x or y`    | Left to right | `true or false`       | `true`
`xor`       | Logical xor                 | `x xor y`   | Left to right | `true xor true`       | `false`
`=`         | Assignment                  | `x = y`     | Right to left | `a = 5`               | `5`
`?` `:`     | Conditional expression      | `x ? y : z` | Right to left | `15 > 100 ? 1 : -1`   | `-1`
`??`        | Nullish coalescing          | `x ?? y`    | Left to right | `null ?? 2`           | `2`
`?.`        | Optional chaining accessor  | `obj?.prop` | Left to right | `obj={}; obj?.a`      | `undefined`
`:`         | Range                       | `x : y`     | Right to left | `1:4`                 | `[1,2,3,4]`
`to`, `in`  | Unit conversion             | `x to y`    | Left to right | `2 inch to cm`        | `5.08 cm`
`==`        | Equal                       | `x == y`    | Left to right | `2 == 4 - 2`          | `true`
`!=`        | Unequal                     | `x != y`    | Left to right | `2 != 3`              | `true`
`<`         | Smaller                     | `x < y`     | Left to right | `2 < 3`               | `true`
`>`         | Larger                      | `x > y`     | Left to right | `2 > 3`               | `false`
`<=`        | Smallereq                   | `x <= y`    | Left to right | `4 <= 3`              | `false`
`>=`        | Largereq                    | `x >= y`    | Left to right | `2 + 4 >= 6`          | `true`


## Expression syntax: Precedence

The operators have the following precedence, from highest to lowest:

Operators                         | Description
--------------------------------- | --------------------
`(...)`<br>`[...]`<br>`{...}`     | Grouping<br>Matrix<br>Object
`x(...)`<br>`x[...]`<br>`obj.prop`<br>`:`| Function call<br>Matrix index<br>Property accessor<br>Key/value separator
`'`                               | Matrix transpose
`!`                               | Factorial
`??`                              | Nullish coalescing
`^`, `.^`                         | Exponentiation
`+`, `-`, `~`, `not`              | Unary plus, unary minus, bitwise not, logical not
`%`                               | Unary percentage
See section below                 | Implicit multiplication
`*`, `/`, `.*`, `./`,`%`, `mod`   | Multiply, divide, modulus
`+`, `-`                          | Add, subtract
`:`                               | Range
`to`, `in`                        | Unit conversion
`<<`, `>>`, `>>>`                 | Bitwise left shift, bitwise right arithmetic shift, bitwise right logical shift
`==`, `!=`, `<`, `>`, `<=`, `>=`  | Relational
`&`                               | Bitwise and (lazily evaluated)
<code>^&#124;</code>              | Bitwise xor
<code>&#124;</code>               | Bitwise or (lazily evaluated)
`and`                             | Logical and (lazily evaluated)
`xor`                             | Logical xor
`or`                              | Logical or (lazily evaluated)
`?`, `:`                          | Conditional expression
`=`                               | Assignment
`,`                               | Parameter and column separator
`;`                               | Row separator
`\n`, `;`                         | Statement separators

Lazy evaluation is used where logically possible for bitwise and logical
operators. In the following example, the sub-expression `x` will not even be
evaluated because it cannot affect the final result:
```js
math.evaluate('false and x')        // false, no matter what x equals
```


## Expression syntax: Functions

Functions are called by entering their name, followed by zero or more
arguments enclosed by parentheses. All available functions are listed on the
page [Functions](../reference/functions.md).

```js
math.evaluate('sqrt(25)')           // 5
math.evaluate('log(10000, 3 + 7)')  // 4
math.evaluate('sin(pi / 4)')        // 0.7071067811865475
```

New functions can be defined by "assigning" an expression to a function call
with one or more variables. Such function assignments are limited: they can
only be defined on a single line.

```js
const parser = math.parser()

parser.evaluate('f(x) = x ^ 2 - 5')
parser.evaluate('f(2)')     // -1
parser.evaluate('f(3)')     // 4

parser.evaluate('g(x, y) = x ^ y')
parser.evaluate('g(2, 3)')  // 8
```

Note that these function assignments do _not_ create closures; put another way,
all free variables in mathjs are dynamic:

```js
const parser = math.parser()

parser.evaluate('x = 7')
parser.evaluate('h(y) = x + y')
parser.evaluate('h(3)')         // 10
parser.evaluate('x = 3')
parser.evaluate('h(3)')         // 6, *not* 10
```

It is however possible to pass functions as parameters:

```js
const parser = math.parser()

parser.evaluate('twice(func, x) = func(func(x))')
parser.evaluate('twice(square, 2)')    // 16
parser.evaluate('f(x) = 3*x')
parser.evaluate('twice(f, 2)')         // 18

// a simplistic "numerical derivative":
parser.evaluate('eps = 1e-10')
parser.evaluate('nd(f, x) = (f(x+eps) - f(x-eps))/(2*eps)')
parser.evaluate('nd(square,2)')        // 4.000000330961484
```

Math.js itself heavily uses typed functions, which ensure correct inputs and
throws meaningful errors when the input arguments are invalid. One can create
a [typed-function](https://github.com/josdejong/typed-function) in the
expression parser like:

```js
const parser = math.parser()

parser.evaluate('f = typed({"number": f(x) = x ^ 2 - 5})')
```

Finally, as mentioned above, there is a function form for nearly every one of
the mathematical operator symbols. Moreover, for some associative operators,
the corresponding function allows arbitrarily many arguments. The table below
gives the full correspondence.

Operator Expression  | Equivalent Function Expression
---------------------|-------------------------------
`a or b`             |`or(a,b)`
`a xor b`            |`xor(a,b)`
`a and b`            |`and(a,b)`
`a \| b`             |`bitOr(a,b)`
`a ^\| b`            |`bitXor(a,b)`
`a & b`              |`bitAnd(a,b)`
`a ?? b`             |`nullish(a,b)`
`a == b`             |`equal(a,b)`
`a != b`             |`unequal(a,b)`
`a < b`              |`smaller(a,b)`
`a > b`              |`larger(a,b)`
`a <= b`             |`smallerEq(a,b)`
`a << 3`             |`leftShift(a,3)`
`a >> 3`             |`rightArithShift(a,3)`
`a >>> 3`            |`rightLogShift(a,3)`
`u to cm`            |`to(u, cm)`
`a + b + c + ...`    |`add(a,b,c,...)`
`a - b`              |`subtract(a,b)`
`a * b * c * ...`    |`multiply(a,b,c,...)`
`A .* B`             |`dotMultiply(A,B)`
`A ./ B`             |`dotDivide(A,B)`
`a mod b`            |`mod(a,b)`
`+a`                 |`unaryPlus(a)`
`-a`                 |`unaryMinus(a)`
`~a`                 |`bitNot(a)`
`not a`              |`not(a)`
`a^b`                |`pow(a,b)`
`A .^ B`             |`dotPow(A,B)`
`a!`                 |`factorial(a)`
`A'`                 |`ctranspose(A)`

Note that math.js embodies a preference for the operator forms, in that calling
`simplify` (see [Algebra](./algebra.md)) converts any instances of the function
form into the corresponding operator.

## Expression syntax: Methods

Some data types have methods which can be used in the parser using the dot notation, for example:

```js
const parser = math.parser()

parser.evaluate('a = 1 m') // Unit 1 m
parser.evaluate('a.toNumber("mm")') // 1000
```

Or a method on a Matrix or Array:

```js
const parser = math.parser()

parser.evaluate('M = [4, 9, 25]') // Matrix [4, 9, 25]
parser.evaluate('M.map(sqrt)')    // Matrix [2, 3, 5]
```

## Expression syntax: Map and forEach

The `map` and `forEach` functions can be used to apply a callback function to each element of an array or matrix.

The callback functions can be functions, typed functions, inline functions (only in the parser) or compiled inline functions (only in the parser).

The callback can have the following inputs:
- **value**: the current value in the array or matrix.
- **index**: the index of the current value expressed as an array of numbers.
- **array**: the array or matrix being iterated.

Below is the syntax for both functions:

### map

The `map` function applies a function to each element of an array and returns a new array with the results.

```js
const parser = math.parser()

// Define a square function
parser.evaluate('square(x) = x ^ 2')

// Apply the square function to each element of the array
parser.evaluate('result = map([1, 2, 3, 4], square)')
// result: [1, 4, 9, 16]

// Apply an inline function to each element of the array
parser.evaluate('result = map([1, 2, 3, 4], f(x) = x ^ 2)')
// result: [1, 4, 9, 16]

// Apply a compiled inline function to each element of the array
parser.evaluate('result = map([1, 2, 3, 4], x ^ 2)')
// result: [1, 4, 9, 16]
```

### forEach
The `forEach` function applies a function to each element of an array or matrix but does not return a new array. It is useful for executing side effects.

```js
// Define a function that prints each element
math.import({consoleLog: x => console.log(x)})
const parser = math.parser()

// Define a squareConsleLog function
parser.evaluate('squareConsoleLog(x) = consoleLog(x ^ 2)')

// Apply the squareConsleLog function to each element of the array
parser.evaluate('forEach([1, 2, 3, 4], squareConsleLog)')
// Prints: 1, 4, 9, 16

// Apply an inline function to each element of the array
parser.evaluate('forEach([1, 2, 3, 4], f(x) = consoleLog(x ^ 2))')
// Prints: 1, 4, 9, 16

// Apply a compiled inline function to each element of the array
parser.evaluate('forEach([1, 2, 3, 4], consoleLog(x ^ 2))')
// Prints: 1, 4, 9, 16
```

These functions are useful for performing element-wise operations on arrays or matrices.

## Expression syntax: Constants and variables

Math.js has a number of built-in constants such as `pi` and `e`.
All available constants are listed on he page
[Constants](../reference/constants.md).

```js
// use constants
math.evaluate('pi')                 // 3.141592653589793
math.evaluate('e ^ 2')              // 7.3890560989306495
math.evaluate('log(e)')             // 1
math.evaluate('e ^ (pi * i) + 1')   // ~0 (Euler)
```

Variables can be defined using the assignment operator `=`, and can be used
like constants.

```js
const parser = math.parser()

// define variables
parser.evaluate('a = 3.4')      // 3.4
parser.evaluate('b = 5 / 2')    // 2.5

// use variables
parser.evaluate('a * b')        // 8.5
```

Variable names must:

- Begin with an "alpha character", which is:
  - A latin letter (upper or lower case). Ascii: `a-z`, `A-Z`
  - An underscore.                        Ascii: `_`
  - A dollar sign.                        Ascii: `$`
  - A latin letter with accents.          Unicode: `\u00C0` - `\u02AF`
  - A greek letter.                       Unicode: `\u0370` - `\u03FF`
  - A letter-like character.              Unicode: `\u2100` - `\u214F`
  - A mathematical alphanumeric symbol.   Unicode: `\u{1D400}` - `\u{1D7FF}` excluding invalid code points
- Contain only alpha characters (above) and digits `0-9`
- Not be any of the following: `mod`, `to`, `in`, `and`, `xor`, `or`, `not`, `end`. It is possible to assign to some of these, but that's not recommended.

It is possible to customize the allowed alpha characters, see [Customize supported characters](customization.md#customize-supported-characters) for more information.


## Expression syntax: Data types

The expression parser supports booleans, numbers, complex numbers, units,
strings, matrices, and objects.


### Booleans

Booleans `true` and `false` can be used in expressions.

```js
// use booleans
math.evaluate('true')               // true
math.evaluate('false')              // false
math.evaluate('(2 == 3) == false')  // true
```

Booleans can be converted to numbers and strings and vice versa using the
functions `number` and `boolean`, and `string`.

```js
// convert booleans
math.evaluate('number(true)')       // 1
math.evaluate('string(false)')      // "false"
math.evaluate('boolean(1)')         // true
math.evaluate('boolean("false")')   // false
```


### Numbers

The most important and basic data type in math.js are numbers. Numbers use a
point as decimal mark. Numbers can be entered with exponential notation.
Examples:

```js
// numbers in math.js
math.evaluate('2')        // 2
math.evaluate('3.14')     // 3.14
math.evaluate('1.4e3')    // 1400
math.evaluate('22e-3')    // 0.022
```

A number can be converted to a string and vice versa using the functions
`number` and `string`.

```js
// convert a string into a number
math.evaluate('number("2.3")')    // 2.3
math.evaluate('string(2.3)')      // "2.3"
```

Math.js uses regular JavaScript numbers, which are floating points with a
limited precision and limited range. The limitations are described in detail
on the page [Numbers](../datatypes/numbers.md).

```js
math.evaluate('1e-325')   // 0
math.evaluate('1e309')    // Infinity
math.evaluate('-1e309')   // -Infinity
```

When doing calculations with floats, one can very easily get round-off errors:

```js
// round-off error due to limited floating point precision
math.evaluate('0.1 + 0.2')  // 0.30000000000000004
```

When outputting results, the function `math.format` can be used to hide
these round-off errors when outputting results for the user:

```js
const ans = math.evaluate('0.1 + 0.2')  //  0.30000000000000004
math.format(ans, {precision: 14})       // "0.3"
```

Numbers can be expressed as binary, octal, and hexadecimal literals:

```js
math.evaluate('0b11')  //  3
math.evaluate('0o77')  //  63
math.evaluate('0xff')  //  255
```

A word size suffix can be used to change the behavior of non decimal literal evaluation:

```js
math.evaluate('0xffi8')         // -1
math.evaluate('0xffffffffi32')  //  -1
math.evaluate('0xfffffffffi32') //  SyntaxError: String "0xfffffffff" is out of range
```

Non decimal numbers can include a radix point:
```js
math.evaluate('0b1.1')         // 1.5
math.evaluate('0o1.4')         // 1.5
math.evaluate('0x1.8')         // 1.5
```

Numbers can be formatted as binary, octal, and hex strings using the `notation` option of the `format` function:

```js
math.evaluate('format(3, {notation: "bin"})')    //  '0b11'
math.evaluate('format(63, {notation: "oct"})')   //  '0o77'
math.evaluate('format(255, {notation: "hex"})')  //  '0xff'
math.evaluate('format(-1, {notation: "hex"})')   //  '-0x1'
math.evaluate('format(2.3, {notation: "hex"})')  //  '0x2.4cccccccccccc'
```

The `format` function accepts a `wordSize` option to use in conjunction with the non binary notations:

```js
math.evaluate('format(-1, {notation: "hex", wordSize: 8})')   //  '0xffi8'
```

The functions `bin`, `oct`, and `hex` are shorthand for the `format` function with `notation` set accordingly:

```js
math.evaluate('bin(-1)')     // '-0b1'
math.evaluate('bin(-1, 8)')  // '0b11111111i8'
```

### BigNumbers

Math.js supports BigNumbers for calculations with an arbitrary precision.
The pros and cons of Number and BigNumber are explained in detail on the page
[Numbers](../datatypes/numbers.md).

BigNumbers are slower but have a higher precision. Calculations with big
numbers are supported only by arithmetic functions.

BigNumbers can be created using the `bignumber` function:

```js
math.evaluate('bignumber(0.1) + bignumber(0.2)') // BigNumber, 0.3
```

The default number type of the expression parser can be changed at instantiation
of math.js. The expression parser parses numbers as BigNumber by default:

```js
// Configure the type of number: 'number' (default), 'BigNumber', or 'Fraction'
math.config({number: 'BigNumber'})

// all numbers are parsed as BigNumber
math.evaluate('0.1 + 0.2')  // BigNumber, 0.3
```

BigNumbers can be converted to numbers and vice versa using the functions
`number` and `bignumber`. When converting a BigNumber to a Number, the high
precision of the BigNumber will be lost. When a BigNumber is too large to be represented
as Number, it will be initialized as `Infinity`.


### Complex numbers

Complex numbers can be created using the imaginary unit `i`, which is defined
as `i^2 = -1`. Complex numbers have a real and complex part, which can be
retrieved using the functions `re` and `im`.

```js
const parser = math.parser()

// create complex numbers
parser.evaluate('a = 2 + 3i')   // Complex, 2 + 3i
parser.evaluate('b = 4 - i')    // Complex, 4 - i

// get real and imaginary part of a complex number
parser.evaluate('re(a)')        // Number,  2
parser.evaluate('im(a)')        // Number,  3

// calculations with complex numbers
parser.evaluate('a + b')        // Complex, 6 + 2i
parser.evaluate('a * b')        // Complex, 11 + 10i
parser.evaluate('i * i')        // Number,  -1
parser.evaluate('sqrt(-4)')     // Complex, 2i
```


### Units

math.js supports units. Units can be used in the arithmetic operations
add, subtract, multiply, divide, and exponentiation.
Units can also be converted from one to another.
An overview of all available units can be found on the page
[Units](../datatypes/units.md).

In expressions, `min` and `sec` resolve by default to the minimum and secant
functions instead of abbreviations for the minute and second units. Use
`minute`, `second`, or `s` for those units in an expression, or parse the
abbreviations with `math.unit('5 min')` and `math.unit('5 sec')`. A scope can
explicitly override these symbols.

Units can be converted using the operator `to` or `in`.

```js
// create a unit
math.evaluate('5.4 kg')                     // Unit, 5.4 kg

// convert a unit
math.evaluate('2 inch to cm')               // Unit, 5.08 cm
math.evaluate('20 celsius in fahrenheit')   // Unit, ~68 fahrenheit
math.evaluate('90 km/h to m/s')             // Unit, 25 m / s

// convert a unit to a number
// A second parameter with the unit for the exported number must be provided
math.evaluate('number(5 cm, mm)')           // Number, 50

// calculations with units
math.evaluate('0.5kg + 33g')                // Unit, 0.533 kg
math.evaluate('3 inch + 2 cm')              // Unit, 3.7874 inch
math.evaluate('3 inch + 2 cm')              // Unit, 3.7874 inch
math.evaluate('12 seconds * 2')             // Unit, 24 seconds
math.evaluate('sin(45 deg)')                // Number, 0.7071067811865475
math.evaluate('9.81 m/s^2 * 5 s to mi/h')   // Unit, 109.72172512527 mi / h
```


### Strings

Strings are enclosed by double quotes " or single quotes '. Strings can be concatenated using the
function `concat` (not by adding them using `+` like in JavaScript). Parts of
a string can be retrieved or replaced by using indexes. Strings can be converted
to a number using function `number`, and numbers can be converted to a string
using function `string`.

When setting the value of a character in a string, the character that has been
set is returned. Likewise, when a range of characters is set, that range of
characters is returned.


```js
const parser = math.parser()

// create a string
parser.evaluate('"hello"')                        // String, "hello"

// string manipulation
parser.evaluate('a = concat("hello", " world")')  // String, "hello world"
parser.evaluate('size(a)')                        // Matrix [11]
parser.evaluate('a[1:5]')                         // String, "hello"
parser.evaluate('a[1] = "H"')                     // String, "H"
parser.evaluate('a[7:12] = "there!"')             // String, "there!"
parser.evaluate('a')                              // String, "Hello there!"

// string conversion
parser.evaluate('number("300")')                  // Number, 300
parser.evaluate('string(300)')                    // String, "300"
```

Strings can be used in the `evaluate` function, to parse expressions inside
the expression parser:

```js
math.evaluate('evaluate("2 + 3")')  // 5
```


### Matrices

Matrices can be created by entering a series of values between square brackets,
elements are separated by a comma `,`.
A matrix like `[1, 2, 3]` will create a vector, a 1-dimensional matrix with
size `[3]`. To create a multi-dimensional matrix, matrices can be nested into
each other. For easier creation of two-dimensional matrices, a semicolon `;`
can be used to separate rows in a matrix.

```js
// create a matrix
math.evaluate('[1, 2, 3]')                                // Matrix, size [3]
math.evaluate('[[1, 2, 3], [4, 5, 6]]')                   // Matrix, size [2, 3]
math.evaluate('[[[1, 2], [3, 4]], [[5, 6], [7, 8]]]')     // Matrix, size [2, 2, 2]

// create a two dimensional matrix
math.evaluate('[1, 2, 3; 4, 5, 6]')                       // Matrix, size [2, 3]
```

Another way to create filled matrices is using the functions `zeros`, `ones`,
`identity`, and `range`.

```js
// initialize a matrix with ones or zeros
math.evaluate('zeros(3, 2)')      // Matrix, [[0, 0], [0, 0], [0, 0]],  size [3, 2]
math.evaluate('ones(3)')          // Matrix, [1, 1, 1],                 size [3]
math.evaluate('5 * ones(2, 2)')   // Matrix, [[5, 5], [5, 5]],          size [2, 2]

// create an identity matrix
math.evaluate('identity(2)')      // Matrix, [[1, 0], [0, 1]],          size [2, 2]

// create a range
math.evaluate('1:4')              // Matrix, [1, 2, 3, 4],              size [4]
math.evaluate('0:2:10')           // Matrix, [0, 2, 4, 6, 8, 10],       size [6]
```

A subset can be retrieved from a matrix using indexes and a subset of a matrix
can be replaced by using indexes. Indexes are enclosed in square brackets, and
contain a number or a range for each of the matrix dimensions. A range can have
its start and/or end undefined. When the start is undefined, the range will start
at 1, when the end is undefined, the range will end at the end of the matrix.

There is a context variable `end` available as well to denote the end of the
matrix. This variable cannot be used in multiple nested indices. In that case,
`end` will be resolved as the end of the innermost matrix. To solve this,
resolving of the nested index needs to be split in two separate operations.

*IMPORTANT: matrix indexes and ranges work differently from the math.js indexes
in JavaScript: They are one-based with an included upper-bound, similar to most
math applications.*

*IMPORTANT: The matrix indexing behavior has been changed in mathjs `v15`, see section [Migrate matrix indexing to mathjs v15](#migrate-matrix-indexing-to-mathjs-v15).*

```js
parser = math.parser()

// create matrices
parser.evaluate('a = [1, 2; 3, 4]')       // Matrix, [[1, 2], [3, 4]]
parser.evaluate('b = zeros(2, 2)')        // Matrix, [[0, 0], [0, 0]]
parser.evaluate('c = 5:9')                // Matrix, [5, 6, 7, 8, 9]

// replace a subset in a matrix (returns the assigned value)
parser.evaluate('b[1, 1:2] = [5, 6]')     // Matrix, [5, 6]
parser.evaluate('b[2, :] = [7, 8]')       // Matrix, [7, 8]
parser.evaluate('b')                      // Matrix, [[5, 6], [7, 8]]

// perform a matrix calculation
parser.evaluate('d = a * b')              // Matrix, [[19, 22], [43, 50]]

// retrieve a subset of a matrix
parser.evaluate('d[2, 1]')                // 43
parser.evaluate('d[2, 1:end]')            // Matrix, [43, 50]
parser.evaluate('c[end - 1 : -1 : 2]')    // Matrix, [8, 7, 6]
```

#### Migrate matrix indexing to mathjs v15

With mathjs v15, matrix indexing has changed to be more consistent and predictable. In v14, using a scalar index would sometimes reduce the dimensionality of the result. In v15, if you want to preserve dimensions, use array, matrix, or range indices. If you want a scalar value, use scalar indices.

To maintain the old indexing behavior without need for any code changes, use the configuration option `legacySubset`:

```js
math.config({ legacySubset: true })
```

To migrate your code, you'll have to change all matrix indexes from the old index notation to the new index notation. Basically: scalar indexes have to be wrapped in array brackets if you want an array as output. Here some examples:

```js
parser = math.parser()
parser.evaluate('m = [1, 2, 3; 4, 5, 6]')
```

| v14 code                | v15 equivalent code           | Result             |
|-------------------------|-------------------------------|--------------------|
| `m[1:2, 2:3]`           | No change needed              | `[[2, 3], [5, 6]]` |
| `m[2, 2:3]`             | `m[[2], 2:3]`                 | `[[5, 6]]`         |
| `m[1:2, 3]`             | `m[1:2, [3]]`                 | `[[3], [6]]`       |
| `m[2, 3]`               | No change needed              | `6`                |

> **Tip:**  
> If you want to always get a scalar value, use scalar indices.  
> If you want to preserve dimensions, use array, matrix or range indices.


## Expression syntax: Objects

Objects in math.js work the same as in languages like JavaScript and Python.
An object is enclosed by curly brackets `{`, `}`, and contains a set of
comma separated key/value pairs. Keys and values are separated by a colon `:`.
Keys can be a symbol like `prop` or a string like `"prop"`.

```js
math.evaluate('{a: 2 + 1, b: 4}')         // {a: 3, b: 4}
math.evaluate('{"a": 2 + 1, "b": 4}')     // {a: 3, b: 4}
```

Objects can contain objects:

```js
math.evaluate('{a: 2, b: {c: 3, d: 4}}')  // {a: 2, b: {c: 3, d: 4}}
```

Object properties can be retrieved or replaced using dot notation or bracket
notation. Like in JavaScript, setting a property value returns the assigned
value, not the whole object

```js
let scope = {
  obj: {
    prop: 42
  }
}

// retrieve properties
math.evaluate('obj.prop', scope)          // 42
math.evaluate('obj["prop"]', scope)       // 42

// set properties (returns the assigned value, not the whole object)
math.evaluate('obj.prop = 43', scope)     // 43
math.evaluate('obj["prop"] = 43', scope)  // 43
scope.obj                                 // {prop: 43}
```


## Expression syntax: Multi-line expressions

An expression can contain multiple lines, and expressions can be spread over
multiple lines. Lines can be separated by a newline character `\n` or by a
semicolon `;`. Output of statements followed by a semicolon will be hidden from
the output, and empty lines are ignored. The output is returned as a `ResultSet`,
with an entry for every visible statement.

```js
// a multi-line expression
math.evaluate('1 * 3 \n 2 * 3 \n 3 * 3')    // ResultSet, [3, 6, 9]

// semicolon statements are hidden from the output
math.evaluate('a=3; b=4; a + b \n a * b')   // ResultSet, [7, 12]

// single expression spread over multiple lines
math.evaluate('a = 2 +\n  3')               // 5
math.evaluate('[\n  1, 2;\n  3, 4\n]')      // Matrix, [[1, 2], [3, 4]]
```

The results can be read from a `ResultSet` via the property `ResultSet.entries`
which is an `Array`, or by calling `ResultSet.valueOf()`, which returns the
array with results.

### Empty expressions

The empty string and any expression consisting solely of whitespace denotes the
value `undefined`. Such an expression can also occur in a multi-expression
block separated by `;`, in which case it is simply ignored. (In a
multi-expression block separated by newlines, empty or whitespace-only
substrings between newlines are absorbed into the separators, and so are not
expressions at all.) In all other cases, such as one of the clauses of a
conditional ternary operator, an empty expression is a syntax error.

## Expression syntax: Implicit multiplication

*Implicit multiplication* means the multiplication of two symbols, numbers, or a grouped expression inside parentheses without using the `*` operator. This type of syntax allows a more natural way to enter expressions. For example:

```js
math.evaluate('2 pi')         // 6.283185307179586
math.evaluate('(1+2)(3+4)')   // 21
```

Parentheses are parsed as a function call when there is a symbol or accessor on
the left hand side, like `sqrt(4)` or `obj.method(4)`. In other cases the
parentheses are interpreted as an implicit multiplication.

Math.js will always evaluate implicit multiplication before explicit multiplication `*`, so that the expression `x * y z` is parsed as `x * (y * z)`. Math.js also gives implicit multiplication higher precedence than division, *except* when the division matches the pattern `[unaryPrefixOp]?[number] / [number] [symbol]` or `[unaryPrefixOp]?[number] / [number] [left paren]`. In that special case, the division is evaluated first:

```js
math.evaluate('20 kg / 4 kg')   // 5      Evaluated as (20 kg) / (4 kg)
math.evaluate('20 / 4 kg')      // 5 kg   Evaluated as (20 / 4) kg
```

The behavior of implicit multiplication can be summarized by these operator precedence rules, listed from highest to lowest precedence:

- Function calls: `[symbol] [left paren]`
- Explicit division `/` when the division matches this pattern: `[+-~]?[number] / [+-~]?[number] [symbol]` or `[number] / [number] [left paren]`
- Implicit multiplication
- All other division `/` and multiplication `*`

Implicit multiplication is tricky as there can appear to be ambiguity in how an expression will be evaluated. Experience has shown that the above rules most closely match user intent when entering expressions that could be interpreted different ways. It's also possible that these rules could be tweaked in future major releases.  Use implicit multiplication carefully. If you don't like the uncertainty introduced by implicit multiplication, use explicit `*` operators and parentheses to ensure your expression is evaluated the way you intend.

Here are some more examples using implicit multiplication:

Expression      | Evaluated as        | Result
--------------- | ------------------- | ------------------
(1 + 3) pi      | (1 + 3) * pi        | 12.566370614359172
(4 - 1) 2       | (4 - 1) * 2         | 6
3 / 4 mm        | (3 / 4) * mm        | 0.75 mm
2 + 3 i         | 2 + (3 * i)         | 2 + 3i
(1 + 2) (4 - 2) | (1 + 2) * (4 - 2)   | 6
sqrt(4) (1 + 2) | sqrt(4) * (1 + 2)   | 6
8 pi / 2 pi     | (8 * pi) / (2 * pi) | 4
pi / 2 pi       | pi / (2 * pi)       | 0.5
1 / 2i          | (1 / 2) * i         | 0.5 i
8.314 J / mol K | 8.314 J / (mol * K) | 8.314 J / (mol * K)


## Expression syntax: Comments

Comments can be added to explain or describe calculations in the text. A comment
starts with a sharp sign character `#`, and ends at the end of the line. A line
can contain a comment only, or can contain an expression followed by a comment.

```js
const parser = math.parser()

parser.evaluate('# define some variables')
parser.evaluate('width = 3')                              // 3
parser.evaluate('height = 4')                             // 4
parser.evaluate('width * height   # calculate the area')  // 12
```


## Getting Started

This getting started describes how to install, load, and use math.js.


## Getting Started: Install

Not for this drive: Math.js can be installed using various package managers like [npm](https://npmjs.org/), or by just downloading the library from the website: [https://mathjs.org/download.html](https://mathjs.org/download.html).

Not for this drive: To install via npm, run:

Not for this drive:     npm install mathjs

Other ways to install math.js are described on the [website](https://mathjs.org/download.html).


## Getting Started: Load

Math.js can be used in node.js and in the browser. The library must be loaded
and instantiated. When creating an instance, one can optionally provide
configuration options as described in
[Configuration](core/configuration.md).

### ES modules

Load the functions you need and use them:

```js
import { sqrt } from 'mathjs'

console.log(sqrt(-4).toString()) // 2i
```

To use lightweight, number only implementations of all functions:

```js
import { sqrt } from 'mathjs/number'

console.log(sqrt(4).toString()) // 2
console.log(sqrt(-4).toString()) // NaN
```

You can create a mathjs instance allowing [configuration](core/configuration.md) and importing of external functions as follows:

```js
import { create, all } from 'mathjs'

const config = { }
const math = create(all, config)

console.log(math.sqrt(-4).toString()) // 2i
```

How to optimize your bundle size using tree-shaking is described on the page
[Custom bundling](custom_bundling.md).


### Node.js

Load math.js in [node.js](https://nodejs.org/) (CommonJS module system):

```js
const { sqrt } = require('mathjs')

console.log(sqrt(-4).toString()) // 2i
```


### Browser

Math.js can be loaded as a regular JavaScript file in the browser, use the global
variable `math` to access the libary once loaded:

```html
<!DOCTYPE HTML>
<html>
<head>
  
</head>
<body>
  
</body>
</html>
```

## Getting Started: Use

Math.js can be used similar to JavaScript's built-in Math library. Besides that,
math.js can evaluate expressions (see [Expressions](expressions/index.md)) and
supports chaining (see [Chaining](core/chaining.md)).

The example code below shows how to use math.js. More examples can be found in the
section [Examples](https://mathjs.org/examples/index.html).

```js
// functions and constants
math.round(math.e, 3)                // 2.718
math.atan2(3, -3) / math.pi          // 0.75
math.log(10000, 10)                  // 4
math.sqrt(-4)                        // 2i
math.pow([[-1, 2], [3, 1]], 2)       // [[7, 0], [0, 7]]

// expressions
math.evaluate('12 / (2.3 + 0.7)')    // 4
math.evaluate('12.7 cm to inch')     // 5 inch
math.evaluate('sin(45 deg) ^ 2')     // 0.5
math.evaluate('9 / 3 + 2i')          // 3 + 2i
math.evaluate('det([-1, 2; 3, 1])')  // -7

// chained operations
math.chain(3)
    .add(4)
    .multiply(2)
    .done() // 14
```

## Getting Started: Next

To learn more about math.js, check out the available documentation and examples:

- [Documentation](index.md)
- [Examples](https://mathjs.org/examples/index.html)


## Documentation

[Math.js](https://mathjs.org) is an extensive math library for JavaScript and Node.js. It features a flexible expression parser with support for symbolic computation, comes with a large set of built-in functions and constants, and offers an integrated solution to work with different data types like numbers, big numbers, complex numbers, fractions, units, and matrices.

Math.js can be used in the browser, in node.js and in any JavaScript engine. Installation and download instructions are available on the [Download page](https://mathjs.org/download.html) of the website.

## Documentation: Getting Started

- [Getting Started](getting_started.md)
- [Examples](//mathjs.org/examples/index.html)

## Documentation: Documentation

- **[Core](core/index.md)**
  - [Configuration](core/configuration.md)
  - [Chaining](core/chaining.md)
  - [Extension](core/extension.md)
  - [Serialization](core/serialization.md)
- **[Expressions](expressions/index.md)**
  - [Parsing and evaluation](expressions/parsing.md)
  - [Syntax](expressions/syntax.md)
  - [Expression trees](expressions/expression_trees.md)
  - [Algebra](expressions/algebra.md)
  - [Customization](expressions/customization.md)
  - [Security](expressions/security.md)
- **[Data Types](datatypes/index.md)**
  - [Booleans](datatypes/booleans.md)
  - [Numbers](datatypes/numbers.md)
  - [BigNumbers](datatypes/bignumbers.md)
  - [bigints](datatypes/bigints.md)
  - [Fractions](datatypes/fractions.md)
  - [Complex Numbers](datatypes/complex_numbers.md)
  - [Matrices](datatypes/matrices.md)
  - [Units](datatypes/units.md)
  - [Strings](datatypes/strings.md)
- **[Reference](reference/index.md)**
  - [Classes](reference/classes.md)
  - [Functions](reference/functions.md)
  - [Constants](reference/constants.md)
- [Custom bundling](custom_bundling.md)
- [Command Line Interface](command_line_interface.md)
- [History](../HISTORY.md)


## Class Reference

This page lists all the various class types in Math.js. Every top-level function is listed here and links to its detailed reference to other parts of the documentation.

## Class Reference: math

The "math" namespace contains the entire math.js functionality. All of the mathematical functions are available in the "math" namespace, and allow for inputs of various types.

- [Function reference](functions.md)
- [Constant reference](constants.md)


## Class Reference: Unit

Stores values for a scalar unit and its postfix. (eg `100 mm` or `100 kg`). Although the `Unit` class contains public functions documented as follows, using the following API directly is *not* recommended. Prefer using the functions in the "math" namespace wherever possible.

- [Overview](../datatypes/units.md)
- [Class API](classes/unit.md)


## Class Reference: Fraction

Stores values for a fractional number.

- [Overview](../datatypes/fractions.md)
- [Class API](https://github.com/rawify/Fraction.js)

## Class Reference: BigNumber

Stores values for a arbitrary-precision floating point number.

- [Overview](../datatypes/bignumbers.md)
- [Class API](https://mikemcl.github.io/decimal.js/)


## Class Reference: Matrix

Two types of matrix classes are available in math.js, for storage of dense and sparse matrices. Although they contain public functions documented as follows, using the following API directly is *not* recommended. Prefer using the functions in the "math" namespace wherever possible.

- [Overview](../datatypes/matrices.md)
- [DenseMatrix](classes/densematrix.md)
- [SparseMatrix](classes/sparsematrix.md)

Classes used internally that may be of use to developers:

- [Index](classes/matrixindex.md)
- [Range](classes/matrixrange.md)
- [ResultSet](classes/resultset.md)
- [FibonacciHeap](classes/fibonacciheap.md)

## Class Reference: Complex

Stores values for a complex number.

- [Overview](../datatypes/complex_numbers.md)
- [Class API](https://github.com/infusion/Complex.js/)

## Class Reference: Parser

The Parser object returned by `math.parser()`.

- [Overview](../expressions/parsing.md)

## Class Reference: Node

A node in an expression-tree, which can be used to analyze, manipulate, and evaluate expressions.

- [Overview](../expressions/expression_trees.md)

`Node` is the base class of all other node classes:

- [AccessorNode](../expressions/expression_trees.md#accessornode)
- [ArrayNode](../expressions/expression_trees.md#arraynode)
- [AssignmentNode](../expressions/expression_trees.md#assignmentnode)
- [BlockNode](../expressions/expression_trees.md#blocknode)
- [ConditionalNode](../expressions/expression_trees.md#conditionalnode)
- [ConstantNode](../expressions/expression_trees.md#constantnode)
- [FunctionAssignmentNode](../expressions/expression_trees.md#functionassignmentnode)
- [FunctionNode](../expressions/expression_trees.md#functionnode)
- [IndexNode](../expressions/expression_trees.md#indexnode)
- [ObjectNode](../expressions/expression_trees.md#objectnode)
- [OperatorNode](../expressions/expression_trees.md#operatornode)
- [ParenthesisNode](../expressions/expression_trees.md#parenthesisnode)
- [RangeNode](../expressions/expression_trees.md#rangenode)
- [SymbolNode](../expressions/expression_trees.md#symbolnode)


## densematrix

<a name="DenseMatrix"></a>
## densematrix: DenseMatrix
Dense Matrix implementation. This type implements an efficient Array format
for dense matrices.

* _instance_
	* [.storage()](#DenseMatrix+storage) ⇒ <code>string</code>
	* [.datatype()](#DenseMatrix+datatype) ⇒ <code>string</code>
	* [.create(data, [datatype])](#DenseMatrix+create)
	* [.subset(index, [replacement], [defaultValue])](#DenseMatrix+subset)
	* [.get(index)](#DenseMatrix+get) ⇒ <code>\*</code>
	* [.set(index, value, [defaultValue])](#DenseMatrix+set) ⇒ <code>DenseMatrix</code>
	* [.resize(size, [defaultValue], [copy])](#DenseMatrix+resize) ⇒ <code>Matrix</code>
	* [.clone()](#DenseMatrix+clone) ⇒ <code>DenseMatrix</code>
	* [.size()](#DenseMatrix+size) ⇒ <code>Array.&lt;number&gt;</code>
	* [.map(callback)](#DenseMatrix+map) ⇒ <code>DenseMatrix</code>
	* [.forEach(callback)](#DenseMatrix+forEach)
	* [.toArray()](#DenseMatrix+toArray) ⇒ <code>Array</code>
	* [.valueOf()](#DenseMatrix+valueOf) ⇒ <code>Array</code>
	* [.format([options])](#DenseMatrix+format) ⇒ <code>string</code>
	* [.toString()](#DenseMatrix+toString) ⇒ <code>string</code>
	* [.toJSON()](#DenseMatrix+toJSON) ⇒ <code>Object</code>
	* [.diagonal([k])](#DenseMatrix+diagonal) ⇒ <code>Array</code>
	* [.swapRows(i, j)](#DenseMatrix+swapRows) ⇒ <code>Matrix</code>
* _static_
	* [.diagonal(size, value, [k], [defaultValue])](#DenseMatrix.diagonal) ⇒ <code>DenseMatrix</code>
	* [.fromJSON(json)](#DenseMatrix.fromJSON) ⇒ <code>DenseMatrix</code>
	* [.preprocess(data)](#DenseMatrix.preprocess) ⇒ <code>Array</code>

<a name="DenseMatrix+storage"></a>
### denseMatrix.storage() ⇒ <code>string</code>
Get the storage format used by the matrix.

Usage:

```js
const format = matrix.storage()                   // retrieve storage format
```

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>string</code> - The storage format.  
<a name="DenseMatrix+datatype"></a>
### denseMatrix.datatype() ⇒ <code>string</code>
Get the datatype of the data stored in the matrix.

Usage:

```js
const format = matrix.datatype()                   // retrieve matrix datatype
```

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>string</code> - The datatype.  
<a name="DenseMatrix+create"></a>
### denseMatrix.create(data, [datatype])
Create a new DenseMatrix

**Kind**: instance method of <code>DenseMatrix</code> 

| Param | Type |
| --- | --- |
| data | <code>Array</code> | 
| [datatype] | <code>string</code> | 

<a name="DenseMatrix+subset"></a>
### denseMatrix.subset(index, [replacement], [defaultValue])
Get a subset of the matrix, or replace a subset of the matrix.

Usage:

```js
const subset = matrix.subset(index)               // retrieve subset
const value = matrix.subset(index, replacement)   // replace subset
```

**Kind**: instance method of <code>DenseMatrix</code> 

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| index | <code>Index</code> |  |  |
| [replacement] | <code>Array</code> &#124; <code>DenseMatrix</code>&#124; <code>\*</code> |  |  |
| [defaultValue] | <code>\*</code> | <code>0</code> | Default value, filled in on new entries when                                  the matrix is resized. If not provided,                                  new matrix elements will be filled with zeros. |

<a name="DenseMatrix+get"></a>
### denseMatrix.get(index) ⇒ <code>\*</code>
Get a single element from the matrix.

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>\*</code> - value  

| Param | Type | Description |
| --- | --- | --- |
| index | <code>Array.&lt;number&gt;</code> | Zero-based index |

<a name="DenseMatrix+set"></a>
### denseMatrix.set(index, value, [defaultValue]) ⇒ <code>DenseMatrix</code>
Replace a single element in the matrix.

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>DenseMatrix</code>- self  

| Param | Type | Description |
| --- | --- | --- |
| index | <code>Array.&lt;number&gt;</code> | Zero-based index |
| value | <code>\*</code> |  |
| [defaultValue] | <code>\*</code> | Default value, filled in on new entries when                                  the matrix is resized. If not provided,                                  new matrix elements will be left undefined. |

<a name="DenseMatrix+resize"></a>
### denseMatrix.resize(size, [defaultValue], [copy]) ⇒ <code>Matrix</code>
Resize the matrix to the given size. Returns a copy of the matrix when
`copy=true`, otherwise return the matrix itself (resize in place).

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>Matrix</code> - The resized matrix  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| size | <code>Array.&lt;number&gt;</code> |  | The new size the matrix should have. |
| [defaultValue] | <code>\*</code> | <code>0</code> | Default value, filled in on new entries.                                  If not provided, the matrix elements will                                  be filled with zeros. |
| [copy] | <code>boolean</code> |  | Return a resized copy of the matrix |

<a name="DenseMatrix+clone"></a>
### denseMatrix.clone() ⇒ <code>DenseMatrix</code>
Create a clone of the matrix

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>DenseMatrix</code>- clone  
<a name="DenseMatrix+size"></a>
### denseMatrix.size() ⇒ <code>Array.&lt;number&gt;</code>
Retrieve the size of the matrix.

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>Array.&lt;number&gt;</code> - size  
<a name="DenseMatrix+map"></a>
### denseMatrix.map(callback) ⇒ <code>DenseMatrix</code>
Create a new matrix with the results of the callback function executed on
each entry of the matrix.

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>DenseMatrix</code>- matrix  

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | The callback function is invoked with three                              parameters: the value of the element, the index                              of the element, and the Matrix being traversed. |

<a name="DenseMatrix+forEach"></a>
### denseMatrix.forEach(callback)
Execute a callback function on each entry of the matrix.

**Kind**: instance method of <code>DenseMatrix</code> 

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | The callback function is invoked with three                              parameters: the value of the element, the index                              of the element, and the Matrix being traversed. |

<a name="DenseMatrix+toArray"></a>
### denseMatrix.toArray() ⇒ <code>Array</code>
Create an Array with a copy of the data of the DenseMatrix

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>Array</code> - array  
<a name="DenseMatrix+valueOf"></a>
### denseMatrix.valueOf() ⇒ <code>Array</code>
Get the primitive value of the DenseMatrix: a multidimensional array

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>Array</code> - array  
<a name="DenseMatrix+format"></a>
### denseMatrix.format([options]) ⇒ <code>string</code>
Get a string representation of the matrix, with optional formatting options.

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>string</code> - str  

| Param | Type | Description |
| --- | --- | --- |
| [options] | <code>Object</code> &#124; <code>number</code> &#124; <code>function</code> | Formatting options. See                                                lib/utils/number:format for a                                                description of the available                                                options. |

<a name="DenseMatrix+toString"></a>
### denseMatrix.toString() ⇒ <code>string</code>
Get a string representation of the matrix

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>string</code> - str  
<a name="DenseMatrix+toJSON"></a>
### denseMatrix.toJSON() ⇒ <code>Object</code>
Get a JSON representation of the matrix

**Kind**: instance method of <code>DenseMatrix</code> 
<a name="DenseMatrix+diagonal"></a>
### denseMatrix.diagonal([k]) ⇒ <code>Array</code>
Get the kth Matrix diagonal.

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>Array</code> - The array vector with the diagonal values.  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| [k] | <code>number</code> &#124; <code>BigNumber</code> | <code>0</code> | The kth diagonal where the vector will retrieved. |

<a name="DenseMatrix+swapRows"></a>
### denseMatrix.swapRows(i, j) ⇒ <code>Matrix</code>
Swap rows i and j in Matrix.

**Kind**: instance method of <code>DenseMatrix</code> 
**Returns**: <code>Matrix</code> - The matrix reference  

| Param | Type | Description |
| --- | --- | --- |
| i | <code>number</code> | Matrix row index 1 |
| j | <code>number</code> | Matrix row index 2 |

<a name="DenseMatrix.diagonal"></a>
### DenseMatrix.diagonal(size, value, [k], [defaultValue]) ⇒ <code>DenseMatrix</code>
Create a diagonal matrix.

**Kind**: static method of <code>DenseMatrix</code> 

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| size | <code>Array</code> |  | The matrix size. |
| value | <code>number</code> &#124; <code>Array</code> |  | The values for the diagonal. |
| [k] | <code>number</code> &#124; <code>BigNumber</code> | <code>0</code> | The kth diagonal where the vector will be filled in. |
| [defaultValue] | <code>number</code> |  | The default value for non-diagonal |

<a name="DenseMatrix.fromJSON"></a>
### DenseMatrix.fromJSON(json) ⇒ <code>DenseMatrix</code>
Generate a matrix from a JSON object

**Kind**: static method of <code>DenseMatrix</code> 

| Param | Type | Description |
| --- | --- | --- |
| json | <code>Object</code> | An object structured like                       `{"mathjs": "DenseMatrix", data: [], size: []}`,                       where mathjs is optional |

<a name="DenseMatrix.preprocess"></a>
### DenseMatrix.preprocess(data) ⇒ <code>Array</code>
Preprocess data, which can be an Array or DenseMatrix with nested Arrays and
Matrices. Replaces all nested Matrices with Arrays

**Kind**: static method of <code>DenseMatrix</code> 
**Returns**: <code>Array</code> - data  

| Param | Type |
| --- | --- |
| data | <code>Array</code> | 



## fibonacciheap

<a name="FibonacciHeap"></a>
## fibonacciheap: FibonacciHeap
* [new FibonacciHeap()](#new_FibonacciHeap_new)
* _instance_
	* [.insert()](#FibonacciHeap+insert)
	* [.size()](#FibonacciHeap+size)
	* [.clear()](#FibonacciHeap+clear)
	* [.isEmpty()](#FibonacciHeap+isEmpty)
	* [.extractMinimum()](#FibonacciHeap+extractMinimum)
	* [.remove()](#FibonacciHeap+remove)
* _static_
	* [._decreaseKey()](#FibonacciHeap._decreaseKey)
	* [._cut()](#FibonacciHeap._cut)
	* [._cascadingCut()](#FibonacciHeap._cascadingCut)
	* [._linkNodes()](#FibonacciHeap._linkNodes)

<a name="new_FibonacciHeap_new"></a>
### new FibonacciHeap()
Creates a new instance of a Fibonacci Heap.

<a name="FibonacciHeap+insert"></a>
### fibonacciHeap.insert()
Inserts a new data element into the heap. No heap consolidation is
performed at this time, the new node is simply inserted into the root
list of this heap. Running time: O(1) actual.

**Kind**: instance method of <code>[FibonacciHeap](#FibonacciHeap)</code>  
<a name="FibonacciHeap+size"></a>
### fibonacciHeap.size()
Returns the number of nodes in heap. Running time: O(1) actual.

**Kind**: instance method of <code>[FibonacciHeap](#FibonacciHeap)</code>  
<a name="FibonacciHeap+clear"></a>
### fibonacciHeap.clear()
Removes all elements from this heap.

**Kind**: instance method of <code>[FibonacciHeap](#FibonacciHeap)</code>  
<a name="FibonacciHeap+isEmpty"></a>
### fibonacciHeap.isEmpty()
Returns true if the heap is empty, otherwise false.

**Kind**: instance method of <code>[FibonacciHeap](#FibonacciHeap)</code>  
<a name="FibonacciHeap+extractMinimum"></a>
### fibonacciHeap.extractMinimum()
Extracts the node with minimum key from heap. Amortized running 
time: O(log n).

**Kind**: instance method of <code>[FibonacciHeap](#FibonacciHeap)</code>  
<a name="FibonacciHeap+remove"></a>
### fibonacciHeap.remove()
Removes a node from the heap given the reference to the node. The trees
in the heap will be consolidated, if necessary. This operation may fail
to remove the correct element if there are nodes with key value -Infinity.
Running time: O(log n) amortized.

**Kind**: instance method of <code>[FibonacciHeap](#FibonacciHeap)</code>  
<a name="FibonacciHeap._decreaseKey"></a>
### FibonacciHeap._decreaseKey()
Decreases the key value for a heap node, given the new value to take on.
The structure of the heap may be changed and will not be consolidated. 
Running time: O(1) amortized.

**Kind**: static method of <code>[FibonacciHeap](#FibonacciHeap)</code>  
<a name="FibonacciHeap._cut"></a>
### FibonacciHeap._cut()
The reverse of the link operation: removes node from the child list of parent.
This method assumes that min is non-null. Running time: O(1).

**Kind**: static method of <code>[FibonacciHeap](#FibonacciHeap)</code>  
<a name="FibonacciHeap._cascadingCut"></a>
### FibonacciHeap._cascadingCut()
Performs a cascading cut operation. This cuts node from its parent and then
does the same for its parent, and so on up the tree.
Running time: O(log n); O(1) excluding the recursion.

**Kind**: static method of <code>[FibonacciHeap](#FibonacciHeap)</code>  
<a name="FibonacciHeap._linkNodes"></a>
### FibonacciHeap._linkNodes()
Make the first node a child of the second one. Running time: O(1) actual.

**Kind**: static method of <code>[FibonacciHeap](#FibonacciHeap)</code>  


## matrixindex

<a name="Index"></a>
## matrixindex: Index
* [new Index(...ranges)](#new_Index_new)
* _instance_
	* [.valueOf](#Index+valueOf) ⇒ <code>Array</code>
	* [.clone()](#Index+clone) ⇒ <code>[Index](#Index)</code>
	* [.size()](#Index+size) ⇒ <code>Array.&lt;number&gt;</code>
	* [.max()](#Index+max) ⇒ <code>Array.&lt;number&gt;</code>
	* [.min()](#Index+min) ⇒ <code>Array.&lt;number&gt;</code>
	* [.forEach(callback)](#Index+forEach)
	* [.dimension(dim)](#Index+dimension) ⇒ <code>Range</code> &#124; <code>null</code>
	* [.isScalar()](#Index+isScalar) ⇒ <code>boolean</code>
	* [.toArray()](#Index+toArray) ⇒ <code>Array</code>
	* [.toString()](#Index+toString) ⇒ <code>String</code>
	* [.toJSON()](#Index+toJSON) ⇒ <code>Object</code>
* _static_
	* [.fromJSON(json)](#Index.fromJSON) ⇒ <code>[Index](#Index)</code>

<a name="new_Index_new"></a>
### new Index(...ranges)
Create an index. An Index can store ranges and sets for multiple dimensions.
Matrix.get, Matrix.set, and math.subset accept an Index as input.

Usage:
```js
const index = new Index(range1, range2, matrix1, array1, ...)
```

Where each parameter can be any of:

- A number
- An instance of Range
- An Array with the Set values
- A Matrix with the Set values

The parameters start, end, and step must be integer numbers.


| Param | Type |
| --- | --- |
| ...ranges | <code>\*</code> | 

<a name="Index+valueOf"></a>
### index.valueOf ⇒ <code>Array</code>
Get the primitive value of the Index, a two dimensional array.
Equivalent to Index.toArray().

**Kind**: instance property of <code>[Index](#Index)</code>  
**Returns**: <code>Array</code> - array  
<a name="Index+clone"></a>
### index.clone() ⇒ <code>[Index](#Index)</code>
Create a clone of the index

**Kind**: instance method of <code>[Index](#Index)</code>  
**Returns**: <code>[Index](#Index)</code> - clone  
<a name="Index+size"></a>
### index.size() ⇒ <code>Array.&lt;number&gt;</code>
Retrieve the size of the index, the number of elements for each dimension.

**Kind**: instance method of <code>[Index](#Index)</code>  
**Returns**: <code>Array.&lt;number&gt;</code> - size  
<a name="Index+max"></a>
### index.max() ⇒ <code>Array.&lt;number&gt;</code>
Get the maximum value for each of the indexes ranges.

**Kind**: instance method of <code>[Index](#Index)</code>  
**Returns**: <code>Array.&lt;number&gt;</code> - max  
<a name="Index+min"></a>
### index.min() ⇒ <code>Array.&lt;number&gt;</code>
Get the minimum value for each of the indexes ranges.

**Kind**: instance method of <code>[Index](#Index)</code>  
**Returns**: <code>Array.&lt;number&gt;</code> - min  
<a name="Index+forEach"></a>
### index.forEach(callback)
Loop over each of the ranges of the index

**Kind**: instance method of <code>[Index](#Index)</code>  

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | Called for each range with a Range as first                              argument, the dimension as second, and the                              index object as third. |

<a name="Index+dimension"></a>
### index.dimension(dim) ⇒ <code>Range</code> &#124; <code>null</code>
Retrieve the dimension for the given index

**Kind**: instance method of <code>[Index](#Index)</code>  
**Returns**: <code>Range</code> &#124; <code>null</code> - range  

| Param | Type | Description |
| --- | --- | --- |
| dim | <code>Number</code> | Number of the dimension |

<a name="Index+isScalar"></a>
### index.isScalar() ⇒ <code>boolean</code>
Test whether this index contains only a single value.

This is the case when the index is created with only scalar values as ranges,
not for ranges resolving into a single value.

**Kind**: instance method of <code>[Index](#Index)</code>  
**Returns**: <code>boolean</code> - isScalar  
<a name="Index+toArray"></a>
### index.toArray() ⇒ <code>Array</code>
Expand the Index into an array.
For example new Index([0,3], [2,7]) returns [[0,1,2], [2,3,4,5,6]]

**Kind**: instance method of <code>[Index](#Index)</code>  
**Returns**: <code>Array</code> - array  
<a name="Index+toString"></a>
### index.toString() ⇒ <code>String</code>
Get the string representation of the index, for example '[2:6]' or '[0:2:10, 4:7, [1,2,3]]'

**Kind**: instance method of <code>[Index](#Index)</code>  
**Returns**: <code>String</code> - str  
<a name="Index+toJSON"></a>
### index.toJSON() ⇒ <code>Object</code>
Get a JSON representation of the Index

**Kind**: instance method of <code>[Index](#Index)</code>  
**Returns**: <code>Object</code> - Returns a JSON object structured as:
                  `{"mathjs": "Index", "ranges": [{"mathjs": "Range", start: 0, end: 10, step:1}, ...]}`  
<a name="Index.fromJSON"></a>
### Index.fromJSON(json) ⇒ <code>[Index](#Index)</code>
Instantiate an Index from a JSON object

**Kind**: static method of <code>[Index](#Index)</code>  

| Param | Type | Description |
| --- | --- | --- |
| json | <code>Object</code> | A JSON object structured as:                     `{"mathjs": "Index", "dimensions": [{"mathjs": "Range", start: 0, end: 10, step:1}, ...]}` |



## matrixrange

<a name="Range"></a>
## matrixrange: Range
* [new Range(start, end, [step])](#new_Range_new)
* _instance_
	* [.size()](#Range+size) ⇒ <code>Array.&lt;number&gt;</code>
	* [.min()](#Range+min) ⇒ <code>number</code> &#124; <code>undefined</code>
	* [.max()](#Range+max) ⇒ <code>number</code> &#124; <code>undefined</code>
	* [.forEach(callback)](#Range+forEach)
	* [.map(callback)](#Range+map) ⇒ <code>Array</code>
	* [.toArray()](#Range+toArray) ⇒ <code>Array</code>
	* [.valueOf()](#Range+valueOf) ⇒ <code>Array</code>
	* [.format([options])](#Range+format) ⇒ <code>string</code>
	* [.toString()](#Range+toString) ⇒ <code>string</code>
	* [.toJSON()](#Range+toJSON) ⇒ <code>Object</code>
* _static_
	* [.parse(str)](#Range.parse) ⇒ <code>[Range](#Range)</code> &#124; <code>null</code>
	* [.fromJSON(json)](#Range.fromJSON) ⇒ <code>[Range](#Range)</code>

<a name="new_Range_new"></a>
### new Range(start, end, [step])
Create a range. A range has a start, step, and end, and contains functions
to iterate over the range.

A range can be constructed as:

```js
const range = new Range(start, end) 
const range = new Range(start, end, step) 
```

To get the result of the range:

```js
range.forEach(function (x) {
    console.log(x) 
}) 
range.map(function (x) {
    return math.sin(x) 
}) 
range.toArray() 
```

Example usage:

```js
const c = new Range(2, 6)        // 2:1:5
c.toArray()                      // [2, 3, 4, 5]
const d = new Range(2, -3, -1)   // 2:-1:-2
d.toArray()                      // [2, 1, 0, -1, -2]
```

| Param | Type | Description |
| --- | --- | --- |
| start | <code>number</code> | included lower bound |
| end | <code>number</code> | excluded upper bound |
| [step] | <code>number</code> | step size, default value is 1 |

<a name="Range+size"></a>
### range.size() ⇒ <code>Array.&lt;number&gt;</code>
Retrieve the size of the range.
Returns an array containing one number, the number of elements in the range.

**Kind**: instance method of <code>[Range](#Range)</code>  
**Returns**: <code>Array.&lt;number&gt;</code> - size  
<a name="Range+min"></a>
### range.min() ⇒ <code>number</code> &#124; <code>undefined</code>
Calculate the minimum value in the range

**Kind**: instance method of <code>[Range](#Range)</code>  
**Returns**: <code>number</code> &#124; <code>undefined</code> - min  
<a name="Range+max"></a>
### range.max() ⇒ <code>number</code> &#124; <code>undefined</code>
Calculate the maximum value in the range

**Kind**: instance method of <code>[Range](#Range)</code>  
**Returns**: <code>number</code> &#124; <code>undefined</code> - max  
<a name="Range+forEach"></a>
### range.forEach(callback)
Execute a callback function for each value in the range.

**Kind**: instance method of <code>[Range](#Range)</code>  

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | The callback method is invoked with three                              parameters: the value of the element, the index                              of the element, and the Range being traversed. |

<a name="Range+map"></a>
### range.map(callback) ⇒ <code>Array</code>
Execute a callback function for each value in the Range, and return the
results as an array

**Kind**: instance method of <code>[Range](#Range)</code>  
**Returns**: <code>Array</code> - array  

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | The callback method is invoked with three                              parameters: the value of the element, the index                              of the element, and the Matrix being traversed. |

<a name="Range+toArray"></a>
### range.toArray() ⇒ <code>Array</code>
Create an Array with a copy of the Ranges data

**Kind**: instance method of <code>[Range](#Range)</code>  
**Returns**: <code>Array</code> - array  
<a name="Range+valueOf"></a>
### range.valueOf() ⇒ <code>Array</code>
Get the primitive value of the Range, a one dimensional array

**Kind**: instance method of <code>[Range](#Range)</code>  
**Returns**: <code>Array</code> - array  
<a name="Range+format"></a>
### range.format([options]) ⇒ <code>string</code>
Get a string representation of the range, with optional formatting options.
Output is formatted as 'start:step:end', for example '2:6' or '0:0.2:11'

**Kind**: instance method of <code>[Range](#Range)</code>  
**Returns**: <code>string</code> - str  

| Param | Type | Description |
| --- | --- | --- |
| [options] | <code>Object</code> &#124; <code>number</code> &#124; <code>function</code> | Formatting options. See                                                lib/utils/number:format for a                                                description of the available                                                options. |

<a name="Range+toString"></a>
### range.toString() ⇒ <code>string</code>
Get a string representation of the range.

**Kind**: instance method of <code>[Range](#Range)</code>  
<a name="Range+toJSON"></a>
### range.toJSON() ⇒ <code>Object</code>
Get a JSON representation of the range

**Kind**: instance method of <code>[Range](#Range)</code>  
**Returns**: <code>Object</code> - Returns a JSON object structured as:
                  `{"mathjs": "Range", "start": 2, "end": 4, "step": 1}`  
<a name="Range.parse"></a>
### Range.parse(str) ⇒ <code>[Range](#Range)</code> &#124; <code>null</code>
Parse a string into a range,
The string contains the start, optional step, and end, separated by a colon.
If the string does not contain a valid range, null is returned.
For example str='0:2:11'.

**Kind**: static method of <code>[Range](#Range)</code>  
**Returns**: <code>[Range](#Range)</code> &#124; <code>null</code> - range  

| Param | Type |
| --- | --- |
| str | <code>string</code> | 

<a name="Range.fromJSON"></a>
### Range.fromJSON(json) ⇒ <code>[Range](#Range)</code>
Instantiate a Range from a JSON object

**Kind**: static method of <code>[Range](#Range)</code>  

| Param | Type | Description |
| --- | --- | --- |
| json | <code>Object</code> | A JSON object structured as:                      `{"mathjs": "Range", "start": 2, "end": 4, "step": 1}` |



## resultset

<a name="ResultSet"></a>
## resultset: ResultSet
* [new ResultSet(entries)](#new_ResultSet_new)
* _instance_
	* [.valueOf()](#ResultSet+valueOf) ⇒ <code>Array</code>
	* [.toString()](#ResultSet+toString) ⇒ <code>string</code>
	* [.toJSON()](#ResultSet+toJSON) ⇒ <code>Object</code>
* _static_
	* [.fromJSON(json)](#ResultSet.fromJSON) ⇒ <code>[ResultSet](#ResultSet)</code>

<a name="new_ResultSet_new"></a>
### new ResultSet(entries)
A ResultSet contains a list or results


| Param | Type |
| --- | --- |
| entries | <code>Array</code> | 

<a name="ResultSet+valueOf"></a>
### resultSet.valueOf() ⇒ <code>Array</code>
Returns the array with results hold by this ResultSet

**Kind**: instance method of <code>[ResultSet](#ResultSet)</code>  
**Returns**: <code>Array</code> - entries  
<a name="ResultSet+toString"></a>
### resultSet.toString() ⇒ <code>string</code>
Returns the stringified results of the ResultSet

**Kind**: instance method of <code>[ResultSet](#ResultSet)</code>  
**Returns**: <code>string</code> - string  
<a name="ResultSet+toJSON"></a>
### resultSet.toJSON() ⇒ <code>Object</code>
Get a JSON representation of the ResultSet

**Kind**: instance method of <code>[ResultSet](#ResultSet)</code>  
**Returns**: <code>Object</code> - Returns a JSON object structured as:
                  `{"mathjs": "ResultSet", "entries": [...]}`  
<a name="ResultSet.fromJSON"></a>
### ResultSet.fromJSON(json) ⇒ <code>[ResultSet](#ResultSet)</code>
Instantiate a ResultSet from a JSON object

**Kind**: static method of <code>[ResultSet](#ResultSet)</code>  

| Param | Type | Description |
| --- | --- | --- |
| json | <code>Object</code> | A JSON object structured as:                       `{"mathjs": "ResultSet", "entries": [...]}` |



## sparsematrix

<a name="SparseMatrix"></a>
## sparsematrix: SparseMatrix
Sparse Matrix implementation. This type implements a Compressed Column Storage format
for sparse matrices.

* _instance_
	* [.storage()](#SparseMatrix+storage) ⇒ <code>string</code>
	* [.datatype()](#SparseMatrix+datatype) ⇒ <code>string</code>
	* [.create(data, [datatype])](#SparseMatrix+create)
	* [.density()](#SparseMatrix+density) ⇒ <code>number</code>
	* [.subset(index, [replacement], [defaultValue])](#SparseMatrix+subset)
	* [.get(index)](#SparseMatrix+get) ⇒ <code>\*</code>
	* [.set(index, value, [defaultValue])](#SparseMatrix+set) ⇒ <code>[SparseMatrix](#SparseMatrix)</code>
	* [.resize(size, [defaultValue], [copy])](#SparseMatrix+resize) ⇒ <code>Matrix</code>
	* [.clone()](#SparseMatrix+clone) ⇒ <code>[SparseMatrix](#SparseMatrix)</code>
	* [.size()](#SparseMatrix+size) ⇒ <code>Array.&lt;number&gt;</code>
	* [.map(callback, [skipZeros])](#SparseMatrix+map) ⇒ <code>[SparseMatrix](#SparseMatrix)</code>
	* [.forEach(callback, [skipZeros])](#SparseMatrix+forEach)
	* [.toArray()](#SparseMatrix+toArray) ⇒ <code>Array</code>
	* [.valueOf()](#SparseMatrix+valueOf) ⇒ <code>Array</code>
	* [.format([options])](#SparseMatrix+format) ⇒ <code>string</code>
	* [.toString()](#SparseMatrix+toString) ⇒ <code>string</code>
	* [.toJSON()](#SparseMatrix+toJSON) ⇒ <code>Object</code>
	* [.diagonal([k])](#SparseMatrix+diagonal) ⇒ <code>Matrix</code>
	* [.swapRows(i, j)](#SparseMatrix+swapRows) ⇒ <code>Matrix</code>
* _static_
	* [.fromJSON(json)](#SparseMatrix.fromJSON) ⇒ <code>[SparseMatrix](#SparseMatrix)</code>
	* [.diagonal(size, value, [k], [datatype])](#SparseMatrix.diagonal) ⇒ <code>[SparseMatrix](#SparseMatrix)</code>

<a name="SparseMatrix+storage"></a>
### sparseMatrix.storage() ⇒ <code>string</code>
Get the storage format used by the matrix.

Usage:
```js
const format = matrix.storage()  // retrieve storage format
```

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>string</code> - The storage format.  
<a name="SparseMatrix+datatype"></a>
### sparseMatrix.datatype() ⇒ <code>string</code>
Get the datatype of the data stored in the matrix.

Usage:
```js
const format = matrix.datatype()  // retrieve matrix datatype
```

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>string</code> - The datatype.  
<a name="SparseMatrix+create"></a>
### sparseMatrix.create(data, [datatype])
Create a new SparseMatrix

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  

| Param | Type |
| --- | --- |
| data | <code>Array</code> | 
| [datatype] | <code>string</code> | 

<a name="SparseMatrix+density"></a>
### sparseMatrix.density() ⇒ <code>number</code>
Get the matrix density.

Usage:
```js
const density = matrix.density()  // retrieve matrix density
```

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>number</code> - The matrix density.  
<a name="SparseMatrix+subset"></a>
### sparseMatrix.subset(index, [replacement], [defaultValue])
Get a subset of the matrix, or replace a subset of the matrix.

Usage:
```js
const subset = matrix.subset(index)               // retrieve subset
const value = matrix.subset(index, replacement)   // replace subset
```

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| index | <code>Index</code> |  |  |
| [replacement] | <code>Array</code> &#124; <code>Maytrix</code> &#124; <code>\*</code> |  |  |
| [defaultValue] | <code>\*</code> | <code>0</code> | Default value, filled in on new entries when                                  the matrix is resized. If not provided,                                  new matrix elements will be filled with zeros. |

<a name="SparseMatrix+get"></a>
### sparseMatrix.get(index) ⇒ <code>\*</code>
Get a single element from the matrix.

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>\*</code> - value  

| Param | Type | Description |
| --- | --- | --- |
| index | <code>Array.&lt;number&gt;</code> | Zero-based index |

<a name="SparseMatrix+set"></a>
### sparseMatrix.set(index, value, [defaultValue]) ⇒ <code>[SparseMatrix](#SparseMatrix)</code>
Replace a single element in the matrix.

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>[SparseMatrix](#SparseMatrix)</code> - self  

| Param | Type | Description |
| --- | --- | --- |
| index | <code>Array.&lt;number&gt;</code> | Zero-based index |
| value | <code>\*</code> |  |
| [defaultValue] | <code>\*</code> | Default value, filled in on new entries when                                  the matrix is resized. If not provided,                                  new matrix elements will be set to zero. |

<a name="SparseMatrix+resize"></a>
### sparseMatrix.resize(size, [defaultValue], [copy]) ⇒ <code>Matrix</code>
Resize the matrix to the given size. Returns a copy of the matrix when 
`copy=true`, otherwise return the matrix itself (resize in place).

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>Matrix</code> - The resized matrix  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| size | <code>Array.&lt;number&gt;</code> |  | The new size the matrix should have. |
| [defaultValue] | <code>\*</code> | <code>0</code> | Default value, filled in on new entries.                                  If not provided, the matrix elements will                                  be filled with zeros. |
| [copy] | <code>boolean</code> |  | Return a resized copy of the matrix |

<a name="SparseMatrix+clone"></a>
### sparseMatrix.clone() ⇒ <code>[SparseMatrix](#SparseMatrix)</code>
Create a clone of the matrix

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>[SparseMatrix](#SparseMatrix)</code> - clone  
<a name="SparseMatrix+size"></a>
### sparseMatrix.size() ⇒ <code>Array.&lt;number&gt;</code>
Retrieve the size of the matrix.

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>Array.&lt;number&gt;</code> - size  
<a name="SparseMatrix+map"></a>
### sparseMatrix.map(callback, [skipZeros]) ⇒ <code>[SparseMatrix](#SparseMatrix)</code>
Create a new matrix with the results of the callback function executed on
each entry of the matrix.

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>[SparseMatrix](#SparseMatrix)</code> - matrix  

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | The callback function is invoked with three                              parameters: the value of the element, the index                              of the element, and the Matrix being traversed. |
| [skipZeros] | <code>boolean</code> | Invoke callback function for non-zero values only. |

<a name="SparseMatrix+forEach"></a>
### sparseMatrix.forEach(callback, [skipZeros])
Execute a callback function on each entry of the matrix.

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  

| Param | Type | Description |
| --- | --- | --- |
| callback | <code>function</code> | The callback function is invoked with three                              parameters: the value of the element, the index                              of the element, and the Matrix being traversed. |
| [skipZeros] | <code>boolean</code> | Invoke callback function for non-zero values only. |

<a name="SparseMatrix+toArray"></a>
### sparseMatrix.toArray() ⇒ <code>Array</code>
Create an Array with a copy of the data of the SparseMatrix

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>Array</code> - array  
<a name="SparseMatrix+valueOf"></a>
### sparseMatrix.valueOf() ⇒ <code>Array</code>
Get the primitive value of the SparseMatrix: a two dimensions array

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>Array</code> - array  
<a name="SparseMatrix+format"></a>
### sparseMatrix.format([options]) ⇒ <code>string</code>
Get a string representation of the matrix, with optional formatting options.

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>string</code> - str  

| Param | Type | Description |
| --- | --- | --- |
| [options] | <code>Object</code> &#124; <code>number</code> &#124; <code>function</code> | Formatting options. See                                                lib/utils/number:format for a                                                description of the available                                                options. |

<a name="SparseMatrix+toString"></a>
### sparseMatrix.toString() ⇒ <code>string</code>
Get a string representation of the matrix

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>string</code> - str  
<a name="SparseMatrix+toJSON"></a>
### sparseMatrix.toJSON() ⇒ <code>Object</code>
Get a JSON representation of the matrix

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
<a name="SparseMatrix+diagonal"></a>
### sparseMatrix.diagonal([k]) ⇒ <code>Matrix</code>
Get the kth Matrix diagonal.

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>Matrix</code> - The matrix vector with the diagonal values.  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| [k] | <code>number</code> &#124; <code>BigNumber</code> | <code>0</code> | The kth diagonal where the vector will retrieved. |

<a name="SparseMatrix+swapRows"></a>
### sparseMatrix.swapRows(i, j) ⇒ <code>Matrix</code>
Swap rows i and j in Matrix.

**Kind**: instance method of <code>[SparseMatrix](#SparseMatrix)</code>  
**Returns**: <code>Matrix</code> - The matrix reference  

| Param | Type | Description |
| --- | --- | --- |
| i | <code>number</code> | Matrix row index 1 |
| j | <code>number</code> | Matrix row index 2 |

<a name="SparseMatrix.fromJSON"></a>
### SparseMatrix.fromJSON(json) ⇒ <code>[SparseMatrix](#SparseMatrix)</code>
Generate a matrix from a JSON object

**Kind**: static method of <code>[SparseMatrix](#SparseMatrix)</code>  

| Param | Type | Description |
| --- | --- | --- |
| json | <code>Object</code> | An object structured like                       `{"mathjs": "SparseMatrix", "values": [], "index": [], "ptr": [], "size": []}`,                       where mathjs is optional |

<a name="SparseMatrix.diagonal"></a>
### SparseMatrix.diagonal(size, value, [k], [datatype]) ⇒ <code>[SparseMatrix](#SparseMatrix)</code>
Create a diagonal matrix.

**Kind**: static method of <code>[SparseMatrix](#SparseMatrix)</code>  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| size | <code>Array</code> |  | The matrix size. |
| value | <code>number</code> &#124; <code>Array</code> &#124; <code>Matrix</code> |  | The values for the diagonal. |
| [k] | <code>number</code> &#124; <code>BigNumber</code> | <code>0</code> | The kth diagonal where the vector will be filled in. |
| [datatype] | <code>string</code> |  | The Matrix datatype, values must be of this datatype. |



## unit

<a name="Unit"></a>
## unit: Unit
* [new Unit([value], [name])](#new_Unit_new)
* _instance_
	* [.valueOf](#Unit+valueOf) ⇒ <code>string</code>
	* [.clone()](#Unit+clone) ⇒ <code>Unit</code>
	* [._isDerived()](#Unit+_isDerived) ⇒ <code>boolean</code>
	* [.hasBase(base)](#Unit+hasBase)
	* [.equalBase(other)](#Unit+equalBase) ⇒ <code>boolean</code>
	* [.equals(other)](#Unit+equals) ⇒ <code>boolean</code>
	* [.multiply(other)](#Unit+multiply) ⇒ <code>Unit</code>
	* [.divide(other)](#Unit+divide) ⇒ <code>Unit</code>
	* [.pow(p)](#Unit+pow) ⇒ <code>Unit</code>
	* [.abs(x)](#Unit+abs) ⇒ <code>Unit</code>
	* [.to(valuelessUnit)](#Unit+to) ⇒ <code>Unit</code>
	* [.toNumber(valuelessUnit)](#Unit+toNumber) ⇒ <code>number</code>
	* [.toNumeric(valuelessUnit)](#Unit+toNumeric) ⇒ <code>number</code> &#124; <code>BigNumber</code> &#124; <code>Fraction</code>
	* [.toString()](#Unit+toString) ⇒ <code>string</code>
	* [.toJSON()](#Unit+toJSON) ⇒ <code>Object</code>
	* [.formatUnits()](#Unit+formatUnits) ⇒ <code>string</code>
	* [.format([options])](#Unit+format) ⇒ <code>string</code>
	* [.toBest(unitList, options)](#Unit+toBest) ⇒ <code>Unit</code>
* _static_
	* [.parse(str)](#Unit.parse) ⇒ <code>Unit</code>
	* [.isValuelessUnit(name)](#Unit.isValuelessUnit) ⇒ <code>boolean</code>
	* [.fromJSON(json)](#Unit.fromJSON) ⇒ <code>Unit</code>

<a name="new_Unit_new"></a>
### new Unit([value], [name])
A unit can be constructed in the following ways:

```js
const a = new Unit(value, name)
const b = new Unit(null, name)
const c = Unit.parse(str)
```

Example usage:

```js
const a = new Unit(5, 'cm')               // 50 mm
const b = Unit.parse('23 kg')             // 23 kg
const c = math.in(a, new Unit(null, 'm')  // 0.05 m
const d = new Unit(9.81, "m/s^2")         // 9.81 m/s^2
```

| Param | Type | Description |
| --- | --- | --- |
| [value] | <code>number</code> &#124; <code>BigNumber</code> &#124; <code>Fraction</code> &#124; <code>Complex</code> &#124; <code>boolean</code> | A value like 5.2 |
| [name] | <code>string</code> | A unit name like "cm" or "inch", or a derived unit of the form: "u1[^ex1] [u2[^ex2] ...] [/ u3[^ex3] [u4[^ex4]]]", such as "kg m^2/s^2", where each unit appearing after the forward slash is taken to be in the denominator. "kg m^2 s^-2" is a synonym and is also acceptable. Any of the units can include a prefix. |

<a name="Unit+valueOf"></a>
### unit.valueOf ⇒ <code>string</code>
Returns the string representation of the unit.

**Kind**: instance property of <code>Unit</code>  
<a name="Unit+clone"></a>
### unit.clone() ⇒ <code>Unit</code>
create a copy of this unit

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>Unit</code> - Returns a cloned version of the unit  
<a name="Unit+_isDerived"></a>
### unit._isDerived() ⇒ <code>boolean</code>
Return whether the unit is derived (such as m/s, or cm^2, but not N)

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>boolean</code> - True if the unit is derived  
<a name="Unit+hasBase"></a>
### unit.hasBase(base)
check if this unit has given base unit
If this unit is a derived unit, this will ALWAYS return false, since by definition base units are not derived.

**Kind**: instance method of <code>Unit</code>  

| Param | Type |
| --- | --- |
| base | <code>BASE_UNITS</code> &#124; <code>STRING</code> &#124; <code>undefined</code> | 

<a name="Unit+equalBase"></a>
### unit.equalBase(other) ⇒ <code>boolean</code>
Check if this unit has a base or bases equal to another base or bases
For derived units, the exponent on each base also must match

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>boolean</code> - true if equal base  

| Param | Type |
| --- | --- |
| other | <code>Unit</code> | 

<a name="Unit+equals"></a>
### unit.equals(other) ⇒ <code>boolean</code>
Check if this unit equals another unit

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>boolean</code> - true if both units are equal  

| Param | Type |
| --- | --- |
| other | <code>Unit</code> | 

<a name="Unit+multiply"></a>
### unit.multiply(other) ⇒ <code>Unit</code>
Multiply this unit with another one

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>Unit</code> - product of this unit and the other unit  

| Param | Type |
| --- | --- |
| other | <code>Unit</code> | 

<a name="Unit+divide"></a>
### unit.divide(other) ⇒ <code>Unit</code>
Divide this unit by another one

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>Unit</code> - result of dividing this unit by the other unit  

| Param | Type |
| --- | --- |
| other | <code>Unit</code> | 

<a name="Unit+pow"></a>
### unit.pow(p) ⇒ <code>Unit</code>
Calculate the power of a unit

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>Unit</code> - The result: this^p  

| Param | Type |
| --- | --- |
| p | <code>number</code> &#124; <code>Fraction</code> &#124; <code>BigNumber</code> | 

<a name="Unit+abs"></a>
### unit.abs(x) ⇒ <code>Unit</code>
Calculate the absolute value of a unit

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>Unit</code> - The result: |x|, absolute value of x  

| Param | Type |
| --- | --- |
| x | <code>number</code> &#124; <code>Fraction</code> &#124; <code>BigNumber</code> | 

<a name="Unit+to"></a>
### unit.to(valuelessUnit) ⇒ <code>Unit</code>
Convert the unit to a specific unit name.

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>Unit</code> - Returns a clone of the unit with a fixed prefix and unit.  

| Param | Type | Description |
| --- | --- | --- |
| valuelessUnit | <code>string</code> &#124; <code>Unit</code> | A unit without value. Can have prefix, like "cm" |

<a name="Unit+toNumber"></a>
### unit.toNumber(valuelessUnit) ⇒ <code>number</code>
Return the value of the unit when represented with given valueless unit

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>number</code> - Returns the unit value as number.  

| Param | Type | Description |
| --- | --- | --- |
| valuelessUnit | <code>string</code> &#124; <code>Unit</code> | For example 'cm' or 'inch' |

<a name="Unit+toNumeric"></a>
### unit.toNumeric(valuelessUnit) ⇒ <code>number</code> &#124; <code>BigNumber</code> &#124; <code>Fraction</code>
Return the value of the unit in the original numeric type

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>number</code> &#124; <code>BigNumber</code> &#124; <code>Fraction</code> - Returns the unit value  

| Param | Type | Description |
| --- | --- | --- |
| valuelessUnit | <code>string</code> &#124; <code>Unit</code> | For example 'cm' or 'inch' |

<a name="Unit+toString"></a>
### unit.toString() ⇒ <code>string</code>
Get a string representation of the unit.

**Kind**: instance method of <code>Unit</code>  
<a name="Unit+toJSON"></a>
### unit.toJSON() ⇒ <code>Object</code>
Get a JSON representation of the unit

**Kind**: instance method of <code>Unit</code>  
**Returns**: <code>Object</code> - Returns a JSON object structured as:
                  `{"mathjs": "Unit", "value": 2, "unit": "cm", "fixPrefix": false}`  
<a name="Unit+formatUnits"></a>
### unit.formatUnits() ⇒ <code>string</code>
Get a string representation of the units of this Unit, without the value.

**Kind**: instance method of <code>Unit</code>  
<a name="Unit+format"></a>
### unit.format([options]) ⇒ <code>string</code>
Get a string representation of the Unit, with optional formatting options.

**Kind**: instance method of <code>Unit</code>  

| Param | Type | Description |
| --- | --- | --- |
| [options] | <code>Object</code> &#124; <code>number</code> &#124; <code>function</code> | Formatting options. See                                                lib/utils/number:format for a                                                description of the available                                                options. |

<a name="Unit+toBest"></a>
### unit.toBest(unitList, options) ⇒ <code>Unit</code>
Converts a unit to the most appropriate display unit with optional unitList and options.

**Kind**: instance method of <code>Unit</code>
**Returns**: <code>Unit</code>  

| Param | Type | Description |
| --- | --- | --- |
| unitList | <code>Array of strings</code> &#124; | Units strings array (optional).
| options | <code>Object</code> &#124; | Only "offset" available right now - better prefix calculation

<a name="Unit.parse"></a>
### Unit.parse(str) ⇒ <code>Unit</code>
Parse a string into a unit. The value of the unit is parsed as number,
BigNumber, or Fraction depending on the math.js config setting `number`.

Throws an exception if the provided string does not contain a valid unit or
cannot be parsed.

**Kind**: static method of <code>Unit</code>  
**Returns**: <code>Unit</code> - unit  

| Param | Type | Description |
| --- | --- | --- |
| str | <code>string</code> | A string like "5.2 inch", "4e2 cm/s^2" |

<a name="Unit.isValuelessUnit"></a>
### Unit.isValuelessUnit(name) ⇒ <code>boolean</code>
Test if the given expression is a unit.
The unit can have a prefix but cannot have a value.

**Kind**: static method of <code>Unit</code>  
**Returns**: <code>boolean</code> - true if the given string is a unit  

| Param | Type | Description |
| --- | --- | --- |
| name | <code>string</code> | A string to be tested whether it is a value less unit.                        The unit can have prefix, like "cm" |

<a name="Unit.fromJSON"></a>
### Unit.fromJSON(json) ⇒ <code>Unit</code>
Instantiate a Unit from a JSON object

**Kind**: static method of <code>Unit</code>  

| Param | Type | Description |
| --- | --- | --- |
| json | <code>Object</code> | A JSON object structured as:                       `{"mathjs": "Unit", "value": 2, "unit": "cm", "fixPrefix": false}` |



## Constant reference

Math.js contains the following constants. 

Constant        | Description | Value
--------------- | ----------- | -----
`e`, `E`        | Euler's number, the base of the natural logarithm. | 2.718281828459045
`i`             | Imaginary unit, defined as `i * i = -1`. A complex number is described as `a + b * i`, where a is the real part, and b is the imaginary part. | `sqrt(-1)`
`Infinity`      | Infinity, a number which is larger than the maximum number that can be handled by a floating point number. | `Infinity`
`LN2`           | Returns the natural logarithm of 2. | `0.6931471805599453`
`LN10`          | Returns the natural logarithm of 10. | `2.302585092994046`
`LOG2E`         | Returns the base-2 logarithm of E. | `1.4426950408889634`
`LOG10E`        | Returns the base-10 logarithm of E. | `0.4342944819032518`
`NaN`           | Not a number. | `NaN`
`null`          | Value null. | `null`
`phi`           | Phi is the golden ratio. Two quantities are in the golden ratio if their ratio is the same as the ratio of their sum to the larger of the two quantities. Phi is defined as `(1 + sqrt(5)) / 2` | `1.618033988749895`
`pi`, `PI`      | The number pi is a mathematical constant that is the ratio of a circle\'s circumference to its diameter. | `3.141592653589793`
`SQRT1_2`       | Returns the square root of 1/2. | `0.7071067811865476`
`SQRT2`         | Returns the square root of 2. | `1.4142135623730951`
`tau`           | Tau is the ratio constant of a circle\'s circumference to radius, equal to `2 * pi`. | `6.283185307179586`
`undefined`     | An undefined value. Preferably, use `null` to indicate undefined values. | `undefined`
`version`       | Returns the version number of math.js. | For example `0.24.1`

Example usage:

```js
math.sin(math.pi / 4)          // 0.70711
math.multiply(math.i, math.i)  // -1
```


## Reference

- [Classes](classes.md)
- [Constants](constants.md)
- [Functions](functions.md)



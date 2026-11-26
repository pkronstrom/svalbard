# @turf/turf 7.4.0 reference

This is the project's own documentation, joined at the version on this drive. Here nothing is installed or downloaded: import the package by name in a module script and pack the page with `svalbard-drive html pack`. Code blocks marked `not-for-this-stick` and lines starting "Not for this drive:" show package managers, CDN links or downloads; this drive does not use them.

## @turf/turf

## @turf/turf: turf

Turf is a modular geospatial analysis engine written in JavaScript. It performs geospatial
processing tasks with GeoJSON data and can be run on a server or in a browser.



---

This module is part of the [Turfjs project](http://turfjs.org/), an open source
module collection dedicated to geographic algorithms. It is maintained in the
[Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create
PRs and issues.

### Installation

Install this module individually:

```not-for-this-stick
$ npm install @turf/turf
```

Or install the Turf module that includes it as a function:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/along

## @turf/along: along

Takes a [LineString][1] and returns a [Point][2] at a specified distance along the line.

### Parameters

*   `line` **([Feature][3]<[LineString][1]> | [LineString][1])** input line
*   `distance` **[number][4]** distance along the line
*   `options` **[Object][5]?** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][6]. (optional, default `"kilometers"`)

### Examples

```javascript
var line = turf.lineString([[-83, 30], [-84, 36], [-78, 41]]);
var options = {units: 'miles'};

var along = turf.along(line, 200, options);

//addToMap
var addToMap = [along, line]
```

Returns **[Feature][3]<[Point][2]>** Point `distance` `units` along the line

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://turfjs.org/docs/api/types/Units



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/along
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/angle

## @turf/angle: angle

Finds the angle formed by two adjacent segments defined by 3 points. The result will be the (positive clockwise)
angle with origin on the `startPoint-midPoint` segment, or its explementary angle if required.

### Parameters

*   `startPoint` **[Coord][1]** Start Point Coordinates
*   `midPoint` **[Coord][1]** Mid Point Coordinates
*   `endPoint` **[Coord][1]** End Point Coordinates
*   `options` **[Object][2]** Optional parameters (optional, default `{}`)

    *   `options.explementary` **[boolean][3]** Returns the explementary angle instead (360 - angle) (optional, default `false`)
    *   `options.mercator` **[boolean][3]** if calculations should be performed over Mercator or WGS84 projection (optional, default `false`)

### Examples

```javascript
turf.angle([5, 5], [5, 6], [3, 4]);
//=45
```

Returns **[number][4]** Angle between the provided points, or its explementary.

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/angle
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


### Diagrams

![turf-angle](diagrams/turf-angle.png)

## @turf/area

## @turf/area: area

Calculates the geodesic area in square meters of one or more polygons.

### Parameters

*   `geojson` **[GeoJSON][1]** input polygon(s) as [Geometry][2], [Feature][3], or [FeatureCollection][4]

### Examples

```javascript
var polygon = turf.polygon([[[125, -15], [113, -22], [154, -27], [144, -15], [125, -15]]]);

var area = turf.area(polygon);

//addToMap
var addToMap = [polygon]
polygon.properties.area = area
```

Returns **[number][5]** area in square meters

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://tools.ietf.org/html/rfc7946#section-3.3

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/area
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/bbox-clip

## @turf/bbox-clip: bboxClip

Takes a [Feature][1] and a bbox and clips the feature to the bbox using
[lineclip][2].
May result in degenerate edges when clipping Polygons.

### Parameters

*   `feature` **[Feature][1]<([LineString][3] | [MultiLineString][4] | [Polygon][5] | [MultiPolygon][6])>** feature to clip to the bbox
*   `bbox` **[BBox][7]** extent in \[minX, minY, maxX, maxY] order

### Examples

```javascript
var bbox = [0, 0, 10, 10];
var poly = turf.polygon([[[2, 2], [8, 4], [12, 8], [3, 7], [2, 2]]]);

var clipped = turf.bboxClip(poly, bbox);

//addToMap
var addToMap = [bbox, poly, clipped]
```

Returns **[Feature][1]<([LineString][3] | [MultiLineString][4] | [Polygon][5] | [MultiPolygon][6])>** clipped Feature

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://github.com/mapbox/lineclip

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[7]: https://tools.ietf.org/html/rfc7946#section-5



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/bbox-clip
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/bbox-polygon

## @turf/bbox-polygon: bboxPolygon

Takes a bbox and returns an equivalent [polygon][1].

### Parameters

*   `bbox` **[BBox][2]** extent in \[minX, minY, maxX, maxY] order
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.properties` **[GeoJsonProperties][4]** Translate properties to Polygon (optional, default `{}`)
    *   `options.id` **([string][5] | [number][6])** Translate Id to Polygon (optional, default `{}`)

### Examples

```javascript
var bbox = [0, 0, 10, 10];

var poly = turf.bboxPolygon(bbox);

//addToMap
var addToMap = [poly]
```

Returns **[Feature][4]<[Polygon][1]>** a Polygon representation of the bounding box

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[2]: https://tools.ietf.org/html/rfc7946#section-5

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://tools.ietf.org/html/rfc7946#section-3.2

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/bbox-polygon
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/bbox

## @turf/bbox: bbox

Calculates the bounding box for any GeoJSON object, including FeatureCollection.
Uses geojson.bbox if available and options.recompute is not set.

### Parameters

*   `geojson` **[GeoJSON][1]** any GeoJSON object
*   `options` **[Object][2]** Optional parameters (optional, default `{}`)

    *   `options.recompute` **[boolean][3]?** Whether to ignore an existing bbox property on geojson

### Examples

```javascript
var line = turf.lineString([[-74, 40], [-78, 42], [-82, 35]]);
var bbox = turf.bbox(line);
var bboxPolygon = turf.bboxPolygon(bbox);

//addToMap
var addToMap = [line, bboxPolygon]
```

Returns **[BBox][4]** bbox extent in \[minX, minY, maxX, maxY] order

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[4]: https://tools.ietf.org/html/rfc7946#section-5



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/bbox
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/bearing

## @turf/bearing: bearing

Takes two [points][1] and finds the geographic bearing between them,
i.e. the angle measured in degrees from the north line (0 degrees)

### Parameters

*   `start` **[Coord][2]** starting Point
*   `end` **[Coord][2]** ending Point
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.final` **[boolean][4]** calculates the final bearing if true (optional, default `false`)

### Examples

```javascript
var point1 = turf.point([-75.343, 39.984]);
var point2 = turf.point([-75.534, 39.123]);

var bearing = turf.bearing(point1, point2);

//addToMap
var addToMap = [point1, point2]
point1.properties['marker-color'] = '#f00'
point2.properties['marker-color'] = '#0f0'
point1.properties.bearing = bearing
```

Returns **[number][5]** bearing in decimal degrees, between -180 and 180 degrees (positive clockwise)

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/bearing
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/bezier-spline

## @turf/bezier-spline: bezierSpline

Takes a [line][1] and returns a curved version
by applying a [Bezier spline][2]
algorithm.

The bezier spline implementation is by [Leszek Rybicki][3].

### Parameters

*   `line` **[Feature][4]<[LineString][1]>** input LineString
*   `options` **[Object][5]** Optional parameters (optional, default `{}`)

    *   `options.properties` **[Object][5]** Translate properties to output (optional, default `{}`)
    *   `options.resolution` **[number][6]** time in milliseconds between points (optional, default `10000`)
    *   `options.sharpness` **[number][6]** a measure of how curvy the path should be between splines (optional, default `0.85`)

### Examples

```javascript
var line = turf.lineString([
  [-76.091308, 18.427501],
  [-76.695556, 18.729501],
  [-76.552734, 19.40443],
  [-74.61914, 19.134789],
  [-73.652343, 20.07657],
  [-73.157958, 20.210656]
]);

var curved = turf.bezierSpline(line);

//addToMap
var addToMap = [line, curved]
curved.properties = { stroke: '#0F0' };
```

Returns **[Feature][4]<[LineString][1]>** curved line

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: http://en.wikipedia.org/wiki/B%C3%A9zier_spline

[3]: http://leszek.rybicki.cc/

[4]: https://tools.ietf.org/html/rfc7946#section-3.2

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/bezier-spline
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/boolean-clockwise

## @turf/boolean-clockwise: booleanClockwise

Takes a ring and return true or false whether or not the ring is clockwise or counter-clockwise.

### Parameters

*   `line` **([Feature][1]<[LineString][2]> | [LineString][2] | [Array][3]<[Array][3]<[number][4]>>)** to be evaluated

### Examples

```javascript
var clockwiseRing = turf.lineString([[0,0],[1,1],[1,0],[0,0]]);
var counterClockwiseRing = turf.lineString([[0,0],[1,0],[1,1],[0,0]]);

turf.booleanClockwise(clockwiseRing)
//=true
turf.booleanClockwise(counterClockwiseRing)
//=false
```

Returns **[boolean][5]** true/false

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-clockwise
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/boolean-concave

## @turf/boolean-concave: booleanConcave

Takes a polygon and return true or false as to whether it is concave or not.

### Parameters

*   `polygon` **[Feature][1]<[Polygon][2]>** to be evaluated

### Examples

```javascript
var convexPolygon = turf.polygon([[[0,0],[0,1],[1,1],[1,0],[0,0]]]);

turf.booleanConcave(convexPolygon)
//=false
```

Returns **[boolean][3]** true/false

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-concave
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/boolean-contains

## @turf/boolean-contains: booleanContains

Tests whether geometry a contains geometry b.
The interiors of both geometries must intersect, and the interior and boundary of geometry b must not intersect the exterior of geometry a.
booleanContains(a, b) is equivalent to booleanWithin(b, a)

### Parameters

*   `feature1` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry
*   `feature2` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry

### Examples

```javascript
var line = turf.lineString([[1, 1], [1, 2], [1, 3], [1, 4]]);
var point = turf.point([1, 2]);

turf.booleanContains(line, point);
//=true
```

Returns **[boolean][3]** true/false

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-contains
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


### Diagrams

![esri-contains](diagrams/esri-contains.png)

## @turf/boolean-crosses

## @turf/boolean-crosses: booleanCrosses

Boolean-Crosses returns True if the intersection results in a geometry whose dimension is one less than
the maximum dimension of the two source geometries and the intersection set is interior to
both source geometries.

Boolean-Crosses returns t (TRUE) for only multipoint/polygon, multipoint/linestring, linestring/linestring, linestring/polygon, and linestring/multipolygon comparisons.
Other comparisons are not supported as they are outside the OpenGIS Simple Features spec and may give unexpected results.

### Parameters

*   `feature1` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry
*   `feature2` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry

### Examples

```javascript
var line1 = turf.lineString([[-2, 2], [4, 2]]);
var line2 = turf.lineString([[1, 1], [1, 2], [1, 3], [1, 4]]);

var cross = turf.booleanCrosses(line1, line2);
//=true
```

Returns **[boolean][3]** true/false

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-crosses
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


### Diagrams

![esri-crosses](diagrams/esri-crosses.gif)

## @turf/boolean-disjoint

## @turf/boolean-disjoint: booleanDisjoint

Boolean-disjoint returns (TRUE) if the intersection of the two geometries is an empty set.

### Parameters

*   `feature1` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry
*   `feature2` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.ignoreSelfIntersections` **[boolean][4]** ignore self-intersections on input features (optional, default `true`)

### Examples

```javascript
var point = turf.point([2, 2]);
var line = turf.lineString([[1, 1], [1, 2], [1, 3], [1, 4]]);

turf.booleanDisjoint(line, point);
//=true
```

Returns **[boolean][4]** true if the intersection is an empty set, false otherwise

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-disjoint
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


### Diagrams

![esri-disjoint](diagrams/esri-disjoint.gif)

## @turf/boolean-equal

## @turf/boolean-equal: booleanEqual

Determine whether two geometries of the same type have identical X,Y coordinate values.
See [http://edndoc.esri.com/arcsde/9.0/general\_topics/understand\_spatial\_relations.htm][1]

### Parameters

*   `feature1` **([Geometry][2] | [Feature][3])** GeoJSON input
*   `feature2` **([Geometry][2] | [Feature][3])** GeoJSON input
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.precision` **[number][5]** decimal precision to use when comparing coordinates (optional, default `6`)

### Examples

```javascript
var pt1 = turf.point([0, 0]);
var pt2 = turf.point([0, 0]);
var pt3 = turf.point([1, 1]);

turf.booleanEqual(pt1, pt2);
//= true
turf.booleanEqual(pt2, pt3);
//= false
```

Returns **[boolean][6]** true if the objects are equal, false otherwise

[1]: http://edndoc.esri.com/arcsde/9.0/general_topics/understand_spatial_relations.htm

[2]: https://tools.ietf.org/html/rfc7946#section-3.1

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-equal
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


### Diagrams

![esri-equals](diagrams/esri-equals.gif)

## @turf/boolean-intersects

## @turf/boolean-intersects: booleanIntersects

Boolean-intersects returns (TRUE) if the intersection of the two geometries is NOT an empty set.

### Parameters

*   `feature1` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry
*   `feature2` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.ignoreSelfIntersections` **[boolean][4]** ignore self-intersections on input features (optional, default `true`)

### Examples

```javascript
var point1 = turf.point([2, 2]);
var point2 = turf.point([1, 2]);
var line = turf.lineString([[1, 1], [1, 3], [1, 4]]);

turf.booleanIntersects(line, point1);
//=false

turf.booleanIntersects(line, point2);
//=true

//addToMap
var addToMap = [point1, point2, line];
point1.properties['marker-color'] = '#f00'
point2.properties['marker-color'] = '#0f0'
```

Returns **[boolean][4]** true if geometries intersect, false otherwise

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-intersects
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/boolean-overlap

## @turf/boolean-overlap: booleanOverlap

Compares two geometries of the same dimension and returns true if their intersection set results in a geometry
different from both but of the same dimension. It applies to Polygon/Polygon, LineString/LineString,
Multipoint/Multipoint, MultiLineString/MultiLineString and MultiPolygon/MultiPolygon.

In other words, it returns true if the two geometries overlap, provided that neither completely contains the other.

### Parameters

*   `feature1` **([Geometry][1] | [Feature][2]<([LineString][3] | [MultiLineString][4] | [Polygon][5] | [MultiPolygon][6])>)** input
*   `feature2` **([Geometry][1] | [Feature][2]<([LineString][3] | [MultiLineString][4] | [Polygon][5] | [MultiPolygon][6])>)** input

### Examples

```javascript
var poly1 = turf.polygon([[[0,0],[0,5],[5,5],[5,0],[0,0]]]);
var poly2 = turf.polygon([[[1,1],[1,6],[6,6],[6,1],[1,1]]]);
var poly3 = turf.polygon([[[10,10],[10,15],[15,15],[15,10],[10,10]]]);

turf.booleanOverlap(poly1, poly2)
//=true
turf.booleanOverlap(poly2, poly3)
//=false
```

Returns **[boolean][7]** true/false

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-overlap
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


### Diagrams

![esri-overlaps](diagrams/esri-overlaps.gif)

## @turf/boolean-parallel

## @turf/boolean-parallel: booleanParallel

Boolean-Parallel returns True if each segment of `line1` is parallel to the correspondent segment of `line2`

### Parameters

*   `line1` **([Geometry][1] | [Feature][2]<[LineString][3]>)** GeoJSON Feature or Geometry
*   `line2` **([Geometry][1] | [Feature][2]<[LineString][3]>)** GeoJSON Feature or Geometry

### Examples

```javascript
var line1 = turf.lineString([[0, 0], [0, 1]]);
var line2 = turf.lineString([[1, 0], [1, 1]]);

turf.booleanParallel(line1, line2);
//=true
```

Returns **[boolean][4]** true/false if the lines are parallel

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-parallel
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/boolean-point-in-polygon

## @turf/boolean-point-in-polygon: booleanPointInPolygon

Takes a [Point][1] and a [Polygon][2] or [MultiPolygon][3] and determines if the point
resides inside the polygon. The polygon can be convex or concave. The function accounts for holes.

### Parameters

*   `point` **[Coord][4]** input point
*   `polygon` **[Feature][5]<([Polygon][2] | [MultiPolygon][3])>** input polygon or multipolygon
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.ignoreBoundary` **[boolean][7]** True if polygon boundary should be ignored when determining if
        the point is inside the polygon otherwise false. (optional, default `false`)

### Examples

```javascript
var pt = turf.point([-77, 44]);
var poly = turf.polygon([[
  [-81, 41],
  [-81, 47],
  [-72, 47],
  [-72, 41],
  [-81, 41]
]]);

turf.booleanPointInPolygon(pt, poly);
//= true
```

Returns **[boolean][7]** `true` if the Point is inside the Polygon; `false` if the Point is not inside the Polygon

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-point-in-polygon
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/boolean-point-on-line

## @turf/boolean-point-on-line: booleanPointOnLine

Returns true if a point is on a line. Accepts a optional parameter to ignore the
start and end vertices of the linestring.

### Parameters

*   `pt` **[Coord][1]** GeoJSON Point
*   `line` **[Feature][2]<[LineString][3]>** GeoJSON LineString
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.ignoreEndVertices` **[boolean][5]** whether to ignore the start and end vertices. (optional, default `false`)
    *   `options.epsilon` **[number][6]?** Fractional number to compare with the cross product result. Useful for dealing with floating points such as lng/lat points

### Examples

```javascript
var pt = turf.point([0, 0]);
var line = turf.lineString([[-1, -1],[1, 1],[1.5, 2.2]]);
var isPointOnLine = turf.booleanPointOnLine(pt, line);
//=true
```

Returns **[boolean][5]** true/false

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-point-on-line
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/boolean-touches

## @turf/boolean-touches: booleanTouches

Boolean-touches true if none of the points common to both geometries
intersect the interiors of both geometries.

### Parameters

*   `feature1` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry
*   `feature2` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry

### Examples

```javascript
var line = turf.lineString([[1, 1], [1, 2], [1, 3], [1, 4]]);
var point = turf.point([1, 1]);

turf.booleanTouches(point, line);
//=true
```

Returns **[boolean][3]** true/false

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-touches
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/boolean-valid

## @turf/boolean-valid: booleanValid

booleanValid checks if the geometry is a valid according to the OGC Simple Feature Specification.

### Parameters

*   `feature` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry

### Examples

```javascript
var line = turf.lineString([[1, 1], [1, 2], [1, 3], [1, 4]]);

turf.booleanValid(line); // => true
turf.booleanValid({foo: "bar"}); // => false
```

Returns **[boolean][3]** true/false

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-valid
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/boolean-within

## @turf/boolean-within: booleanWithin

Tests whether geometry a is contained by geometry b.
The interiors of both geometries must intersect, and the interior and boundary of geometry a must not intersect the exterior of geometry b.
booleanWithin(a, b) is equivalent to booleanContains(b, a)

### Parameters

*   `feature1` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry
*   `feature2` **([Geometry][1] | [Feature][2]\<any>)** GeoJSON Feature or Geometry

### Examples

```javascript
var line = turf.lineString([[1, 1], [1, 2], [1, 3], [1, 4]]);
var point = turf.point([1, 2]);

turf.booleanWithin(point, line);
//=true
```

Returns **[boolean][3]** true/false

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/boolean-within
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


### Diagrams

![esri-within](diagrams/esri-within.png)

## @turf/buffer

## @turf/buffer: buffer

Calculates a buffer for input features for a given radius.

When using a negative radius, the resulting geometry may be invalid if
it's too small compared to the radius magnitude. If the input is a
FeatureCollection, only valid members will be returned in the output
FeatureCollection - i.e., the output collection may have fewer members than
the input, or even be empty.

### Parameters

*   `geojson` **([FeatureCollection][1] | [Geometry][2] | [Feature][3]\<any>)** input to be buffered
*   `radius` **[number][4]** distance to draw the buffer (negative values are allowed)
*   `options` **[Object][5]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][6]. (optional, default `"kilometers"`)
    *   `options.steps` **[number][4]** number of steps (optional, default `8`)

### Examples

```javascript
var point = turf.point([-90.548630, 14.616599]);
var buffered = turf.buffer(point, 500, {units: 'miles'});

//addToMap
var addToMap = [point, buffered]
```

Returns **([FeatureCollection][1] | [Feature][3]<([Polygon][7] | [MultiPolygon][8])> | [undefined][9])** buffered features

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://turfjs.org/docs/api/types/Units

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[8]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[9]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/undefined



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/buffer
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/center-mean

## @turf/center-mean: centerMean

Takes a [Feature][1] or [FeatureCollection][2] and returns the mean center. Can be weighted.

### Parameters

*   `geojson` **[GeoJSON][3]** GeoJSON to be centered
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.properties` **[Object][4]** Translate GeoJSON Properties to Point (optional, default `{}`)
    *   `options.bbox` **[Object][4]** Translate GeoJSON BBox to Point (optional, default `{}`)
    *   `options.id` **[Object][4]** Translate GeoJSON Id to Point (optional, default `{}`)
    *   `options.weight` **[string][5]?** the property name used to weight the center

### Examples

```javascript
var features = turf.featureCollection([
  turf.point([-97.522259, 35.4691], {value: 10}),
  turf.point([-97.502754, 35.463455], {value: 3}),
  turf.point([-97.508269, 35.463245], {value: 5})
]);

var options = {weight: "value"}
var mean = turf.centerMean(features, options);

//addToMap
var addToMap = [features, mean]
mean.properties['marker-size'] = 'large';
mean.properties['marker-color'] = '#000';
```

Returns **[Feature][1]<[Point][6]>** a Point feature at the mean center point of all input features

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.3

[3]: https://tools.ietf.org/html/rfc7946#section-3

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/center-mean
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/center-median

## @turf/center-median: centerMedian

Takes a [FeatureCollection][1] of points and calculates the median center,
algorithimically. The median center is understood as the point that is
requires the least total travel from all other points.

Turfjs has four different functions for calculating the center of a set of
data. Each is useful depending on circumstance.

`@turf/center` finds the simple center of a dataset, by finding the
midpoint between the extents of the data. That is, it divides in half the
farthest east and farthest west point as well as the farthest north and
farthest south.

`@turf/center-of-mass` imagines that the dataset is a sheet of paper.
The center of mass is where the sheet would balance on a fingertip.

`@turf/center-mean` takes the averages of all the coordinates and
produces a value that respects that. Unlike `@turf/center`, it is
sensitive to clusters and outliers. It lands in the statistical middle of a
dataset, not the geographical. It can also be weighted, meaning certain
points are more important than others.

`@turf/center-median` takes the mean center and tries to find, iteratively,
a new point that requires the least amount of travel from all the points in
the dataset. It is not as sensitive to outliers as `@turf/center-mean`, but it is
attracted to clustered data. It, too, can be weighted.

**Bibliography**

Harold W. Kuhn and Robert E. Kuenne, “An Efficient Algorithm for the
Numerical Solution of the Generalized Weber Problem in Spatial
Economics,” *Journal of Regional Science* 4, no. 2 (1962): 21–33,
doi:{@link [https://doi.org/10.1111/j.1467-9787.1962.tb00902.x}][2].

James E. Burt, Gerald M. Barber, and David L. Rigby, *Elementary
Statistics for Geographers*, 3rd ed., New York: The Guilford
Press, 2009, 150–151.

### Parameters

*   `features` **[FeatureCollection][1]\<any>** Any GeoJSON Feature Collection
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.weight` **[string][4]?** the property name used to weight the center
    *   `options.tolerance` **[number][5]** the difference in distance between candidate medians at which point the algorighim stops iterating. (optional, default `0.001`)
    *   `options.counter` **[number][5]** how many attempts to find the median, should the tolerance be insufficient. (optional, default `10`)

### Examples

```javascript
var points = turf.points([[0, 0], [1, 0], [0, 1], [5, 8]]);
var medianCenter = turf.centerMedian(points);

//addToMap
var addToMap = [points, medianCenter]
```

Returns **[Feature][6]<[Point][7]>** The median center of the collection

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://doi.org/10.1111/j.1467-9787.1962.tb00902.x}

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[6]: https://tools.ietf.org/html/rfc7946#section-3.2

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/center-median
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/center-of-mass

## @turf/center-of-mass: centerOfMass

Takes any [Feature][1] or a [FeatureCollection][2] and returns its [center of mass][3] using this formula: [Centroid of Polygon][4].

### Parameters

*   `geojson` **[GeoJSON][5]** GeoJSON to be centered
*   `options` **[Object][6]** Optional Parameters (optional, default `{}`)

    *   `options.properties` **[Object][6]** Translate Properties to Feature (optional, default `{}`)

### Examples

```javascript
var polygon = turf.polygon([[[-81, 41], [-88, 36], [-84, 31], [-80, 33], [-77, 39], [-81, 41]]]);

var center = turf.centerOfMass(polygon);

//addToMap
var addToMap = [polygon, center]
```

Returns **[Feature][1]<[Point][7]>** the center of mass

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.3

[3]: https://en.wikipedia.org/wiki/Center_of_mass

[4]: https://en.wikipedia.org/wiki/Centroid#Centroid_of_polygon

[5]: https://tools.ietf.org/html/rfc7946#section-3

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/center-of-mass
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/center

## @turf/center: center

Takes a [Feature][1] or [FeatureCollection][2] and returns the absolute center point of all features.

### Parameters

*   `geojson` **[GeoJSON][3]** GeoJSON to be centered
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.properties` **[Object][4]** Translate GeoJSON Properties to Point (optional, default `{}`)
    *   `options.bbox` **[Object][4]** Translate GeoJSON BBox to Point (optional, default `{}`)
    *   `options.id` **[Object][4]** Translate GeoJSON Id to Point (optional, default `{}`)

### Examples

```javascript
var features = turf.points([
  [-97.522259, 35.4691],
  [-97.502754, 35.463455],
  [-97.508269, 35.463245]
]);

var center = turf.center(features);

//addToMap
var addToMap = [features, center]
center.properties['marker-size'] = 'large';
center.properties['marker-color'] = '#000';
```

Returns **[Feature][1]<[Point][5]>** a Point feature at the absolute center point of all input features

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.3

[3]: https://tools.ietf.org/html/rfc7946#section-3

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/center
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/centroid

## @turf/centroid: centroid

Computes the centroid as the mean of all vertices within the object.

### Parameters

*   `geojson` **[GeoJSON][1]** GeoJSON to be centered
*   `options` **[Object][2]** Optional Parameters (optional, default `{}`)

    *   `options.properties` **[Object][2]** an Object that is used as the [Feature][3]'s properties (optional, default `{}`)

### Examples

```javascript
var polygon = turf.polygon([[[-81, 41], [-88, 36], [-84, 31], [-80, 33], [-77, 39], [-81, 41]]]);

var centroid = turf.centroid(polygon);

//addToMap
var addToMap = [polygon, centroid]
```

Returns **[Feature][3]<[Point][4]>** the centroid of the input object

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/centroid
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/circle

## @turf/circle: circle

Takes a [Point][1] and calculates the circle polygon given a radius in [Units][2]; and steps for precision.

### Parameters

*   `center` **([Feature][3]<[Point][1]> | [Array][4]<[number][5]>)** center point
*   `radius` **[number][5]** radius of the circle
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.steps` **[number][5]** number of steps (optional, default `64`)
    *   `options.units` **Units** Supports all valid Turf [Units][2] (optional, default `'kilometers'`)
    *   `options.properties` **[Object][6]** properties (optional, default `{}`)

### Examples

```javascript
var center = [-75.343, 39.984];
var radius = 5;
var options = {steps: 10, units: 'kilometers', properties: {foo: 'bar'}};
var circle = turf.circle(center, radius, options);

//addToMap
var addToMap = [turf.point(center), circle]
```

Returns **[Feature][3]<[Polygon][7]>** circle polygon

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://turfjs.org/docs/api/types/Units

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.6



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/circle
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/clean-coords

## @turf/clean-coords: cleanCoords

Removes redundant coordinates from any GeoJSON Geometry.

### Parameters

*   `geojson` **([Geometry][1] | [Feature][2])** Feature or Geometry
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.mutate` **[boolean][4]** allows GeoJSON input to be mutated (optional, default `false`)

### Examples

```javascript
var line = turf.lineString([[0, 0], [0, 2], [0, 5], [0, 8], [0, 8], [0, 10]]);
var multiPoint = turf.multiPoint([[0, 0], [0, 0], [2, 2]]);

turf.cleanCoords(line).geometry.coordinates;
//= [[0, 0], [0, 10]]

turf.cleanCoords(multiPoint).geometry.coordinates;
//= [[0, 0], [2, 2]]
```

Returns **([Geometry][1] | [Feature][2])** the cleaned input Feature/Geometry

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/clean-coords
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/clone

## @turf/clone: clone

Returns a cloned copy of the passed GeoJSON Object, including possible 'Foreign Members'.
\~3-5x faster than the common JSON.parse + JSON.stringify combo method.

### Parameters

*   `geojson` **[GeoJSON][1]** GeoJSON Object

### Examples

```javascript
var line = turf.lineString([[-74, 40], [-78, 42], [-82, 35]], {color: 'red'});

var lineCloned = turf.clone(line);
```

Returns **[GeoJSON][1]** cloned GeoJSON Object

[1]: https://tools.ietf.org/html/rfc7946#section-3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/clone
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/clusters-dbscan

## @turf/clusters-dbscan: Dbscan

Point classification within the cluster.

Type: (`"core"` | `"edge"` | `"noise"`)

## @turf/clusters-dbscan: DbscanProps

**Extends GeoJsonProperties**

Properties assigned to each clustered point.

Type: [object][1]

### Properties

*   `dbscan` **[Dbscan][2]?** type of point it has been classified as
*   `cluster` **[number][3]?** associated clusterId

## @turf/clusters-dbscan: clustersDbscan

Takes a set of [points][4] and partition them into clusters according to [DBSCAN's][5] data clustering algorithm.

### Parameters

*   `points` **[FeatureCollection][6]<[Point][4]>** to be clustered
*   `maxDistance` **[number][3]** Maximum Distance between any point of the cluster to generate the clusters (kilometers by default, see options)
*   `options` **[Object][1]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** in which `maxDistance` is expressed, Supports all valid Turf [Units][7] (optional, default `"kilometers"`)
    *   `options.mutate` **[boolean][8]** Allows GeoJSON input to be mutated (optional, default `false`)
    *   `options.minPoints` **[number][3]** Minimum number of points to generate a single cluster,
        points which do not meet this requirement will be classified as an 'edge' or 'noise'. (optional, default `3`)

### Examples

```javascript
// create random points with random z-values in their properties
var points = turf.randomPoint(100, {bbox: [0, 30, 20, 50]});
var maxDistance = 100;
var clustered = turf.clustersDbscan(points, maxDistance);

//addToMap
var addToMap = [clustered];
```

Returns **[FeatureCollection][6]<[Point][4], [DbscanProps][9]>** Clustered Points with an additional two properties associated to each Feature:*   {number} cluster - the associated clusterId
*   {string} dbscan - type of point it has been classified as ('core'|'edge'|'noise')

[1]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[2]: #dbscan

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[5]: https://en.wikipedia.org/wiki/DBSCAN

[6]: https://tools.ietf.org/html/rfc7946#section-3.3

[7]: https://turfjs.org/docs/api/types/Units

[8]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[9]: #dbscanprops



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/clusters-dbscan
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/clusters-kmeans

## @turf/clusters-kmeans: clustersKmeans

Takes a set of [points][1] and partition them into clusters using the k-mean .
It uses the [k-means algorithm][2]

### Parameters

*   `points` **[FeatureCollection][3]<[Point][1]>** to be clustered
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.numberOfClusters` **[number][5]** numberOfClusters that will be generated (optional, default `Math.sqrt(numberOfPoints/2)`)
    *   `options.mutate` **[boolean][6]** allows GeoJSON input to be mutated (significant performance increase if true) (optional, default `false`)

### Examples

```javascript
// create random points with random z-values in their properties
var points = turf.randomPoint(100, {bbox: [0, 30, 20, 50]});
var options = {numberOfClusters: 7};
var clustered = turf.clustersKmeans(points, options);

//addToMap
var addToMap = [clustered];
```

Returns **[FeatureCollection][3]<[Point][1]>** Clustered Points with an additional two properties associated to each Feature:*   {number} cluster - the associated clusterId
*   {\[number, number]} centroid - Centroid of the cluster \[Longitude, Latitude]

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://en.wikipedia.org/wiki/K-means_clustering

[3]: https://tools.ietf.org/html/rfc7946#section-3.3

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/clusters-kmeans
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/clusters

## @turf/clusters: getCluster

Get Cluster

### Parameters

*   `geojson` **[FeatureCollection][1]** GeoJSON Features
*   `filter` **any** Filter used on GeoJSON properties to get Cluster

### Examples

```javascript
var geojson = turf.featureCollection([
    turf.point([0, 0], {'marker-symbol': 'circle'}),
    turf.point([2, 4], {'marker-symbol': 'star'}),
    turf.point([3, 6], {'marker-symbol': 'star'}),
    turf.point([5, 1], {'marker-symbol': 'square'}),
    turf.point([4, 2], {'marker-symbol': 'circle'})
]);

// Create a cluster using K-Means (adds `cluster` to GeoJSON properties)
var clustered = turf.clustersKmeans(geojson);

// Retrieve first cluster (0)
var cluster = turf.getCluster(clustered, {cluster: 0});
//= cluster

// Retrieve cluster based on custom properties
turf.getCluster(clustered, {'marker-symbol': 'circle'}).length;
//= 2
turf.getCluster(clustered, {'marker-symbol': 'square'}).length;
//= 1
```

Returns **[FeatureCollection][1]** Single Cluster filtered by GeoJSON Properties

## @turf/clusters: clusterEachCallback

Callback for clusterEach

Type: [Function][2]

### Parameters

*   `cluster` **[FeatureCollection][1]?** The current cluster being processed.
*   `clusterValue` **any?** Value used to create cluster being processed.
*   `currentIndex` **[number][3]?** The index of the current element being processed in the array.Starts at index 0

Returns **void**&#x20;

## @turf/clusters: clusterEach

clusterEach

### Parameters

*   `geojson` **[FeatureCollection][1]** GeoJSON Features
*   `property` **([string][4] | [number][3])** GeoJSON property key/value used to create clusters
*   `callback` **[clusterEachCallback][5]** a method that takes (cluster, clusterValue, currentIndex)

### Examples

```javascript
var geojson = turf.featureCollection([
    turf.point([0, 0]),
    turf.point([2, 4]),
    turf.point([3, 6]),
    turf.point([5, 1]),
    turf.point([4, 2])
]);

// Create a cluster using K-Means (adds `cluster` to GeoJSON properties)
var clustered = turf.clustersKmeans(geojson);

// Iterate over each cluster
turf.clusterEach(clustered, 'cluster', function (cluster, clusterValue, currentIndex) {
    //= cluster
    //= clusterValue
    //= currentIndex
})

// Calculate the total number of clusters
var total = 0
turf.clusterEach(clustered, 'cluster', function () {
    total++;
});

// Create an Array of all the values retrieved from the 'cluster' property
var values = []
turf.clusterEach(clustered, 'cluster', function (cluster, clusterValue) {
    values.push(clusterValue);
});
```

Returns **void**&#x20;

## @turf/clusters: clusterReduceCallback

Callback for clusterReduce

The first time the callback function is called, the values provided as arguments depend
on whether the reduce method has an initialValue argument.

If an initialValue is provided to the reduce method:

*   The previousValue argument is initialValue.
*   The currentValue argument is the value of the first element present in the array.

If an initialValue is not provided:

*   The previousValue argument is the value of the first element present in the array.
*   The currentValue argument is the value of the second element present in the array.

Type: [Function][2]

### Parameters

*   `previousValue` **any?** The accumulated value previously returned in the last invocation
    of the callback, or initialValue, if supplied.
*   `cluster` **[FeatureCollection][1]?** The current cluster being processed.
*   `clusterValue` **any?** Value used to create cluster being processed.
*   `currentIndex` **[number][3]?** The index of the current element being processed in the
    array. Starts at index 0, if an initialValue is provided, and at index 1 otherwise.

Returns **void**&#x20;

## @turf/clusters: clusterReduce

Reduce clusters in GeoJSON Features, similar to Array.reduce()

### Parameters

*   `geojson` **[FeatureCollection][1]** GeoJSON Features
*   `property` **([string][4] | [number][3])** GeoJSON property key/value used to create clusters
*   `callback` **[clusterReduceCallback][6]** a method that takes (previousValue, cluster, clusterValue, currentIndex)
*   `initialValue` **any?** Value to use as the first argument to the first call of the callback.

### Examples

```javascript
var geojson = turf.featureCollection([
    turf.point([0, 0]),
    turf.point([2, 4]),
    turf.point([3, 6]),
    turf.point([5, 1]),
    turf.point([4, 2])
]);

// Create a cluster using K-Means (adds `cluster` to GeoJSON properties)
var clustered = turf.clustersKmeans(geojson);

// Iterate over each cluster and perform a calculation
var initialValue = 0
turf.clusterReduce(clustered, 'cluster', function (previousValue, cluster, clusterValue, currentIndex) {
    //=previousValue
    //=cluster
    //=clusterValue
    //=currentIndex
    return previousValue++;
}, initialValue);

// Calculate the total number of clusters
var total = turf.clusterReduce(clustered, 'cluster', function (previousValue) {
    return previousValue++;
}, 0);

// Create an Array of all the values retrieved from the 'cluster' property
var values = turf.clusterReduce(clustered, 'cluster', function (previousValue, cluster, clusterValue) {
    return previousValue.concat(clusterValue);
}, []);
```

Returns **any** The value that results from the reduction.

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Statements/function

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[5]: #clustereachcallback

[6]: #clusterreducecallback



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/clusters
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/collect

## @turf/collect: collect

Merges a specified property from a FeatureCollection of points into a
FeatureCollection of polygons. Given an `inProperty` on points and an `outProperty`
for polygons, this finds every point that lies within each polygon, collects the
`inProperty` values from those points, and adds them as an array to `outProperty`
on the polygon.

### Parameters

*   `polygons` **[FeatureCollection][1]<[Polygon][2]>** polygons with values on which to aggregate
*   `points` **[FeatureCollection][1]<[Point][3]>** points to be aggregated
*   `inProperty` **[string][4]** property to be nested from
*   `outProperty` **[string][4]** property to be nested into

### Examples

```javascript
var poly1 = turf.polygon([[[0,0],[10,0],[10,10],[0,10],[0,0]]]);
var poly2 = turf.polygon([[[10,0],[20,10],[20,20],[20,0],[10,0]]]);
var polyFC = turf.featureCollection([poly1, poly2]);
var pt1 = turf.point([5,5], {population: 200});
var pt2 = turf.point([1,3], {population: 600});
var pt3 = turf.point([14,2], {population: 100});
var pt4 = turf.point([13,1], {population: 200});
var pt5 = turf.point([19,7], {population: 300});
var pointFC = turf.featureCollection([pt1, pt2, pt3, pt4, pt5]);
var collected = turf.collect(polyFC, pointFC, 'population', 'values');
var values = collected.features[0].properties.values
//=values => [200, 600]

//addToMap
var addToMap = [pointFC, collected]
```

Returns **[FeatureCollection][1]<[Polygon][2]>** polygons with properties listed based on `outField`

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/collect
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/combine

## @turf/combine: combine

Combines a [FeatureCollection][1] of [Point][2], [MultiPoint][3], [LineString][4],
[MultiLineString][5], [Polygon][6], or [MultiPolygon][7] features
into [MultiPoint][3], [MultiLineString][5], or [MultiPolygon][7] features.

### Parameters

*   `fc` **[FeatureCollection][1]<([Point][2] | [MultiPoint][3] | [LineString][4] | [MultiLineString][5] | [Polygon][6] | [MultiPolygon][7])>** a FeatureCollection of Points, MultiPoints, LineStrings, MultiLineStrings, Polygons, or MultiPolygons

### Examples

```javascript
var fc = turf.featureCollection([
  turf.point([19.026432, 47.49134]),
  turf.point([19.074497, 47.509548])
]);

var combined = turf.combine(fc);

//addToMap
var addToMap = [combined]
```

Returns **[FeatureCollection][1]<([MultiPoint][3] | [MultiLineString][5] | [MultiPolygon][7])>** a FeatureCollection of corresponding type to input

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.3

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.7



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/combine
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/concave

## @turf/concave: concave

Takes a set of [points][1] and returns a concave hull Polygon or MultiPolygon.
Internally, this uses [turf-tin][2] to generate geometries.

### Parameters

*   `points` **[FeatureCollection][3]<[Point][1]>** input points
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.maxEdge` **[number][5]** the length (in 'units') of an edge necessary for part of the
        hull to become concave. (optional, default `Infinity`)
    *   `options.units` **Units** Supports all valid Turf [Units][6]. (optional, default `'kilometers'`)

### Examples

```javascript
var points = turf.featureCollection([
  turf.point([-63.601226, 44.642643]),
  turf.point([-63.591442, 44.651436]),
  turf.point([-63.580799, 44.648749]),
  turf.point([-63.573589, 44.641788]),
  turf.point([-63.587665, 44.64533]),
  turf.point([-63.595218, 44.64765])
]);
var options = {units: 'miles', maxEdge: 1};

var hull = turf.concave(points, options);

//addToMap
var addToMap = [points, hull]
```

Returns **([Feature][7]<([Polygon][8] | [MultiPolygon][9])> | null)** a concave hull (null value is returned if unable to compute hull)

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://github.com/Turfjs/turf-tin

[3]: https://tools.ietf.org/html/rfc7946#section-3.3

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[6]: https://turfjs.org/docs/api/types/Units

[7]: https://tools.ietf.org/html/rfc7946#section-3.2

[8]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[9]: https://tools.ietf.org/html/rfc7946#section-3.1.7



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/concave
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/convex

## @turf/convex: convex

Takes a [Feature][1] or a [FeatureCollection][2] and returns a convex hull [Polygon][3].

Internally this uses
the [convex-hull][4] module that implements a
[monotone chain hull][5].

### Parameters

*   `geojson` **[GeoJSON][6]** input Feature or FeatureCollection
*   `options` **[Object][7]** Optional parameters (optional, default `{}`)

    *   `options.concavity` **[number][8]** 1 - thin shape. Infinity - convex hull. (optional, default `Infinity`)
    *   `options.properties` **[Object][7]** Translate Properties to Feature (optional, default `{}`)

### Examples

```javascript
var points = turf.featureCollection([
  turf.point([10.195312, 43.755225]),
  turf.point([10.404052, 43.8424511]),
  turf.point([10.579833, 43.659924]),
  turf.point([10.360107, 43.516688]),
  turf.point([10.14038, 43.588348]),
  turf.point([10.195312, 43.755225])
]);

var hull = turf.convex(points);

//addToMap
var addToMap = [points, hull]
```

Returns **[Feature][1]<[Polygon][3]>** a convex hull

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.3

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[4]: https://github.com/mikolalysenko/convex-hull

[5]: http://en.wikibooks.org/wiki/Algorithm_Implementation/Geometry/Convex_hull/Monotone_chain

[6]: https://tools.ietf.org/html/rfc7946#section-3

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[8]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/convex
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/destination

## @turf/destination: destination

Takes a [Point][1] and calculates the location of a destination point given a distance in
degrees, radians, miles, or kilometers; and bearing in degrees.
This uses the [Haversine formula][2] to account for global curvature.

### Parameters

*   `origin` **[Coord][3]** starting point
*   `distance` **[number][4]** distance from the origin point
*   `bearing` **[number][4]** ranging from -180 to 180
*   `options` **[Object][5]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][6] (optional, default `'kilometers'`)
    *   `options.properties` **[Object][5]** Translate properties to Point (optional, default `{}`)

### Examples

```javascript
var point = turf.point([-75.343, 39.984]);
var distance = 50;
var bearing = 90;
var options = {units: 'miles'};

var destination = turf.destination(point, distance, bearing, options);

//addToMap
var addToMap = [point, destination]
destination.properties['marker-color'] = '#f00';
point.properties['marker-color'] = '#0f0';
```

Returns **[Feature][7]<[Point][1]>** destination point

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: http://en.wikipedia.org/wiki/Haversine_formula

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://turfjs.org/docs/api/types/Units

[7]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/destination
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/difference

## @turf/difference: difference

Finds the difference between multiple [polygons][1] by clipping the subsequent polygon from the first.

### Parameters

*   `features` **[FeatureCollection][2]<([Polygon][1] | [MultiPolygon][3])>** input Polygon features

### Examples

```javascript
var polygon1 = turf.polygon([[
  [128, -26],
  [141, -26],
  [141, -21],
  [128, -21],
  [128, -26]
]], {
  "fill": "#F00",
  "fill-opacity": 0.1
});
var polygon2 = turf.polygon([[
  [126, -28],
  [140, -28],
  [140, -20],
  [126, -20],
  [126, -28]
]], {
  "fill": "#00F",
  "fill-opacity": 0.1
});

var difference = turf.difference(turf.featureCollection([polygon1, polygon2]));

//addToMap
var addToMap = [polygon1, polygon2, difference];
```

Returns **([Feature][4]<([Polygon][1] | [MultiPolygon][3])> | null)** a Polygon or MultiPolygon feature showing the area of `polygon1` excluding the area of `polygon2` (if empty returns `null`)

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[2]: https://tools.ietf.org/html/rfc7946#section-3.3

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[4]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/difference
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/directional-mean

## @turf/directional-mean: DirectionalMeanLine

Type: [Object][1]

### Properties

*   `cartesianAngle` **[number][2]** the mean angle of all lines. (measure from due earth counterclockwise).
*   `bearingAngle` **[number][2]** the mean angle of all lines. (bearing).
*   `circularVariance` **[number][2]** the extent to which features all point in the same direction.
    the value ranges 0-1, the bigger the value, the more variation in directions between lines.
*   `averageX` **[number][2]** the centroid of all lines.
*   `averageY` **[number][2]** the centroid of all line.
*   `averageLength` **[number][2]** the average length of line.
*   `countOfLines` **[number][2]** the count of features.

## @turf/directional-mean: directionalMean

This module calculate the average angle of a set of lines, measuring the trend of it.
It can be used in both project coordinate system and geography coordinate system.
It can handle segments of line or the whole line.

### Parameters

*   `lines` **[FeatureCollection][3]<[LineString][4]>**&#x20;
*   `options` **[object][1]**  (optional, default `{}`)

    *   `options.planar` **[boolean][5]** whether the spatial reference system is projected or geographical. (optional, default `true`)
    *   `options.segment` **[boolean][5]** whether treat a LineString as a whole or a set of segments. (optional, default `false`)

### Examples

```javascript
var lines = turf.lineStrings([
  [[110, 45], [120, 50]],
  [[100, 50], [115, 55]],
])
var directionalMeanLine = turf.directionalMean(lines);
// => directionalMeanLine
```

Returns **[DirectionalMeanLine][6]** Directional Mean Line

[1]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[3]: https://tools.ietf.org/html/rfc7946#section-3.3

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[6]: #directionalmeanline



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/directional-mean
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/dissolve

## @turf/dissolve: dissolve

Dissolves a FeatureCollection of [Polygon][1] features, filtered by an optional property name:value.
Note that [MultiPolygon][2] features within the collection are not supported

### Parameters

*   `fc` **[FeatureCollection][3]<[Polygon][1]>**&#x20;
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.propertyName` **[string][5]?** features with the same `propertyName` value will be dissolved.
*   `featureCollection` **[FeatureCollection][3]<[Polygon][1]>** input feature collection to be dissolved

### Examples

```javascript
var features = turf.featureCollection([
  turf.polygon([[[0, 0], [0, 1], [1, 1], [1, 0], [0, 0]]], {combine: 'yes'}),
  turf.polygon([[[0, -1], [0, 0], [1, 0], [1, -1], [0,-1]]], {combine: 'yes'}),
  turf.polygon([[[1,-1],[1, 0], [2, 0], [2, -1], [1, -1]]], {combine: 'no'}),
]);

var dissolved = turf.dissolve(features, {propertyName: 'combine'});

//addToMap
var addToMap = [features, dissolved]
```

Returns **[FeatureCollection][3]<[Polygon][1]>** a FeatureCollection containing the dissolved polygons

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[3]: https://tools.ietf.org/html/rfc7946#section-3.3

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/dissolve
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/distance-weight

## @turf/distance-weight: pNormDistance

calcualte the Minkowski p-norm distance between two features.

### Parameters

*   `feature1` **[Feature][1]<[Point][2]>** point feature
*   `feature2` **[Feature][1]<[Point][2]>** point feature
*   `p`  p-norm 1=\<p<=infinity 1: Manhattan distance 2: Euclidean distance (optional, default `2`)

Returns **[number][3]**&#x20;

## @turf/distance-weight: distanceWeight

### Parameters

*   `fc` **[FeatureCollection][4]\<any>** FeatureCollection.
*   `options` **[Object][5]?** option object.

    *   `options.threshold` **[number][3]** If the distance between neighbor and
        target features is greater than threshold, the weight of that neighbor is 0. (optional, default `10000`)
    *   `options.p` **[number][3]** Minkowski p-norm distance parameter.
        1: Manhattan distance. 2: Euclidean distance. 1=\<p<=infinity. (optional, default `2`)
    *   `options.binary` **[boolean][6]** If true, weight=1 if d <= threshold otherwise weight=0.
        If false, weight=Math.pow(d, alpha). (optional, default `false`)
    *   `options.alpha` **[number][3]** distance decay parameter.
        A big value means the weight decay quickly as distance increases. (optional, default `-1`)
    *   `options.standardization` **[boolean][6]** row standardization. (optional, default `false`)

### Examples

```javascript
var bbox = [-65, 40, -63, 42];
var dataset = turf.randomPoint(100, { bbox: bbox });
var result = turf.distanceWeight(dataset);
```

Returns **[Array][7]<[Array][7]<[number][3]>>** distance weight matrix.

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: https://tools.ietf.org/html/rfc7946#section-3.3

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/distance-weight
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/distance

## @turf/distance: distance

Calculates the distance between two [coordinates][1] in degrees, radians, miles, or kilometers.
This uses the [Haversine formula][2] to account for global curvature.

### Parameters

*   `from` **[Coord][1]** origin coordinate
*   `to` **[Coord][1]** destination coordinate
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][4]. (optional, default `'kilometers'`)

### Examples

```javascript
var from = turf.point([-75.343, 39.984]);
var to = turf.point([-75.534, 39.123]);
var options = {units: 'miles'};

var distance = turf.distance(from, to, options);

//addToMap
var addToMap = [from, to];
from.properties.distance = distance;
to.properties.distance = distance;
```

Returns **[number][5]** distance between the two coordinates

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[2]: http://en.wikipedia.org/wiki/Haversine_formula

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://turfjs.org/docs/api/types/Units

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/distance
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/ellipse

## @turf/ellipse: ellipse

Takes a [Point][1] and calculates the ellipse polygon given two semi-axes expressed in variable units and steps for precision.

### Parameters

*   `center` **[Coord][2]** center point
*   `xSemiAxis` **[number][3]** semi (major) axis of the ellipse along the x-axis
*   `ySemiAxis` **[number][3]** semi (minor) axis of the ellipse along the y-axis
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.angle` **[number][3]** angle of rotation in decimal degrees, positive clockwise (optional, default `0`)
    *   `options.pivot` **[Coord][2]** point around which any rotation will be performed (optional, default `center`)
    *   `options.steps` **[number][3]** number of steps (optional, default `64`)
    *   `options.units` **Units** unit of measurement for axes. Supports all valid Turf [Units][5] (optional, default `'kilometers'`)
    *   `options.properties` **[Object][4]** properties (optional, default `{}`)

### Examples

```javascript
var center = [-75, 40];
var xSemiAxis = 5;
var ySemiAxis = 2;
var ellipse = turf.ellipse(center, xSemiAxis, ySemiAxis);

//addToMap
var addToMap = [turf.point(center), ellipse]
```

Returns **[Feature][6]<[Polygon][7]>** ellipse polygon

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://turfjs.org/docs/api/types/Units

[6]: https://tools.ietf.org/html/rfc7946#section-3.2

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.6



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/ellipse
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/envelope

## @turf/envelope: envelope

Takes any number of features and returns a rectangular [Polygon][1] that encompasses all vertices.

### Parameters

*   `geojson` **[GeoJSON][2]** input features

### Examples

```javascript
var features = turf.featureCollection([
  turf.point([-75.343, 39.984], {"name": "Location A"}),
  turf.point([-75.833, 39.284], {"name": "Location B"}),
  turf.point([-75.534, 39.123], {"name": "Location C"})
]);

var enveloped = turf.envelope(features);

//addToMap
var addToMap = [features, enveloped];
```

Returns **[Feature][3]<[Polygon][1]>** a rectangular Polygon feature that encompasses all vertices

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[2]: https://tools.ietf.org/html/rfc7946#section-3

[3]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/envelope
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/explode

## @turf/explode: explode

Takes a feature or set of features and returns all positions as [points][1].

### Parameters

*   `geojson` **[GeoJSON][2]** input features

### Examples

```javascript
var polygon = turf.polygon([[[-81, 41], [-88, 36], [-84, 31], [-80, 33], [-77, 39], [-81, 41]]]);

var explode = turf.explode(polygon);

//addToMap
var addToMap = [polygon, explode]
```

*   Throws **[Error][3]** if it encounters an unknown geometry type

Returns **[FeatureCollection][4]\<point>** points representing the exploded input features

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Error

[4]: https://tools.ietf.org/html/rfc7946#section-3.3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/explode
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/flatten

## @turf/flatten: flatten

Flattens any [GeoJSON][1] to a [FeatureCollection][2] inspired by [geojson-flatten][3].

### Parameters

*   `geojson` **[GeoJSON][1]** any valid GeoJSON Object

### Examples

```javascript
var multiGeometry = turf.multiPolygon([
  [[[102.0, 2.0], [103.0, 2.0], [103.0, 3.0], [102.0, 3.0], [102.0, 2.0]]],
  [[[100.0, 0.0], [101.0, 0.0], [101.0, 1.0], [100.0, 1.0], [100.0, 0.0]],
  [[100.2, 0.2], [100.8, 0.2], [100.8, 0.8], [100.2, 0.8], [100.2, 0.2]]]
]);

var flatten = turf.flatten(multiGeometry);

//addToMap
var addToMap = [flatten]
```

Returns **[FeatureCollection][2]\<any>** all Multi-Geometries are flattened into single Features

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://tools.ietf.org/html/rfc7946#section-3.3

[3]: https://github.com/tmcw/geojson-flatten



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/flatten
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/flip

## @turf/flip: flip

Takes input features and flips all of their coordinates from `[x, y]` to `[y, x]`.

### Parameters

*   `geojson` **[GeoJSON][1]** input features
*   `options` **[Object][2]** Optional parameters (optional, default `{}`)

    *   `options.mutate` **[boolean][3]** allows GeoJSON input to be mutated (significant performance increase if true) (optional, default `false`)

### Examples

```javascript
var serbia = turf.point([20.566406, 43.421008]);

var saudiArabia = turf.flip(serbia);

//addToMap
var addToMap = [serbia, saudiArabia];
```

Returns **[GeoJSON][1]** a feature or set of features of the same type as `input` with flipped coordinates

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/flip
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/geojson-rbush

## @turf/geojson-rbush: rbush

### insert

[insert][1]

#### Parameters

*   `feature` **[Feature][2]** insert single GeoJSON Feature

#### Examples

```javascript
var poly = turf.polygon([[[-78, 41], [-67, 41], [-67, 48], [-78, 48], [-78, 41]]]);
tree.insert(poly)
```

Returns **RBush** GeoJSON RBush

### load

[load][3]

#### Parameters

*   `features` **([FeatureCollection][4] | [Array][5]<[Feature][2]>)** load entire GeoJSON FeatureCollection

#### Examples

```javascript
var polys = turf.polygons([
    [[[-78, 41], [-67, 41], [-67, 48], [-78, 48], [-78, 41]]],
    [[[-93, 32], [-83, 32], [-83, 39], [-93, 39], [-93, 32]]]
]);
tree.load(polys);
```

Returns **RBush** GeoJSON RBush

### remove

[remove][6]

#### Parameters

*   `feature` **[Feature][2]** remove single GeoJSON Feature
*   `equals` **[Function][7]** Pass a custom equals function to compare by value for removal.

#### Examples

```javascript
var poly = turf.polygon([[[-78, 41], [-67, 41], [-67, 48], [-78, 48], [-78, 41]]]);

tree.remove(poly);
```

Returns **RBush** GeoJSON RBush

### clear

[clear][6]

#### Examples

```javascript
tree.clear()
```

Returns **RBush** GeoJSON Rbush

### search

[search][8]

#### Parameters

*   `geojson` **([BBox][9] | [FeatureCollection][4] | [Feature][2])** search with GeoJSON

#### Examples

```javascript
var poly = turf.polygon([[[-78, 41], [-67, 41], [-67, 48], [-78, 48], [-78, 41]]]);

tree.search(poly);
```

Returns **[FeatureCollection][4]** all features that intersects with the given GeoJSON.

### collides

[collides][10]

#### Parameters

*   `geojson` **([BBox][9] | [FeatureCollection][4] | [Feature][2])** collides with GeoJSON

#### Examples

```javascript
var poly = turf.polygon([[[-78, 41], [-67, 41], [-67, 48], [-78, 48], [-78, 41]]]);

tree.collides(poly);
```

Returns **[boolean][11]** true if there are any items intersecting the given GeoJSON, otherwise false.

### all

[all][8]

#### Examples

```javascript
tree.all()
```

Returns **[FeatureCollection][4]** all the features in RBush

### toJSON

[toJSON][12]

#### Examples

```javascript
var exported = tree.toJSON()
```

Returns **any** export data as JSON object

### fromJSON

[fromJSON][12]

#### Parameters

*   `json` **any** import previously exported data

#### Examples

```javascript
var exported = {
  "children": [
    {
      "type": "Feature",
      "geometry": {
        "type": "Point",
        "coordinates": [110, 50]
      },
      "properties": {},
      "bbox": [110, 50, 110, 50]
    }
  ],
  "height": 1,
  "leaf": true,
  "minX": 110,
  "minY": 50,
  "maxX": 110,
  "maxY": 50
}
tree.fromJSON(exported)
```

Returns **RBush** GeoJSON RBush

## @turf/geojson-rbush: rbush

GeoJSON implementation of [RBush][13] spatial index.

### Parameters

*   `maxEntries` **[number][14]** defines the maximum number of entries in a tree node. 9 (used by default) is a
    reasonable choice for most applications. Higher value means faster insertion and slower search, and vice versa. (optional, default `9`)

### Examples

```javascript
var geojsonRbush = require('geojson-rbush').default;
var tree = geojsonRbush();
```

Returns **RBush** GeoJSON RBush

### insert

[insert][1]

#### Parameters

*   `feature` **[Feature][2]** insert single GeoJSON Feature

#### Examples

```javascript
var poly = turf.polygon([[[-78, 41], [-67, 41], [-67, 48], [-78, 48], [-78, 41]]]);
tree.insert(poly)
```

Returns **RBush** GeoJSON RBush

### load

[load][3]

#### Parameters

*   `features` **([FeatureCollection][4] | [Array][5]<[Feature][2]>)** load entire GeoJSON FeatureCollection

#### Examples

```javascript
var polys = turf.polygons([
    [[[-78, 41], [-67, 41], [-67, 48], [-78, 48], [-78, 41]]],
    [[[-93, 32], [-83, 32], [-83, 39], [-93, 39], [-93, 32]]]
]);
tree.load(polys);
```

Returns **RBush** GeoJSON RBush

### remove

[remove][6]

#### Parameters

*   `feature` **[Feature][2]** remove single GeoJSON Feature
*   `equals` **[Function][7]** Pass a custom equals function to compare by value for removal.

#### Examples

```javascript
var poly = turf.polygon([[[-78, 41], [-67, 41], [-67, 48], [-78, 48], [-78, 41]]]);

tree.remove(poly);
```

Returns **RBush** GeoJSON RBush

### clear

[clear][6]

#### Examples

```javascript
tree.clear()
```

Returns **RBush** GeoJSON Rbush

### search

[search][8]

#### Parameters

*   `geojson` **([BBox][9] | [FeatureCollection][4] | [Feature][2])** search with GeoJSON

#### Examples

```javascript
var poly = turf.polygon([[[-78, 41], [-67, 41], [-67, 48], [-78, 48], [-78, 41]]]);

tree.search(poly);
```

Returns **[FeatureCollection][4]** all features that intersects with the given GeoJSON.

### collides

[collides][10]

#### Parameters

*   `geojson` **([BBox][9] | [FeatureCollection][4] | [Feature][2])** collides with GeoJSON

#### Examples

```javascript
var poly = turf.polygon([[[-78, 41], [-67, 41], [-67, 48], [-78, 48], [-78, 41]]]);

tree.collides(poly);
```

Returns **[boolean][11]** true if there are any items intersecting the given GeoJSON, otherwise false.

### all

[all][8]

#### Examples

```javascript
tree.all()
```

Returns **[FeatureCollection][4]** all the features in RBush

### toJSON

[toJSON][12]

#### Examples

```javascript
var exported = tree.toJSON()
```

Returns **any** export data as JSON object

### fromJSON

[fromJSON][12]

#### Parameters

*   `json` **any** import previously exported data

#### Examples

```javascript
var exported = {
  "children": [
    {
      "type": "Feature",
      "geometry": {
        "type": "Point",
        "coordinates": [110, 50]
      },
      "properties": {},
      "bbox": [110, 50, 110, 50]
    }
  ],
  "height": 1,
  "leaf": true,
  "minX": 110,
  "minY": 50,
  "maxX": 110,
  "maxY": 50
}
tree.fromJSON(exported)
```

Returns **RBush** GeoJSON RBush

[1]: https://github.com/mourner/rbush#data-format

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://github.com/mourner/rbush#bulk-inserting-data

[4]: https://tools.ietf.org/html/rfc7946#section-3.3

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array

[6]: https://github.com/mourner/rbush#removing-data

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Statements/function

[8]: https://github.com/mourner/rbush#search

[9]: https://tools.ietf.org/html/rfc7946#section-5

[10]: https://github.com/mourner/rbush#collisions

[11]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[12]: https://github.com/mourner/rbush#export-and-import

[13]: https://github.com/mourner/rbush#rbush

[14]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/geojson-rbush
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/great-circle

## @turf/great-circle: greatCircle

Calculate great circles routes as [LineString][1] or [MultiLineString][2].
If the `start` and `end` points span the antimeridian, the resulting feature will
be split into a `MultiLineString`. If the `start` and `end` positions are the same
then a `LineString` will be returned with duplicate coordinates the length of the `npoints` option.

### Parameters

*   `start` **[Coord][3]** source point feature
*   `end` **[Coord][3]** destination point feature
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.properties` **[Object][4]** line feature properties (optional, default `{}`)
    *   `options.npoints` **[number][5]** number of points (optional, default `100`)
    *   `options.offset` **[number][5]** offset controls the likelyhood that lines will
        be split which cross the dateline. The higher the number the more likely. (optional, default `10`)

### Examples

```javascript
var start = turf.point([-122, 48]);
var end = turf.point([-77, 39]);

var greatCircle = turf.greatCircle(start, end, {properties: {name: 'Seattle to DC'}});

//addToMap
var addToMap = [start, end, greatCircle]
```

Returns **[Feature][6]<([LineString][1] | [MultiLineString][2])>** great circle line feature

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[6]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/great-circle
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/helpers

## @turf/helpers: helpers

## @turf/helpers: Units

Linear measurement units.

⚠️ Warning. Be aware of the implications of using radian or degree units to
measure distance. The distance represented by a degree of longitude *varies*
depending on latitude.

See [https://www.thoughtco.com/degree-of-latitude-and-longitude-distance-4070616][1]
for an illustration of this behaviour.

Type: (`"meters"` | `"metres"` | `"m"` | `"millimeters"` | `"millimetres"` | `"mm"` | `"centimeters"` | `"centimetres"` | `"cm"` | `"kilometers"` | `"kilometres"` | `"km"` | `"miles"` | `"mi"` | `"nauticalmiles"` | `"nmi"` | `"inches"` | `"in"` | `"yards"` | `"yd"` | `"feet"` | `"ft"` | `"radians"` | `"rad"` | `"degrees"` | `"deg"`)

## @turf/helpers: AreaUnits

Area measurement units.

Type: (Exclude<[Units][2], (`"radians"` | `"rad"` | `"degrees"` | `"deg"`)> | `"acres"` | `"ac"` | `"hectares"` | `"ha"`)

## @turf/helpers: Grid

Grid types.

Type: (`"point"` | `"square"` | `"hex"` | `"triangle"`)

## @turf/helpers: Corners

Shorthand corner identifiers.

Type: (`"sw"` | `"se"` | `"nw"` | `"ne"` | `"center"` | `"centroid"`)

## @turf/helpers: Lines

Geometries made up of lines i.e. lines and polygons.

Type: ([LineString][3] | [MultiLineString][4] | [Polygon][5] | [MultiPolygon][6])

## @turf/helpers: AllGeoJSON

Convenience type for all possible GeoJSON.

Type: ([Feature][7] | [FeatureCollection][8] | [Geometry][9] | [GeometryCollection][10])

## @turf/helpers: earthRadius

The Earth radius in meters. Used by Turf modules that model the Earth as a sphere. The [mean radius][11] was selected because it is [recommended ][12] by the Haversine formula (used by turf/distance) to reduce error.

Type: [number][13]

## @turf/helpers: factors

Unit of measurement factors based on earthRadius.

Keys are the name of the unit, values are the number of that unit in a single radian

Type: Record<[Units][2], [number][13]>

## @turf/helpers: areaFactors

Area of measurement factors based on 1 square meter.

Type: Record<[AreaUnits][14], [number][13]>

## @turf/helpers: feature

Wraps a GeoJSON [Geometry][9] in a GeoJSON [Feature][7].

### Parameters

*   `geom` **(G | null)**&#x20;
*   `properties` **[GeoJsonProperties][7]** an Object of key-value pairs to add as properties (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north] associated with the Feature
    *   `options.id` **Id?** Identifier associated with the Feature
*   `geometry` **[GeometryObject][9]** input geometry

### Examples

```javascript
var geometry = {
  "type": "Point",
  "coordinates": [110, 50]
};

var feature = turf.feature(geometry);

//=feature
```

Returns **[Feature][7]<[GeometryObject][9], [GeoJsonProperties][7]>** a GeoJSON Feature

## @turf/helpers: geometry

Creates a GeoJSON [Geometry][9] from a Geometry string type & coordinates.
For GeometryCollection type use `helpers.geometryCollection`

### Parameters

*   `type` **(`"Point"` | `"LineString"` | `"Polygon"` | `"MultiPoint"` | `"MultiLineString"` | `"MultiPolygon"`)** Geometry Type
*   `coordinates` **[Array][17]\<any>** Coordinates
*   `_options` **Record<[string][18], never>**  (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

### Examples

```javascript
var type = "Point";
var coordinates = [110, 50];
var geometry = turf.geometry(type, coordinates);
// => geometry
```

Returns **[Geometry][9]** a GeoJSON Geometry

## @turf/helpers: point

Creates a [Point][19] [Feature][7] from a Position.

### Parameters

*   `coordinates` **[Position][20]** longitude, latitude position (each in decimal degrees)
*   `properties` **[GeoJsonProperties][7]** an Object of key-value pairs to add as properties (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north] associated with the Feature
    *   `options.id` **Id?** Identifier associated with the Feature

### Examples

```javascript
var point = turf.point([-75.343, 39.984]);

//=point
```

Returns **[Feature][7]<[Point][19], [GeoJsonProperties][7]>** a Point feature

## @turf/helpers: points

Creates a [Point][19] [FeatureCollection][8] from an Array of Point coordinates.

### Parameters

*   `coordinates` **[Array][17]<[Position][20]>** an array of Points
*   `properties` **[GeoJsonProperties][7]** Translate these properties to each Feature (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north]
        associated with the FeatureCollection
    *   `options.id` **Id?** Identifier associated with the FeatureCollection

### Examples

```javascript
var points = turf.points([
  [-75, 39],
  [-80, 45],
  [-78, 50]
]);

//=points
```

Returns **[FeatureCollection][8]<[Point][19]>** Point Feature

## @turf/helpers: polygon

Creates a [Polygon][5] [Feature][7] from an Array of LinearRings.

### Parameters

*   `coordinates` **[Array][17]<[Array][17]<[Position][20]>>**&#x20;
*   `properties` **[GeoJsonProperties][7]** an Object of key-value pairs to add as properties (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north] associated with the Feature
    *   `options.id` **Id?** Identifier associated with the Feature

### Examples

```javascript
var polygon = turf.polygon([[[-5, 52], [-4, 56], [-2, 51], [-7, 54], [-5, 52]]], { name: 'poly1' });

//=polygon
```

Returns **[Feature][7]<[Polygon][5], [GeoJsonProperties][7]>** Polygon Feature

## @turf/helpers: polygons

Creates a [Polygon][5] [FeatureCollection][8] from an Array of Polygon coordinates.

### Parameters

*   `coordinates` **[Array][17]<[Array][17]<[Array][17]<[Position][20]>>>**&#x20;
*   `properties` **[GeoJsonProperties][7]** an Object of key-value pairs to add as properties (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north] associated with the Feature
    *   `options.id` **Id?** Identifier associated with the FeatureCollection

### Examples

```javascript
var polygons = turf.polygons([
  [[[-5, 52], [-4, 56], [-2, 51], [-7, 54], [-5, 52]]],
  [[[-15, 42], [-14, 46], [-12, 41], [-17, 44], [-15, 42]]],
]);

//=polygons
```

Returns **[FeatureCollection][8]<[Polygon][5], [GeoJsonProperties][7]>** Polygon FeatureCollection

## @turf/helpers: lineString

Creates a [LineString][3] [Feature][7] from an Array of Positions.

### Parameters

*   `coordinates` **[Array][17]<[Position][20]>** an array of Positions
*   `properties` **[GeoJsonProperties][7]** an Object of key-value pairs to add as properties (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north] associated with the Feature
    *   `options.id` **Id?** Identifier associated with the Feature

### Examples

```javascript
var linestring1 = turf.lineString([[-24, 63], [-23, 60], [-25, 65], [-20, 69]], {name: 'line 1'});
var linestring2 = turf.lineString([[-14, 43], [-13, 40], [-15, 45], [-10, 49]], {name: 'line 2'});

//=linestring1
//=linestring2
```

Returns **[Feature][7]<[LineString][3], [GeoJsonProperties][7]>** LineString Feature

## @turf/helpers: lineStrings

Creates a [LineString][3] [FeatureCollection][8] from an Array of LineString coordinates.

### Parameters

*   `coordinates` **[Array][17]<[Array][17]<[Position][20]>>**&#x20;
*   `properties` **[GeoJsonProperties][7]** an Object of key-value pairs to add as properties (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north]
        associated with the FeatureCollection
    *   `options.id` **Id?** Identifier associated with the FeatureCollection

### Examples

```javascript
var linestrings = turf.lineStrings([
  [[-24, 63], [-23, 60], [-25, 65], [-20, 69]],
  [[-14, 43], [-13, 40], [-15, 45], [-10, 49]]
]);

//=linestrings
```

Returns **[FeatureCollection][8]<[LineString][3], [GeoJsonProperties][7]>** LineString FeatureCollection

## @turf/helpers: featureCollection

Takes one or more [Features][7] and creates a [FeatureCollection][8].

### Parameters

*   `features` **[Array][17]<[Feature][7]<[GeometryObject][9], [GeoJsonProperties][7]>>** input features
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north] associated with the Feature
    *   `options.id` **Id?** Identifier associated with the Feature

### Examples

```javascript
var locationA = turf.point([-75.343, 39.984], {name: 'Location A'});
var locationB = turf.point([-75.833, 39.284], {name: 'Location B'});
var locationC = turf.point([-75.534, 39.123], {name: 'Location C'});

var collection = turf.featureCollection([
  locationA,
  locationB,
  locationC
]);

//=collection
```

Returns **[FeatureCollection][8]<[GeometryObject][9], [GeoJsonProperties][7]>** FeatureCollection of Features

## @turf/helpers: multiLineString

Creates a [Feature][7]<[MultiLineString][4]> based on a
coordinate array. Properties can be added optionally.

### Parameters

*   `coordinates` **[Array][17]<[Array][17]<[Position][20]>>**&#x20;
*   `properties` **[GeoJsonProperties][7]** an Object of key-value pairs to add as properties (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north] associated with the Feature
    *   `options.id` **Id?** Identifier associated with the Feature

### Examples

```javascript
var multiLine = turf.multiLineString([[[0,0],[10,10]]]);

//=multiLine
```

*   Throws **[Error][21]** if no coordinates are passed

Returns **[Feature][7]<[MultiLineString][4], [GeoJsonProperties][7]>** a MultiLineString feature

## @turf/helpers: multiPoint

Creates a [Feature][7]<[MultiPoint][22]> based on a
coordinate array. Properties can be added optionally.

### Parameters

*   `coordinates` **[Array][17]<[Position][20]>** an array of Positions
*   `properties` **[GeoJsonProperties][7]** an Object of key-value pairs to add as properties (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north] associated with the Feature
    *   `options.id` **Id?** Identifier associated with the Feature

### Examples

```javascript
var multiPt = turf.multiPoint([[0,0],[10,10]]);

//=multiPt
```

*   Throws **[Error][21]** if no coordinates are passed

Returns **[Feature][7]<[MultiPoint][22], [GeoJsonProperties][7]>** a MultiPoint feature

## @turf/helpers: multiPolygon

Creates a [Feature][7]<[MultiPolygon][6]> based on a
coordinate array. Properties can be added optionally.

### Parameters

*   `coordinates` **[Array][17]<[Array][17]<[Array][17]<[Position][20]>>>**&#x20;
*   `properties` **[GeoJsonProperties][7]** an Object of key-value pairs to add as properties (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north] associated with the Feature
    *   `options.id` **Id?** Identifier associated with the Feature

### Examples

```javascript
var multiPoly = turf.multiPolygon([[[[0,0],[0,10],[10,10],[10,0],[0,0]]]]);

//=multiPoly
```

*   Throws **[Error][21]** if no coordinates are passed

Returns **[Feature][7]<[MultiPolygon][6], [GeoJsonProperties][7]>** a multipolygon feature

## @turf/helpers: geometryCollection

Creates a Feature<GeometryCollection> based on a
coordinate array. Properties can be added optionally.

### Parameters

*   `geometries` **[Array][17]<([Point][19] | [LineString][3] | [Polygon][5] | [MultiPoint][22] | [MultiLineString][4] | [MultiPolygon][6])>** an array of GeoJSON Geometries
*   `properties` **[GeoJsonProperties][7]** an Object of key-value pairs to add as properties (optional, default `{}`)
*   `options` **[Object][15]** Optional Parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][16]?** Bounding Box Array \[west, south, east, north] associated with the Feature
    *   `options.id` **Id?** Identifier associated with the Feature

### Examples

```javascript
var pt = turf.geometry("Point", [100, 0]);
var line = turf.geometry("LineString", [[101, 0], [102, 1]]);
var collection = turf.geometryCollection([pt, line]);

// => collection
```

Returns **[Feature][7]<[GeometryCollection][10], [GeoJsonProperties][7]>** a GeoJSON GeometryCollection Feature

## @turf/helpers: round

Round number to precision

### Parameters

*   `num` **[number][13]** Number
*   `precision` **[number][13]** Precision (optional, default `0`)

### Examples

```javascript
turf.round(120.4321)
//=120

turf.round(120.4321, 2)
//=120.43
```

Returns **[number][13]** rounded number

## @turf/helpers: radiansToLength

Convert a distance measurement (assuming a spherical Earth) from radians to a more friendly unit.
Valid units: miles, nauticalmiles, inches, yards, meters, metres, kilometers, centimeters, feet

### Parameters

*   `radians` **[number][13]** in radians across the sphere
*   `units` **[Units][2]** can be degrees, radians, miles, inches, yards, metres,
    meters, kilometres, kilometers. (optional, default `"kilometers"`)

Returns **[number][13]** distance

## @turf/helpers: lengthToRadians

Convert a distance measurement (assuming a spherical Earth) from a real-world unit into radians
Valid units: miles, nauticalmiles, inches, yards, meters, metres, kilometers, centimeters, feet

### Parameters

*   `distance` **[number][13]** in real units
*   `units` **[Units][2]** can be degrees, radians, miles, inches, yards, metres,
    meters, kilometres, kilometers. (optional, default `"kilometers"`)

Returns **[number][13]** radians

## @turf/helpers: lengthToDegrees

Convert a distance measurement (assuming a spherical Earth) from a real-world unit into degrees
Valid units: miles, nauticalmiles, inches, yards, meters, metres, centimeters, kilometres, feet

### Parameters

*   `distance` **[number][13]** in real units
*   `units` **[Units][2]** can be degrees, radians, miles, inches, yards, metres,
    meters, kilometres, kilometers. (optional, default `"kilometers"`)

Returns **[number][13]** degrees

## @turf/helpers: bearingToAzimuth

Converts any bearing angle from the north line direction (positive clockwise)
and returns an angle between 0-360 degrees (positive clockwise), 0 being the north line

### Parameters

*   `bearing` **[number][13]** angle, between -180 and +180 degrees

Returns **[number][13]** angle between 0 and 360 degrees

## @turf/helpers: azimuthToBearing

Converts any azimuth angle from the north line direction (positive clockwise)
and returns an angle between -180 and +180 degrees (positive clockwise), 0 being the north line

### Parameters

*   `angle` **[number][13]** between 0 and 360 degrees

Returns **[number][13]** bearing between -180 and +180 degrees

## @turf/helpers: radiansToDegrees

Converts an angle in radians to degrees

### Parameters

*   `radians` **[number][13]** angle in radians

Returns **[number][13]** degrees between 0 and 360 degrees

## @turf/helpers: degreesToRadians

Converts an angle in degrees to radians

### Parameters

*   `degrees` **[number][13]** angle between 0 and 360 degrees

Returns **[number][13]** angle in radians

## @turf/helpers: convertLength

Converts a length from one unit to another.

### Parameters

*   `length` **[number][13]** Length to be converted
*   `originalUnit` **[Units][2]** Input length unit (optional, default `"kilometers"`)
*   `finalUnit` **[Units][2]** Returned length unit (optional, default `"kilometers"`)

Returns **[number][13]** The converted length

## @turf/helpers: convertArea

Converts an area from one unit to another.

### Parameters

*   `area` **[number][13]** Area to be converted
*   `originalUnit` **[AreaUnits][14]** Input area unit (optional, default `"meters"`)
*   `finalUnit` **[AreaUnits][14]** Returned area unit (optional, default `"kilometers"`)

Returns **[number][13]** The converted length

## @turf/helpers: isNumber

isNumber

### Parameters

*   `num` **any** Number to validate

### Examples

```javascript
turf.isNumber(123)
//=true
turf.isNumber('foo')
//=false
```

Returns **[boolean][23]** true/false

## @turf/helpers: isObject

isObject

### Parameters

*   `input` **any** variable to validate

### Examples

```javascript
turf.isObject({elevation: 10})
//=true
turf.isObject('foo')
//=false
```

Returns **[boolean][23]** true/false, including false for Arrays and Functions

## @turf/helpers: removeBbox

Recursively removes bounding boxes from a GeoJSON object.

This function mutates the input GeoJSON object.

### Parameters

*   `geojson` **[GeoJSON][24]** GeoJSON object whose bounding boxes should be removed

Returns **void**&#x20;

[1]: https://www.thoughtco.com/degree-of-latitude-and-longitude-distance-4070616

[2]: #units

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[7]: https://tools.ietf.org/html/rfc7946#section-3.2

[8]: https://tools.ietf.org/html/rfc7946#section-3.3

[9]: https://tools.ietf.org/html/rfc7946#section-3.1

[10]: https://tools.ietf.org/html/rfc7946#section-3.1.8

[11]: https://en.wikipedia.org/wiki/Earth_radius#Arithmetic_mean_radius

[12]: https://rosettacode.org/wiki/Haversine_formula#:~:text=This%20value%20is%20recommended

[13]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[14]: #areaunits

[15]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[16]: https://tools.ietf.org/html/rfc7946#section-5

[17]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array

[18]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[19]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[20]: https://developer.mozilla.org/docs/Web/API/Position

[21]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Error

[22]: https://tools.ietf.org/html/rfc7946#section-3.1.3

[23]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[24]: https://tools.ietf.org/html/rfc7946#section-3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/helpers
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/hex-grid

## @turf/hex-grid: hexGrid

Takes a bounding box and the diameter of the cell and returns a [FeatureCollection][1] of flat-topped
hexagons or triangles ([Polygon][2] features) aligned in an "odd-q" vertical grid as
described in [Hexagonal Grids][3].

### Parameters

*   `bbox` **[BBox][4]** extent in \[minX, minY, maxX, maxY] order
*   `cellSide` **[number][5]** length of the side of the the hexagons or triangles, in units. It will also coincide with the
    radius of the circumcircle of the hexagons.
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** used in calculating cell size. Supports all valid Turf [Units][7]. (optional, default `'kilometers'`)
    *   `options.properties` **[Object][6]** passed to each hexagon or triangle of the grid (optional, default `{}`)
    *   `options.mask` **[Feature][8]<([Polygon][2] | [MultiPolygon][9])>?** if passed a Polygon or MultiPolygon, the grid Points will be created only inside it
    *   `options.triangles` **[boolean][10]** whether to return as triangles instead of hexagons (optional, default `false`)

### Examples

```javascript
var bbox = [-96,31,-84,40];
var cellSide = 50;
var options = {units: 'miles'};

var hexgrid = turf.hexGrid(bbox, cellSide, options);

//addToMap
var addToMap = [hexgrid];
```

Returns **[FeatureCollection][1]<[Polygon][2]>** a hexagonal grid

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[3]: http://www.redblobgames.com/grids/hexagons/

[4]: https://tools.ietf.org/html/rfc7946#section-5

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://turfjs.org/docs/api/types/Units

[8]: https://tools.ietf.org/html/rfc7946#section-3.2

[9]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[10]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/hex-grid
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/interpolate

## @turf/interpolate: interpolate

Takes a set of points and estimates their 'property' values on a grid using the [Inverse Distance Weighting (IDW) method][1].

### Parameters

*   `points` **[FeatureCollection][2]<[Point][3]>** with known value
*   `cellSize` **[number][4]** the distance across each grid point
*   `options` **[Object][5]** Optional parameters (optional, default `{}`)

    *   `options.gridType` **[string][6]** defines the output format based on a Grid Type (options: 'square' | 'point' | 'hex' | 'triangle') (optional, default `'square'`)
    *   `options.property` **[string][6]** the property name in `points` from which z-values will be pulled, zValue fallbacks to 3rd coordinate if no property exists. (optional, default `'elevation'`)
    *   `options.units` **Units** used in calculating cellSize. Supports all valid Turf [Units][7]. (optional, default `'kilometers'`)
    *   `options.weight` **[number][4]** exponent regulating the distance-decay weighting (optional, default `1`)
    *   `options.bbox` **[BBox][8]** Bounding Box Array \[west, south, east, north] associated with the FeatureCollection. (optional, default `bbox(points)`)

### Examples

```javascript
var points = turf.randomPoint(30, {bbox: [50, 30, 70, 50]});

// add a random property to each point
turf.featureEach(points, function(point) {
    point.properties.solRad = Math.random() * 50;
});
var options = {gridType: 'points', property: 'solRad', units: 'miles'};
var grid = turf.interpolate(points, 100, options);

//addToMap
var addToMap = [grid];
```

Returns **[FeatureCollection][2]<([Point][3] | [Polygon][9])>** grid of points or polygons with interpolated 'property'

[1]: https://en.wikipedia.org/wiki/Inverse_distance_weighting

[2]: https://tools.ietf.org/html/rfc7946#section-3.3

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[7]: https://turfjs.org/docs/api/types/Units

[8]: https://tools.ietf.org/html/rfc7946#section-5

[9]: https://tools.ietf.org/html/rfc7946#section-3.1.6



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/interpolate
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/intersect

## @turf/intersect: intersect

Takes [polygon][1] or [multi-polygon][2] geometries and
finds their polygonal intersection. If they don't intersect, returns null.

### Parameters

*   `features` **[FeatureCollection][3]<([Polygon][1] | [MultiPolygon][2])>** the features to intersect
*   `options` **[Object][4]** Optional Parameters (optional, default `{}`)

    *   `options.properties` **[Object][4]** Translate GeoJSON Properties to Feature (optional, default `{}`)

### Examples

```javascript
var poly1 = turf.polygon([[
  [-122.801742, 45.48565],
  [-122.801742, 45.60491],
  [-122.584762, 45.60491],
  [-122.584762, 45.48565],
  [-122.801742, 45.48565]
]]);

var poly2 = turf.polygon([[
  [-122.520217, 45.535693],
  [-122.64038, 45.553967],
  [-122.720031, 45.526554],
  [-122.669906, 45.507309],
  [-122.723464, 45.446643],
  [-122.532577, 45.408574],
  [-122.487258, 45.477466],
  [-122.520217, 45.535693]
]]);

var intersection = turf.intersect(turf.featureCollection([poly1, poly2]));

//addToMap
var addToMap = [poly1, poly2, intersection];
```

Returns **([Feature][5] | null)** returns a feature representing the area they share (either a [Polygon][1] or
[MultiPolygon][2]). If they do not share any area, returns `null`.

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[3]: https://tools.ietf.org/html/rfc7946#section-3.3

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/intersect
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/invariant

## @turf/invariant: getCoord

Unwrap a coordinate from a Point Feature, Geometry or a single coordinate.

### Parameters

*   `coord` **([Array][1]<[number][2]> | [Geometry][3]<[Point][4]> | [Feature][5]<[Point][4]>)** GeoJSON Point or an Array of numbers

### Examples

```javascript
var pt = turf.point([10, 10]);

var coord = turf.getCoord(pt);
//= [10, 10]
```

Returns **[Array][1]<[number][2]>** coordinates

## @turf/invariant: getCoords

Unwrap coordinates from a Feature, Geometry Object or an Array

### Parameters

*   `coords` **([Array][1]\<any> | [Geometry][3] | [Feature][5])** Feature, Geometry Object or an Array

### Examples

```javascript
var poly = turf.polygon([[[119.32, -8.7], [119.55, -8.69], [119.51, -8.54], [119.32, -8.7]]]);

var coords = turf.getCoords(poly);
//= [[[119.32, -8.7], [119.55, -8.69], [119.51, -8.54], [119.32, -8.7]]]
```

Returns **[Array][1]\<any>** coordinates

## @turf/invariant: containsNumber

Checks if coordinates contains a number

### Parameters

*   `coordinates` **[Array][1]\<any>** GeoJSON Coordinates

Returns **[boolean][6]** true if Array contains a number

## @turf/invariant: geojsonType

Enforce expectations about types of GeoJSON objects for Turf.

### Parameters

*   `value` **[GeoJSON][7]** any GeoJSON object
*   `type` **[string][8]** expected GeoJSON type
*   `name` **[string][8]** name of calling function



*   Throws **[Error][9]** if value is not the expected type.

Returns **void**&#x20;

## @turf/invariant: featureOf

Enforce expectations about types of [Feature][5] inputs for Turf.
Internally this uses [geojsonType][10] to judge geometry types.

### Parameters

*   `feature` **[Feature][5]** a feature with an expected geometry type
*   `type` **[string][8]** expected GeoJSON type
*   `name` **[string][8]** name of calling function



*   Throws **[Error][9]** error if value is not the expected type.

Returns **void**&#x20;

## @turf/invariant: collectionOf

Enforce expectations about types of [FeatureCollection][11] inputs for Turf.
Internally this uses [geojsonType][10] to judge geometry types.

### Parameters

*   `featureCollection` **[FeatureCollection][11]** a FeatureCollection for which features will be judged
*   `type` **[string][8]** expected GeoJSON type
*   `name` **[string][8]** name of calling function



*   Throws **[Error][9]** if value is not the expected type.

## @turf/invariant: getGeom

Get Geometry from Feature or Geometry Object

### Parameters

*   `geojson` **([Feature][5] | [Geometry][3])** GeoJSON Feature or Geometry Object

### Examples

```javascript
var point = {
  "type": "Feature",
  "properties": {},
  "geometry": {
    "type": "Point",
    "coordinates": [110, 40]
  }
}
var geom = turf.getGeom(point)
//={"type": "Point", "coordinates": [110, 40]}
```

*   Throws **[Error][9]** if geojson is not a Feature or Geometry Object

Returns **([Geometry][3] | null)** GeoJSON Geometry Object

## @turf/invariant: getType

Get GeoJSON object's type, Geometry type is prioritize.

### Parameters

*   `geojson` **[GeoJSON][7]** GeoJSON object
*   `_name` **[string][8]?**&#x20;
*   `name` **[string][8]** name of the variable to display in error message (unused) (optional, default `"geojson"`)

### Examples

```javascript
var point = {
  "type": "Feature",
  "properties": {},
  "geometry": {
    "type": "Point",
    "coordinates": [110, 40]
  }
}
var geom = turf.getType(point)
//="Point"
```

Returns **[string][8]** GeoJSON type

[1]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[3]: https://tools.ietf.org/html/rfc7946#section-3.1

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[7]: https://tools.ietf.org/html/rfc7946#section-3

[8]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[9]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Error

[10]: #geojsontype

[11]: https://tools.ietf.org/html/rfc7946#section-3.3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/invariant
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/isobands

## @turf/isobands: isobands

Takes a square or rectangular grid [FeatureCollection][1] of [Point][2] features with z-values and an array of
value breaks and generates filled contour isobands.

### Parameters

*   `pointGrid` **[FeatureCollection][1]<[Point][2]>** input points - must be square or rectangular and already gridded. That is, to have consistent x and y dimensions and be at least 2x2 in size.
*   `breaks` **[Array][3]<[number][4]>** where to draw contours
*   `options` **[Object][5]** options on output (optional, default `{}`)

    *   `options.zProperty` **[string][6]** the property name in `points` from which z-values will be pulled (optional, default `'elevation'`)
    *   `options.commonProperties` **[Object][5]** GeoJSON properties passed to ALL isobands (optional, default `{}`)
    *   `options.breaksProperties` **[Array][3]<[Object][5]>** GeoJSON properties passed, in order, to the correspondent isoband (order defined by breaks) (optional, default `[]`)

Returns **[FeatureCollection][1]<[MultiPolygon][7]>** a FeatureCollection of [MultiPolygon][7] features representing isobands

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.7



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/isobands
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/isolines

## @turf/isolines: isolines

Takes a grid [FeatureCollection][1] of [Point][2] features with z-values and an array of
value breaks and generates [isolines][3].

### Parameters

*   `pointGrid` **[FeatureCollection][1]<[Point][2]>** input points - must be square or rectangular and already gridded. That is, to have consistent x and y dimensions and be at least 2x2 in size.
*   `breaks` **[Array][4]<[number][5]>** values of `zProperty` where to draw isolines
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.zProperty` **[string][7]** the property name in `points` from which z-values will be pulled (optional, default `'elevation'`)
    *   `options.commonProperties` **[Object][6]** GeoJSON properties passed to ALL isolines (optional, default `{}`)
    *   `options.breaksProperties` **[Array][4]<[Object][6]>** GeoJSON properties passed, in order, to the correspondent isoline;
        the breaks array will define the order in which the isolines are created (optional, default `[]`)

### Examples

```javascript
// create a grid of points with random z-values in their properties
var extent = [0, 30, 20, 50];
var cellWidth = 100;
var pointGrid = turf.pointGrid(extent, cellWidth, {units: 'miles'});

for (var i = 0; i < pointGrid.features.length; i++) {
    pointGrid.features[i].properties.temperature = Math.random() * 10;
}
var breaks = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

var lines = turf.isolines(pointGrid, breaks, {zProperty: 'temperature'});

//addToMap
var addToMap = [lines];
```

Returns **[FeatureCollection][1]<[MultiLineString][8]>** a FeatureCollection of [MultiLineString][8] features representing isolines

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://en.wikipedia.org/wiki/Contour_line

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[8]: https://tools.ietf.org/html/rfc7946#section-3.1.5



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/isolines
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/kinks

## @turf/kinks: kinks

Takes a [linestring][1], [multi-linestring][2],
[multi-polygon][3] or [polygon][4] and
returns [points][5] at all self-intersections.

### Parameters

*   `featureIn` **[Feature][6]<([LineString][1] | [MultiLineString][2] | [MultiPolygon][3] | [Polygon][4])>** input feature

### Examples

```javascript
var poly = turf.polygon([[
  [-12.034835, 8.901183],
  [-12.060413, 8.899826],
  [-12.03638, 8.873199],
  [-12.059383, 8.871418],
  [-12.034835, 8.901183]
]]);

var kinks = turf.kinks(poly);

//addToMap
var addToMap = [poly, kinks]
```

Returns **[FeatureCollection][7]<[Point][5]>** self-intersections

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[6]: https://tools.ietf.org/html/rfc7946#section-3.2

[7]: https://tools.ietf.org/html/rfc7946#section-3.3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/kinks
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/length

## @turf/length: length

Takes a [GeoJSON][1] and measures its length in the specified units, [(Multi)Point][2]'s distance are ignored.

### Parameters

*   `geojson` **[Feature][3]<([LineString][4] | [MultiLineString][5])>** GeoJSON to measure
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][7]. (optional, default `kilometers`)

### Examples

```javascript
var line = turf.lineString([[115, -32], [131, -22], [143, -25], [150, -34]]);
var length = turf.length(line, {units: 'miles'});

//addToMap
var addToMap = [line];
line.properties.distance = length;
```

Returns **[number][8]** length of GeoJSON

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://turfjs.org/docs/api/types/Units

[8]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/length
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/line-arc

## @turf/line-arc: lineArc

Creates a circular arc, of a circle of the given radius and center point, between bearing1 and bearing2;
0 bearing is North of center point, positive clockwise.

### Parameters

*   `center` **[Coord][1]** center point
*   `radius` **[number][2]** radius of the circle
*   `bearing1` **[number][2]** angle, in decimal degrees, of the first radius of the arc
*   `bearing2` **[number][2]** angle, in decimal degrees, of the second radius of the arc
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.steps` **[number][2]** number of steps (straight segments) that will constitute the arc (optional, default `64`)
    *   `options.units` **Units** Supports all valid Turf [Units][4]. (optional, default `'kilometers'`)

### Examples

```javascript
var center = turf.point([-75, 40]);
var radius = 5;
var bearing1 = 25;
var bearing2 = 47;

var arc = turf.lineArc(center, radius, bearing1, bearing2);

//addToMap
var addToMap = [center, arc]
```

Returns **[Feature][5]<[LineString][6]>** line arc

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://turfjs.org/docs/api/types/Units

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.4



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/line-arc
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/line-chunk

## @turf/line-chunk: lineChunk

Divides a [LineString][1] into chunks of a specified length.
If the line is shorter than the segment length then the original line is returned.

### Parameters

*   `geojson` **([FeatureCollection][2] | [Geometry][3] | [Feature][4]<([LineString][1] | [MultiLineString][5])>)** the lines to split
*   `segmentLength` **[number][6]** how long to make each segment
*   `options` **[Object][7]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][8] (optional, default `'kilometers'`)
    *   `options.reverse` **[boolean][9]** reverses coordinates to start the first chunked segment at the end (optional, default `false`)

### Examples

```javascript
var line = turf.lineString([[-95, 40], [-93, 45], [-85, 50]]);

var chunk = turf.lineChunk(line, 15, {units: 'miles'});

//addToMap
var addToMap = [chunk];
```

Returns **[FeatureCollection][2]<[LineString][1]>** collection of line segments

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: https://tools.ietf.org/html/rfc7946#section-3.3

[3]: https://tools.ietf.org/html/rfc7946#section-3.1

[4]: https://tools.ietf.org/html/rfc7946#section-3.2

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[8]: https://turfjs.org/docs/api/types/Units

[9]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/line-chunk
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/line-intersect

## @turf/line-intersect: lineIntersect

Takes any LineString or Polygon GeoJSON and returns the intersecting point(s).

### Parameters

*   `line1` **[GeoJSON][1]** any LineString or Polygon
*   `line2` **[GeoJSON][1]** any LineString or Polygon
*   `options` **[Object][2]** Optional parameters (optional, default `{}`)

    *   `options.removeDuplicates` **[boolean][3]** remove duplicate intersections (optional, default `true`)
    *   `options.ignoreSelfIntersections` **[boolean][3]** ignores self-intersections on input features (optional, default `true`)

### Examples

```javascript
var line1 = turf.lineString([[126, -11], [129, -21]]);
var line2 = turf.lineString([[123, -18], [131, -14]]);
var intersects = turf.lineIntersect(line1, line2);

//addToMap
var addToMap = [line1, line2, intersects]
```

Returns **[FeatureCollection][4]<[Point][5]>** point(s) that intersect both

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[4]: https://tools.ietf.org/html/rfc7946#section-3.3

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/line-intersect
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/line-offset

## @turf/line-offset: lineOffset

Takes a [line][1] and returns a [line][1] at offset by the specified distance.

### Parameters

*   `geojson` **([Geometry][2] | [Feature][3]<([LineString][1] | [MultiLineString][4])>)** input GeoJSON
*   `distance` **[number][5]** distance to offset the line (can be of negative value)
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][7]. (optional, default `'kilometers'`)

### Examples

```javascript
var line = turf.lineString([[-83, 30], [-84, 36], [-78, 41]], { "stroke": "#F00" });

var offsetLine = turf.lineOffset(line, 2, {units: 'miles'});

//addToMap
var addToMap = [offsetLine, line]
offsetLine.properties.stroke = "#00F"
```

Returns **[Feature][3]<([LineString][1] | [MultiLineString][4])>** Line offset from the input line

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: https://tools.ietf.org/html/rfc7946#section-3.1

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://turfjs.org/docs/api/types/Units



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/line-offset
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/line-overlap

## @turf/line-overlap: lineOverlap

Takes any LineString or Polygon and returns the overlapping lines between both features.

### Parameters

*   `line1` **([Geometry][1] | [Feature][2]<([LineString][3] | [MultiLineString][4] | [Polygon][5] | [MultiPolygon][6])>)** any LineString or Polygon
*   `line2` **([Geometry][1] | [Feature][2]<([LineString][3] | [MultiLineString][4] | [Polygon][5] | [MultiPolygon][6])>)** any LineString or Polygon
*   `options` **[Object][7]** Optional parameters (optional, default `{}`)

    *   `options.tolerance` **[number][8]** Tolerance distance to match overlapping line segments (in kilometers) (optional, default `0`)

### Examples

```javascript
var line1 = turf.lineString([[115, -35], [125, -30], [135, -30], [145, -35]]);
var line2 = turf.lineString([[115, -25], [125, -30], [135, -30], [145, -25]]);

var overlapping = turf.lineOverlap(line1, line2);

//addToMap
var addToMap = [line1, line2, overlapping]
```

Returns **[FeatureCollection][9]<[LineString][3]>** lines(s) that are overlapping between both features

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[8]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[9]: https://tools.ietf.org/html/rfc7946#section-3.3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/line-overlap
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/line-segment

## @turf/line-segment: lineSegment

Creates a [FeatureCollection][1] of 2-vertex [LineString][2] segments from a
[(Multi)LineString][2] or [(Multi)Polygon][3].

### Parameters

*   `geojson` **[GeoJSON][4]** GeoJSON Polygon or LineString

### Examples

```javascript
var polygon = turf.polygon([[[-50, 5], [-40, -10], [-50, -10], [-40, 5], [-50, 5]]]);
var segments = turf.lineSegment(polygon);

//addToMap
var addToMap = [polygon, segments]
```

Returns **[FeatureCollection][1]<[LineString][2]>** 2-vertex line segments

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[4]: https://tools.ietf.org/html/rfc7946#section-3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/line-segment
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/line-slice-along

## @turf/line-slice-along: lineSliceAlong

Takes a [line][1], a specified distance along the line to a start [Point][2],
and a specified  distance along the line to a stop point
and returns a subsection of the line in-between those points.

This can be useful for extracting only the part of a route between two distances.

### Parameters

*   `line` **([Feature][3]<[LineString][1]> | [LineString][1])** input line
*   `startDist` **[number][4]** distance along the line to starting point
*   `stopDist` **[number][4]** distance along the line to ending point
*   `options` **[Object][5]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][6] (optional, default `'kilometers'`)

### Examples

```javascript
var line = turf.lineString([[7, 45], [9, 45], [14, 40], [14, 41]]);
var start = 12.5;
var stop = 25;
var sliced = turf.lineSliceAlong(line, start, stop, {units: 'miles'});

//addToMap
var addToMap = [line, start, stop, sliced]
```

Returns **[Feature][3]<[LineString][1]>** sliced line

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://turfjs.org/docs/api/types/Units



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/line-slice-along
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/line-slice

## @turf/line-slice: lineSlice

Takes a [line][1], a start [Point][2], and a stop point
and returns a subsection of the line in-between those points.
The start & stop points don't need to fall exactly on the line.

This can be useful for extracting only the part of a route between waypoints.

### Parameters

*   `startPt` **[Coord][3]** starting point
*   `stopPt` **[Coord][3]** stopping point
*   `line` **([Feature][4]<[LineString][1]> | [LineString][1])** line to slice

### Examples

```javascript
var line = turf.lineString([
    [-77.031669, 38.878605],
    [-77.029609, 38.881946],
    [-77.020339, 38.884084],
    [-77.025661, 38.885821],
    [-77.021884, 38.889563],
    [-77.019824, 38.892368]
]);
var start = turf.point([-77.029609, 38.881946]);
var stop = turf.point([-77.021884, 38.889563]);

var sliced = turf.lineSlice(start, stop, line);

//addToMap
var addToMap = [start, stop, line]
```

Returns **[Feature][4]<[LineString][1]>** sliced line

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[4]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/line-slice
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/line-split

## @turf/line-split: lineSplit

Split a LineString by another GeoJSON Feature.

### Parameters

*   `line` **[Feature][1]<[LineString][2]>** LineString Feature to split
*   `splitter` **[Feature][1]\<any>** Feature used to split line

### Examples

```javascript
var line = turf.lineString([[120, -25], [145, -25]]);
var splitter = turf.lineString([[130, -15], [130, -35]]);

var split = turf.lineSplit(line, splitter);

//addToMap
var addToMap = [line, splitter, split]

split.features[0].properties.stroke = "red";
split.features[1].properties.stroke = "blue";
```

Returns **[FeatureCollection][3]<[LineString][2]>** Split LineStrings

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[3]: https://tools.ietf.org/html/rfc7946#section-3.3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/line-split
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/line-to-polygon

## @turf/line-to-polygon: lineToPolygon

Converts (Multi)LineString(s) to Polygon(s).

### Parameters

*   `lines` **([FeatureCollection][1] | [Feature][2]<([LineString][3] | [MultiLineString][4])>)** Features to convert
*   `options` **[Object][5]** Optional parameters (optional, default `{}`)

    *   `options.properties` **[Object][5]** translates GeoJSON properties to Feature (optional, default `{}`)
    *   `options.autoComplete` **[boolean][6]** auto complete linestrings (matches first & last coordinates) (optional, default `true`)
    *   `options.orderCoords` **[boolean][6]** sorts linestrings to place outer ring at the first position of the coordinates (optional, default `true`)
    *   `options.mutate` **[boolean][6]** mutate the original linestring using autoComplete (matches first & last coordinates) (optional, default `false`)

### Examples

```javascript
var line = turf.lineString([[125, -30], [145, -30], [145, -20], [125, -20], [125, -30]]);

var polygon = turf.lineToPolygon(line);

//addToMap
var addToMap = [polygon];
```

Returns **[Feature][2]<([Polygon][7] | [MultiPolygon][8])>** converted to Polygons

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[8]: https://tools.ietf.org/html/rfc7946#section-3.1.7



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/line-to-polygon
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/mask

## @turf/mask: mask

Takes polygons or multipolygons and an optional mask, and returns an exterior
ring polygon with holes.

### Parameters

*   `polygon` **([Polygon][1] | [MultiPolygon][2] | [Feature][3]<([Polygon][1] | [MultiPolygon][2])> | [FeatureCollection][4]<([Polygon][1] | [MultiPolygon][2])>)** GeoJSON polygon used as interior rings or holes
*   `mask` **([Polygon][1] | [Feature][3]<[Polygon][1]>)?** GeoJSON polygon used as the exterior ring (if undefined, the world extent is used)
*   `options` **[Object][5]** Optional parameters (optional, default `{}`)

    *   `options.mutate` **[boolean][6]** allows the `mask` GeoJSON input to be mutated (performance improvement if true) (optional, default `false`)

### Examples

```javascript
const polygon = turf.polygon([[[112, -21], [116, -36], [146, -39], [153, -24], [133, -10], [112, -21]]]);
const mask = turf.polygon([[[90, -55], [170, -55], [170, 10], [90, 10], [90, -55]]]);

const masked = turf.mask(polygon, mask);

//addToMap
const addToMap = [masked]
```

Returns **[Feature][3]<[Polygon][1]>** Masked Polygon (exterior ring with holes)

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://tools.ietf.org/html/rfc7946#section-3.3

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/mask
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/meta

## @turf/meta: coordEachCallback

Callback for coordEach

Type: [Function][1]

### Parameters

*   `currentCoord` **[Array][2]<[number][3]>** The current coordinate being processed.
*   `coordIndex` **[number][3]** The current index of the coordinate being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.
*   `multiFeatureIndex` **[number][3]** The current index of the Multi-Feature being processed.
*   `geometryIndex` **[number][3]** The current index of the Geometry being processed.

Returns **void**&#x20;

## @turf/meta: coordEach

Iterate over coordinates in any GeoJSON object, similar to Array.forEach()

### Parameters

*   `geojson` **AllGeoJSON** any GeoJSON object
*   `callback` **[coordEachCallback][4]** a method that takes (currentCoord, coordIndex, featureIndex, multiFeatureIndex)
*   `excludeWrapCoord` **[boolean][5]** whether or not to include the final coordinate of LinearRings that wraps the ring in its iteration. (optional, default `false`)

### Examples

```javascript
var features = turf.featureCollection([
  turf.point([26, 37], {"foo": "bar"}),
  turf.point([36, 53], {"hello": "world"})
]);

turf.coordEach(features, function (currentCoord, coordIndex, featureIndex, multiFeatureIndex, geometryIndex) {
  //=currentCoord
  //=coordIndex
  //=featureIndex
  //=multiFeatureIndex
  //=geometryIndex
});
```

Returns **void**&#x20;

## @turf/meta: coordReduceCallback

Callback for coordReduce

The first time the callback function is called, the values provided as arguments depend
on whether the reduce method has an initialValue argument.

If an initialValue is provided to the reduce method:

*   The previousValue argument is initialValue.
*   The currentValue argument is the value of the first element present in the array.

If an initialValue is not provided:

*   The previousValue argument is the value of the first element present in the array.
*   The currentValue argument is the value of the second element present in the array.

Type: [Function][1]

### Parameters

*   `previousValue` **Reducer** The accumulated value previously returned in the last invocation
    of the callback, or initialValue, if supplied.
*   `currentCoord` **[Array][2]<[number][3]>** The current coordinate being processed.
*   `coordIndex` **[number][3]** The current index of the coordinate being processed.
    Starts at index 0, if an initialValue is provided, and at index 1 otherwise.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.
*   `multiFeatureIndex` **[number][3]** The current index of the Multi-Feature being processed.
*   `geometryIndex` **[number][3]** The current index of the Geometry being processed.

Returns **Reducer**&#x20;

## @turf/meta: coordReduce

Reduce coordinates in any GeoJSON object, similar to Array.reduce()

### Parameters

*   `geojson` **AllGeoJSON** any GeoJSON object
*   `callback` **[coordReduceCallback][6]** a method that takes (previousValue, currentCoord, coordIndex)
*   `initialValue` **Reducer?** Value to use as the first argument to the first call of the callback.
*   `excludeWrapCoord` **[boolean][5]** whether or not to include the final coordinate of LinearRings that wraps the ring in its iteration. (optional, default `false`)

### Examples

```javascript
var features = turf.featureCollection([
  turf.point([26, 37], {"foo": "bar"}),
  turf.point([36, 53], {"hello": "world"})
]);

turf.coordReduce(features, function (previousValue, currentCoord, coordIndex, featureIndex, multiFeatureIndex, geometryIndex) {
  //=previousValue
  //=currentCoord
  //=coordIndex
  //=featureIndex
  //=multiFeatureIndex
  //=geometryIndex
  return currentCoord;
});
```

Returns **Reducer** The value that results from the reduction.

## @turf/meta: propEachCallback

Callback for propEach

Type: [Function][1]

### Parameters

*   `currentProperties` **[GeoJsonProperties][7]** The current Properties being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.

Returns **void**&#x20;

## @turf/meta: propEach

Iterate over properties in any GeoJSON object, similar to Array.forEach()

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7])** any GeoJSON object
*   `callback` **[propEachCallback][9]** a method that takes (currentProperties, featureIndex)

### Examples

```javascript
var features = turf.featureCollection([
    turf.point([26, 37], {foo: 'bar'}),
    turf.point([36, 53], {hello: 'world'})
]);

turf.propEach(features, function (currentProperties, featureIndex) {
  //=currentProperties
  //=featureIndex
});
```

Returns **void**&#x20;

## @turf/meta: propReduceCallback

Callback for propReduce

The first time the callback function is called, the values provided as arguments depend
on whether the reduce method has an initialValue argument.

If an initialValue is provided to the reduce method:

*   The previousValue argument is initialValue.
*   The currentValue argument is the value of the first element present in the array.

If an initialValue is not provided:

*   The previousValue argument is the value of the first element present in the array.
*   The currentValue argument is the value of the second element present in the array.

Type: [Function][1]

### Parameters

*   `previousValue` **Reducer** The accumulated value previously returned in the last invocation
    of the callback, or initialValue, if supplied.
*   `currentProperties` **[GeoJsonProperties][7]** The current Properties being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.

Returns **Reducer**&#x20;

## @turf/meta: propReduce

Reduce properties in any GeoJSON object into a single value,
similar to how Array.reduce works. However, in this case we lazily run
the reduction, so an array of all properties is unnecessary.

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7] | [Geometry][10])** any GeoJSON object
*   `callback` **[propReduceCallback][11]** a method that takes (previousValue, currentProperties, featureIndex)
*   `initialValue` **Reducer?** Value to use as the first argument to the first call of the callback.

### Examples

```javascript
var features = turf.featureCollection([
    turf.point([26, 37], {foo: 'bar'}),
    turf.point([36, 53], {hello: 'world'})
]);

turf.propReduce(features, function (previousValue, currentProperties, featureIndex) {
  //=previousValue
  //=currentProperties
  //=featureIndex
  return currentProperties
});
```

Returns **Reducer** The value that results from the reduction.

## @turf/meta: featureEachCallback

Callback for featureEach

Type: [Function][1]

### Parameters

*   `currentFeature` **[Feature][7]\<any>** The current Feature being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.

Returns **void**&#x20;

## @turf/meta: featureEach

Iterate over features in any GeoJSON object, similar to
Array.forEach.

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7] | [Feature][7]<[GeometryCollection][12]>)** any GeoJSON object
*   `callback` **[featureEachCallback][13]** a method that takes (currentFeature, featureIndex)

### Examples

```javascript
var features = turf.featureCollection([
  turf.point([26, 37], {foo: 'bar'}),
  turf.point([36, 53], {hello: 'world'})
]);

turf.featureEach(features, function (currentFeature, featureIndex) {
  //=currentFeature
  //=featureIndex
});
```

Returns **void**&#x20;

## @turf/meta: featureReduceCallback

Callback for featureReduce

The first time the callback function is called, the values provided as arguments depend
on whether the reduce method has an initialValue argument.

If an initialValue is provided to the reduce method:

*   The previousValue argument is initialValue.
*   The currentValue argument is the value of the first element present in the array.

If an initialValue is not provided:

*   The previousValue argument is the value of the first element present in the array.
*   The currentValue argument is the value of the second element present in the array.

Type: [Function][1]

### Parameters

*   `previousValue` **Reducer** The accumulated value previously returned in the last invocation
    of the callback, or initialValue, if supplied.
*   `currentFeature` **[Feature][7]** The current Feature being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.

Returns **Reducer**&#x20;

## @turf/meta: featureReduce

Reduce features in any GeoJSON object, similar to Array.reduce().

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7] | [Feature][7]<[GeometryCollection][12]>)** any GeoJSON object
*   `callback` **[featureReduceCallback][14]** a method that takes (previousValue, currentFeature, featureIndex)
*   `initialValue` **Reducer?** Value to use as the first argument to the first call of the callback.

### Examples

```javascript
var features = turf.featureCollection([
  turf.point([26, 37], {"foo": "bar"}),
  turf.point([36, 53], {"hello": "world"})
]);

turf.featureReduce(features, function (previousValue, currentFeature, featureIndex) {
  //=previousValue
  //=currentFeature
  //=featureIndex
  return currentFeature
});
```

Returns **Reducer** The value that results from the reduction.

## @turf/meta: coordAll

Get all coordinates from any GeoJSON object.

### Parameters

*   `geojson` **AllGeoJSON** any GeoJSON object

### Examples

```javascript
var features = turf.featureCollection([
  turf.point([26, 37], {foo: 'bar'}),
  turf.point([36, 53], {hello: 'world'})
]);

var coords = turf.coordAll(features);
//= [[26, 37], [36, 53]]
```

Returns **[Array][2]<[Array][2]<[number][3]>>** coordinate position array

## @turf/meta: geomEachCallback

Callback for geomEach

Type: [Function][1]

### Parameters

*   `currentGeometry` **[GeometryObject][10]** The current Geometry being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.
*   `featureProperties` **[GeoJsonProperties][7]** The current Feature Properties being processed.
*   `featureBBox` **[BBox][15]** The current Feature BBox being processed.
*   `featureId` **Id** The current Feature Id being processed.

Returns **void**&#x20;

## @turf/meta: geomEach

Iterate over each geometry in any GeoJSON object, similar to Array.forEach()

Note: Nested GeometryCollections are not recursively unwrapped.

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7] | [Geometry][10] | [GeometryObject][10] | [Feature][7]<[GeometryCollection][12]>)** any GeoJSON object
*   `callback` **[geomEachCallback][16]** a method that takes (currentGeometry, featureIndex, featureProperties, featureBBox, featureId)

### Examples

```javascript
var features = turf.featureCollection([
    turf.point([26, 37], {foo: 'bar'}),
    turf.point([36, 53], {hello: 'world'})
]);

turf.geomEach(features, function (currentGeometry, featureIndex, featureProperties, featureBBox, featureId) {
  //=currentGeometry
  //=featureIndex
  //=featureProperties
  //=featureBBox
  //=featureId
});
```

Returns **void**&#x20;

## @turf/meta: geomReduceCallback

Callback for geomReduce

The first time the callback function is called, the values provided as arguments depend
on whether the reduce method has an initialValue argument.

If an initialValue is provided to the reduce method:

*   The previousValue argument is initialValue.
*   The currentValue argument is the value of the first element present in the array.

If an initialValue is not provided:

*   The previousValue argument is the value of the first element present in the array.
*   The currentValue argument is the value of the second element present in the array.

Type: [Function][1]

### Parameters

*   `previousValue` **Reducer** The accumulated value previously returned in the last invocation
    of the callback, or initialValue, if supplied.
*   `currentGeometry` **[GeometryObject][10]** The current Geometry being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.
*   `featureProperties` **[GeoJsonProperties][7]** The current Feature Properties being processed.
*   `featureBBox` **[BBox][15]** The current Feature BBox being processed.
*   `featureId` **Id** The current Feature Id being processed.

Returns **Reducer**&#x20;

## @turf/meta: geomReduce

Reduce geometry in any GeoJSON object, similar to Array.reduce().

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7] | [GeometryObject][10] | [GeometryCollection][12] | [Feature][7]<[GeometryCollection][12]>)** any GeoJSON object
*   `callback` **[geomReduceCallback][17]** a method that takes (previousValue, currentGeometry, featureIndex, featureProperties, featureBBox, featureId)
*   `initialValue` **Reducer?** Value to use as the first argument to the first call of the callback.

### Examples

```javascript
var features = turf.featureCollection([
    turf.point([26, 37], {foo: 'bar'}),
    turf.point([36, 53], {hello: 'world'})
]);

turf.geomReduce(features, function (previousValue, currentGeometry, featureIndex, featureProperties, featureBBox, featureId) {
  //=previousValue
  //=currentGeometry
  //=featureIndex
  //=featureProperties
  //=featureBBox
  //=featureId
  return currentGeometry
});
```

Returns **Reducer** The value that results from the reduction.

## @turf/meta: flattenEachCallback

Callback for flattenEach

Type: [Function][1]

### Parameters

*   `currentFeature` **[Feature][7]** The current flattened feature being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.
*   `multiFeatureIndex` **[number][3]** The current index of the Multi-Feature being processed.

Returns **void**&#x20;

## @turf/meta: flattenEach

Iterate over flattened features in any GeoJSON object, similar to
Array.forEach.

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7] | [GeometryObject][10] | [GeometryCollection][12] | [Feature][7]<[GeometryCollection][12]>)** any GeoJSON object
*   `callback` **[flattenEachCallback][18]** a method that takes (currentFeature, featureIndex, multiFeatureIndex)

### Examples

```javascript
var features = turf.featureCollection([
    turf.point([26, 37], {foo: 'bar'}),
    turf.multiPoint([[40, 30], [36, 53]], {hello: 'world'})
]);

turf.flattenEach(features, function (currentFeature, featureIndex, multiFeatureIndex) {
  //=currentFeature
  //=featureIndex
  //=multiFeatureIndex
});
```

Returns **void**&#x20;

## @turf/meta: flattenReduceCallback

Callback for flattenReduce

The first time the callback function is called, the values provided as arguments depend
on whether the reduce method has an initialValue argument.

If an initialValue is provided to the reduce method:

*   The previousValue argument is initialValue.
*   The currentValue argument is the value of the first element present in the array.

If an initialValue is not provided:

*   The previousValue argument is the value of the first element present in the array.
*   The currentValue argument is the value of the second element present in the array.

Type: [Function][1]

### Parameters

*   `previousValue` **Reducer** The accumulated value previously returned in the last invocation
    of the callback, or initialValue, if supplied.
*   `currentFeature` **[Feature][7]** The current Feature being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.
*   `multiFeatureIndex` **[number][3]** The current index of the Multi-Feature being processed.

Returns **Reducer**&#x20;

## @turf/meta: flattenReduce

Reduce flattened features in any GeoJSON object, similar to Array.reduce().

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7] | [GeometryObject][10] | [GeometryCollection][12] | [Feature][7]<[GeometryCollection][12]>)** any GeoJSON object
*   `callback` **[flattenReduceCallback][19]** a method that takes (previousValue, currentFeature, featureIndex, multiFeatureIndex)
*   `initialValue` **Reducer?** Value to use as the first argument to the first call of the callback.

### Examples

```javascript
var features = turf.featureCollection([
    turf.point([26, 37], {foo: 'bar'}),
    turf.multiPoint([[40, 30], [36, 53]], {hello: 'world'})
]);

turf.flattenReduce(features, function (previousValue, currentFeature, featureIndex, multiFeatureIndex) {
  //=previousValue
  //=currentFeature
  //=featureIndex
  //=multiFeatureIndex
  return currentFeature
});
```

Returns **Reducer** The value that results from the reduction.

## @turf/meta: segmentEachCallback

Callback for segmentEach

Type: [Function][1]

### Parameters

*   `currentSegment` **[Feature][7]<[LineString][20]>** The current Segment being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.
*   `multiFeatureIndex` **[number][3]** The current index of the Multi-Feature being processed.
*   `geometryIndex` **[number][3]** The current index of the Geometry being processed.
*   `segmentIndex` **[number][3]** The current index of the Segment being processed.

Returns **void**&#x20;

## @turf/meta: segmentEach

Iterate over 2-vertex line segment in any GeoJSON object, similar to Array.forEach()
(Multi)Point geometries do not contain segments therefore they are ignored during this operation.

### Parameters

*   `geojson` **AllGeoJSON** any GeoJSON
*   `callback` **[segmentEachCallback][21]** a method that takes (currentSegment, featureIndex, multiFeatureIndex, geometryIndex, segmentIndex)

### Examples

```javascript
var polygon = turf.polygon([[[-50, 5], [-40, -10], [-50, -10], [-40, 5], [-50, 5]]]);

// Iterate over GeoJSON by 2-vertex segments
turf.segmentEach(polygon, function (currentSegment, featureIndex, multiFeatureIndex, geometryIndex, segmentIndex) {
  //=currentSegment
  //=featureIndex
  //=multiFeatureIndex
  //=geometryIndex
  //=segmentIndex
});

// Calculate the total number of segments
var total = 0;
turf.segmentEach(polygon, function () {
    total++;
});
```

Returns **void**&#x20;

## @turf/meta: segmentReduceCallback

Callback for segmentReduce

The first time the callback function is called, the values provided as arguments depend
on whether the reduce method has an initialValue argument.

If an initialValue is provided to the reduce method:

*   The previousValue argument is initialValue.
*   The currentValue argument is the value of the first element present in the array.

If an initialValue is not provided:

*   The previousValue argument is the value of the first element present in the array.
*   The currentValue argument is the value of the second element present in the array.

Type: [Function][1]

### Parameters

*   `previousValue` **Reducer** The accumulated value previously returned in the last invocation
    of the callback, or initialValue, if supplied.
*   `currentSegment` **[Feature][7]<[LineString][20]>** The current Segment being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed.
*   `multiFeatureIndex` **[number][3]** The current index of the Multi-Feature being processed.
*   `geometryIndex` **[number][3]** The current index of the Geometry being processed.
*   `segmentIndex` **[number][3]** The current index of the Segment being processed.

Returns **Reducer**&#x20;

## @turf/meta: segmentReduce

Reduce 2-vertex line segment in any GeoJSON object, similar to Array.reduce()
(Multi)Point geometries do not contain segments therefore they are ignored during this operation.

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7] | [Geometry][10])** any GeoJSON
*   `callback` **[segmentReduceCallback][22]** a method that takes (previousValue, currentSegment, currentIndex)
*   `initialValue` **Reducer?** Value to use as the first argument to the first call of the callback.

### Examples

```javascript
var polygon = turf.polygon([[[-50, 5], [-40, -10], [-50, -10], [-40, 5], [-50, 5]]]);

// Iterate over GeoJSON by 2-vertex segments
turf.segmentReduce(polygon, function (previousSegment, currentSegment, featureIndex, multiFeatureIndex, geometryIndex, segmentIndex) {
  //= previousSegment
  //= currentSegment
  //= featureIndex
  //= multiFeatureIndex
  //= geometryIndex
  //= segmentIndex
  return currentSegment
});

// Calculate the total number of segments
var initialValue = 0
var total = turf.segmentReduce(polygon, function (previousValue) {
    previousValue++;
    return previousValue;
}, initialValue);
```

Returns **Reducer**&#x20;

## @turf/meta: lineEachCallback

Callback for lineEach

Type: [Function][1]

### Parameters

*   `currentLine` **[Feature][7]<[LineString][20]>** The current LineString|LinearRing being processed
*   `featureIndex` **[number][3]** The current index of the Feature being processed
*   `multiFeatureIndex` **[number][3]** The current index of the Multi-Feature being processed
*   `geometryIndex` **[number][3]** The current index of the Geometry being processed

Returns **void**&#x20;

## @turf/meta: lineEach

Iterate over line or ring coordinates in LineString, Polygon, MultiLineString, MultiPolygon Features or Geometries,
similar to Array.forEach.

### Parameters

*   `geojson` **([FeatureCollection][8]\<Lines> | [Feature][7]\<Lines> | Lines | [Feature][7]<[GeometryCollection][12]> | [GeometryCollection][12])** object
*   `callback` **[lineEachCallback][23]** a method that takes (currentLine, featureIndex, multiFeatureIndex, geometryIndex)

### Examples

```javascript
var multiLine = turf.multiLineString([
  [[26, 37], [35, 45]],
  [[36, 53], [38, 50], [41, 55]]
]);

turf.lineEach(multiLine, function (currentLine, featureIndex, multiFeatureIndex, geometryIndex) {
  //=currentLine
  //=featureIndex
  //=multiFeatureIndex
  //=geometryIndex
});
```

Returns **void**&#x20;

## @turf/meta: lineReduceCallback

Callback for lineReduce

The first time the callback function is called, the values provided as arguments depend
on whether the reduce method has an initialValue argument.

If an initialValue is provided to the reduce method:

*   The previousValue argument is initialValue.
*   The currentValue argument is the value of the first element present in the array.

If an initialValue is not provided:

*   The previousValue argument is the value of the first element present in the array.
*   The currentValue argument is the value of the second element present in the array.

Type: [Function][1]

### Parameters

*   `previousValue` **Reducer** The accumulated value previously returned in the last invocation
    of the callback, or initialValue, if supplied.
*   `currentLine` **[Feature][7]<[LineString][20]>** The current LineString|LinearRing being processed.
*   `featureIndex` **[number][3]** The current index of the Feature being processed
*   `multiFeatureIndex` **[number][3]** The current index of the Multi-Feature being processed
*   `geometryIndex` **[number][3]** The current index of the Geometry being processed

Returns **Reducer**&#x20;

## @turf/meta: lineReduce

Reduce features in any GeoJSON object, similar to Array.reduce().

### Parameters

*   `geojson` **([FeatureCollection][8]\<Lines> | [Feature][7]\<Lines> | Lines | [Feature][7]<[GeometryCollection][12]> | [GeometryCollection][12])** object
*   `callback` **[Function][1]** a method that takes (previousValue, currentLine, featureIndex, multiFeatureIndex, geometryIndex)
*   `initialValue` **Reducer?** Value to use as the first argument to the first call of the callback.

### Examples

```javascript
var multiPoly = turf.multiPolygon([
  turf.polygon([[[12,48],[2,41],[24,38],[12,48]], [[9,44],[13,41],[13,45],[9,44]]]),
  turf.polygon([[[5, 5], [0, 0], [2, 2], [4, 4], [5, 5]]])
]);

turf.lineReduce(multiPoly, function (previousValue, currentLine, featureIndex, multiFeatureIndex, geometryIndex) {
  //=previousValue
  //=currentLine
  //=featureIndex
  //=multiFeatureIndex
  //=geometryIndex
  return currentLine
});
```

Returns **Reducer** The value that results from the reduction.

## @turf/meta: findSegment

Finds a particular 2-vertex LineString Segment from a GeoJSON using `@turf/meta` indexes.

Negative indexes are permitted.
Point & MultiPoint will always return null.

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7] | [Geometry][10])** Any GeoJSON Feature or Geometry
*   `options` **[Object][24]** Optional parameters (optional, default `{}`)

    *   `options.featureIndex` **[number][3]** Feature Index (optional, default `0`)
    *   `options.multiFeatureIndex` **[number][3]** Multi-Feature Index (optional, default `0`)
    *   `options.geometryIndex` **[number][3]** Geometry Index (optional, default `0`)
    *   `options.segmentIndex` **[number][3]** Segment Index (optional, default `0`)
    *   `options.properties` **[Object][24]** Translate Properties to output LineString (optional, default `{}`)
    *   `options.bbox` **[BBox][15]** Translate BBox to output LineString (optional, default `{}`)
    *   `options.id` **([number][3] | [string][25])** Translate Id to output LineString (optional, default `{}`)

### Examples

```javascript
var multiLine = turf.multiLineString([
    [[10, 10], [50, 30], [30, 40]],
    [[-10, -10], [-50, -30], [-30, -40]]
]);

// First Segment (defaults are 0)
turf.findSegment(multiLine);
// => Feature<LineString<[[10, 10], [50, 30]]>>

// First Segment of 2nd Multi Feature
turf.findSegment(multiLine, {multiFeatureIndex: 1});
// => Feature<LineString<[[-10, -10], [-50, -30]]>>

// Last Segment of Last Multi Feature
turf.findSegment(multiLine, {multiFeatureIndex: -1, segmentIndex: -1});
// => Feature<LineString<[[-50, -30], [-30, -40]]>>
```

Returns **[Feature][7]<[LineString][20]>** 2-vertex GeoJSON Feature LineString

## @turf/meta: findPoint

Finds a particular Point from a GeoJSON using `@turf/meta` indexes.

Negative indexes are permitted.

### Parameters

*   `geojson` **([FeatureCollection][8] | [Feature][7] | [Geometry][10])** Any GeoJSON Feature or Geometry
*   `options` **[Object][24]** Optional parameters (optional, default `{}`)

    *   `options.featureIndex` **[number][3]** Feature Index (optional, default `0`)
    *   `options.multiFeatureIndex` **[number][3]** Multi-Feature Index (optional, default `0`)
    *   `options.geometryIndex` **[number][3]** Geometry Index (optional, default `0`)
    *   `options.coordIndex` **[number][3]** Coord Index (optional, default `0`)
    *   `options.properties` **[Object][24]** Translate Properties to output Point (optional, default `{}`)
    *   `options.bbox` **[BBox][15]** Translate BBox to output Point (optional, default `{}`)
    *   `options.id` **([number][3] | [string][25])** Translate Id to output Point (optional, default `{}`)

### Examples

```javascript
var multiLine = turf.multiLineString([
    [[10, 10], [50, 30], [30, 40]],
    [[-10, -10], [-50, -30], [-30, -40]]
]);

// First Segment (defaults are 0)
turf.findPoint(multiLine);
// => Feature<Point<[10, 10]>>

// First Segment of the 2nd Multi-Feature
turf.findPoint(multiLine, {multiFeatureIndex: 1});
// => Feature<Point<[-10, -10]>>

// Last Segment of last Multi-Feature
turf.findPoint(multiLine, {multiFeatureIndex: -1, coordIndex: -1});
// => Feature<Point<[-30, -40]>>
```

Returns **[Feature][7]<[Point][26]>** 2-vertex GeoJSON Feature Point

[1]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Statements/function

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: #coordeachcallback

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[6]: #coordreducecallback

[7]: https://tools.ietf.org/html/rfc7946#section-3.2

[8]: https://tools.ietf.org/html/rfc7946#section-3.3

[9]: #propeachcallback

[10]: https://tools.ietf.org/html/rfc7946#section-3.1

[11]: #propreducecallback

[12]: https://tools.ietf.org/html/rfc7946#section-3.1.8

[13]: #featureeachcallback

[14]: #featurereducecallback

[15]: https://tools.ietf.org/html/rfc7946#section-5

[16]: #geomeachcallback

[17]: #geomreducecallback

[18]: #flatteneachcallback

[19]: #flattenreducecallback

[20]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[21]: #segmenteachcallback

[22]: #segmentreducecallback

[23]: #lineeachcallback

[24]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[25]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[26]: https://tools.ietf.org/html/rfc7946#section-3.1.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/meta
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/midpoint

## @turf/midpoint: midpoint

Takes two points and returns a point midway between them. The midpoint is
calculated geodesically, meaning the curvature of the earth is taken into
account.

### Parameters

*   `point1` **[Coord][1]** first point
*   `point2` **[Coord][1]** second point

### Examples

```javascript
const point1 = turf.point([144.834823, -37.771257]);
const point2 = turf.point([145.14244, -37.830937]);

const midpoint = turf.midpoint(point1, point2);

//addToMap
const addToMap = [point1, point2, midpoint];
midpoint.properties['marker-color'] = '#f00';
```

Returns **[Feature][2]<[Point][3]>** a point midway between `pt1` and `pt2`

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/midpoint
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/moran-index

## @turf/moran-index: MoranIndex

Type: [object][1]

### Properties

*   `moranIndex` **[number][2]** the moran's Index of the observed feature set
*   `expectedMoranIndex` **[number][2]** the moran's Index of the random distribution
*   `stdNorm` **[number][2]** the standard devitaion of the random distribution
*   `zNorm` **[number][2]** the z-score of the observe samples with regard to the random distribution

## @turf/moran-index: moranIndex

Moran's I measures patterns of attribute values associated with features.
The method reveal whether similar values tend to occur near each other,
or whether high or low values are interspersed.

Moran's I > 0 means a clusterd pattern.
Moran's I < 0 means a dispersed pattern.
Moran's I = 0 means a random pattern.

In order to test the significance of the result. The z score is calculated.
A positive enough z-score (ex. >1.96) indicates clustering,
while a negative enough z-score (ex. <-1.96) indicates a dispersed pattern.

the z-score can be calculated based on a normal or random assumption.

**Bibliography**\*

1.  [Moran's I][3]

2.  [pysal][4]

3.  Andy Mitchell, The ESRI Guide to GIS Analysis Volume 2: Spatial Measurements & Statistics.

### Parameters

*   `fc` **[FeatureCollection][5]\<any>**&#x20;
*   `options` **[Object][1]**&#x20;

    *   `options.inputField` **[string][6]** the property name, must contain numeric values
    *   `options.threshold` **[number][2]** the distance threshold (optional, default `100000`)
    *   `options.p` **[number][2]** the Minkowski p-norm distance parameter (optional, default `2`)
    *   `options.binary` **[boolean][7]** whether transfrom the distance to binary (optional, default `false`)
    *   `options.alpha` **[number][2]** the distance decay parameter (optional, default `-1`)
    *   `options.standardization` **[boolean][7]** wheter row standardization the distance (optional, default `true`)

### Examples

```javascript
const bbox = [-65, 40, -63, 42];
const dataset = turf.randomPoint(100, { bbox: bbox });

const result = turf.moranIndex(dataset, {
  inputField: 'CRIME',
});
```

Returns **[MoranIndex][8]**&#x20;

[1]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[3]: https://en.wikipedia.org/wiki/Moran%27s_I

[4]: http://pysal.readthedocs.io/en/latest/index.html

[5]: https://tools.ietf.org/html/rfc7946#section-3.3

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[8]: #moranindex



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/moran-index
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/nearest-neighbor-analysis

## @turf/nearest-neighbor-analysis: NearestNeighborStatistics

Nearest neighbour statistics.

Type: [object][1]

### Properties

*   `arealUnits` **[string][2]**&#x20;
*   `observedMeanDistance` **[number][3]**&#x20;
*   `expectedMeanDistance` **[number][3]**&#x20;
*   `numberOfPoints` **[number][3]**&#x20;
*   `zScore` **[number][3]**&#x20;

## @turf/nearest-neighbor-analysis: NearestNeighborStudyArea

Nearest neighbour study area polygon feature.

Type: [object][1]

### Properties

*   `properties` **[GeoJsonProperties][4]**&#x20;

    *   `properties.nearestNeighborAnalysis` **[NearestNeighborStatistics][5]**&#x20;

## @turf/nearest-neighbor-analysis: nearestNeighborAnalysis

Nearest Neighbor Analysis calculates an index based on the average distances
between points in the dataset, thereby providing inference as to whether the
data is clustered, dispersed, or randomly distributed within the study area.

It returns a [Feature][4]<[Polygon][6]> of the study area, with the results of
the analysis attached as part of of the `nearestNeighborAnalysis` property
of the study area's `properties`. The attached
[*z*-score][7] indicates how many
standard deviations above or below the expected mean distance the data's
observed mean distance is. The more negative, the more clustered. The more
positive, the more evenly dispersed. A *z*-score between -2 and 2 indicates
a seemingly random distribution. That is, within *p* of less than 0.05, the
distribution appears statistically significantly neither clustered nor
dispersed.

**Remarks**

*   Though the analysis will work on any [FeatureCollection][8] type, it
    works best with [Point][9] collections.

*   This analysis is *very* sensitive to the study area provided.
    If no [Feature][4]<[Polygon][6]> is passed as the study area, the function draws a box
    around the data, which may distort the findings. This analysis works best
    with a bounded area of interest within with the data is either clustered,
    dispersed, or randomly distributed. For example, a city's subway stops may
    look extremely clustered if the study area is an entire state. On the other
    hand, they may look rather evenly dispersed if the study area is limited to
    the city's downtown.

**Bibliography**

Philip J. Clark and Francis C. Evans, “Distance to Nearest Neighbor as a
Measure of Spatial Relationships in Populations,” *Ecology* 35, no. 4
(1954): 445–453, doi:[10.2307/1931034][10].

### Parameters

*   `dataset` **[FeatureCollection][8]\<any>** FeatureCollection (pref. of points) to study
*   `options` **[Object][1]** Optional parameters (optional, default `{}`)

    *   `options.studyArea` **[Feature][4]<[Polygon][6]>?** polygon representing the study area
    *   `options.properties` **[GeoJsonProperties][4]** properties (optional, default `{}`)

### Examples

```javascript
var bbox = [-65, 40, -63, 42];
var dataset = turf.randomPoint(100, { bbox: bbox });
var nearestNeighborStudyArea = turf.nearestNeighborAnalysis(dataset);

//addToMap
var addToMap = [dataset, nearestNeighborStudyArea];
```

Returns **[NearestNeighborStudyArea][11]** A polygon of the study area or an approximation of one.

[1]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: https://tools.ietf.org/html/rfc7946#section-3.2

[5]: #nearestneighborstatistics

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[7]: https://en.wikipedia.org/wiki/Standard_score

[8]: https://tools.ietf.org/html/rfc7946#section-3.3

[9]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[10]: http://doi.org/10.2307/1931034

[11]: #nearestneighborstudyarea



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/nearest-neighbor-analysis
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/nearest-point-on-line

## @turf/nearest-point-on-line: nearestPointOnLine

Returns the nearest point on a line to a given point.

If any of the segments in the input line string are antipodal and therefore
have an undefined arc, this function will instead return that the point lies
on the line.

⚠️ We have begun the process of migrating to different return properties for
this function. The new properties we recommend using as of v7.4 are:

*   lineStringIndex - point was found on the nth LineString of an input MultiLineString. Previously `multiFeatureIndex`
*   segmentIndex - point was found on the nth segment of the above LineString. Previously `index`
*   totalDistance - distance from the start of the overall MultiLineString. Previously `location`
*   lineDistance - distance from the start of the relevant LineString
*   segmentDistance - distance from the start of the relevant segment
*   pointDistance - distance between found point is from input reference point. Previously `dist`

multiFeatureIndex, index, location, and dist continue to work as previously
until at least the next major release.

### Parameters

*   `lines` **([Geometry][1] | [Feature][2]<([LineString][3] | [MultiLineString][4])>)** Lines to snap to
*   `inputPoint` **([Geometry][1] | [Feature][2]<[Point][5]> | [Array][6]<[number][7]>)** Point to snap from
*   `options` **[Object][8]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][9] (optional, default `'kilometers'`)

### Examples

```javascript
var line = turf.lineString([
    [-77.031669, 38.878605],
    [-77.029609, 38.881946],
    [-77.020339, 38.884084],
    [-77.025661, 38.885821],
    [-77.021884, 38.889563],
    [-77.019824, 38.892368]
]);
var inputPoint = turf.point([-77.037076, 38.884017]);

var snapped = turf.nearestPointOnLine(line, inputPoint, {units: 'miles'});

//addToMap
var addToMap = [line, inputPoint, snapped];
snapped.properties['marker-color'] = '#00f';
```

Returns **[Feature][2]<[Point][5]>** closest point on the `lines` to the `inputPoint`. The point will have the following properties: `lineStringIndex`: closest point was found on the nth LineString (only relevant if input is MultiLineString), `segmentIndex`: closest point was found on nth line segment of the LineString, `totalDistance`: distance along the line from the absolute start of the MultiLineString, `lineDistance`: distance along the line from the start of the LineString where the closest point was found, `segmentDistance`: distance along the line from the start of the line segment where the closest point was found, `pointDistance`: distance to the input point.

[1]: https://tools.ietf.org/html/rfc7946#section-3.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[8]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[9]: https://turfjs.org/docs/api/types/Units



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/nearest-point-on-line
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/nearest-point-to-line

## @turf/nearest-point-to-line: nearestPointToLine

Returns the closest [point][1], of a [collection][2] of points,
to a [line][3]. The returned point has a `dist` property indicating its distance to the line.

### Parameters

*   `points` **([FeatureCollection][2] | [GeometryCollection][4]<[Point][1]>)** Point Collection
*   `line` **([Feature][5] | [Geometry][6]<[LineString][3]>)** Line Feature
*   `options` **[Object][7]?** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** unit of the output distance property. Supports all valid Turf [Units][8].
        (eg: degrees, radians, miles, or kilometers) (optional, default `'kilometers'`)
    *   `options.properties` **[Object][7]** Translate Properties to Point (optional, default `{}`)

### Examples

```javascript
var pt1 = turf.point([0, 0]);
var pt2 = turf.point([0.5, 0.5]);
var points = turf.featureCollection([pt1, pt2]);
var line = turf.lineString([[1,1], [-1,1]]);

var nearest = turf.nearestPointToLine(points, line);

//addToMap
var addToMap = [nearest, line];
```

Returns **[Feature][5]<[Point][1]>** the closest point

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.3

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.8

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://tools.ietf.org/html/rfc7946#section-3.1

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[8]: https://turfjs.org/docs/api/types/Units



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/nearest-point-to-line
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/nearest-point

## @turf/nearest-point: nearestPoint

Takes a reference [point][1] and a FeatureCollection of Features
with Point geometries and returns the
point from the FeatureCollection closest to the reference. This calculation
is geodesic.

### Parameters

*   `targetPoint` **[Coord][2]** the reference point
*   `points` **[FeatureCollection][3]<[Point][1]>** against input point set
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** the units of the numeric result. Supports all valid Turf [Units][5]. (optional, default `'kilometers'`)

### Examples

```javascript
var targetPoint = turf.point([28.965797, 41.010086], {"marker-color": "#0F0"});
var points = turf.featureCollection([
    turf.point([28.973865, 41.011122]),
    turf.point([28.948459, 41.024204]),
    turf.point([28.938674, 41.013324])
]);

var nearest = turf.nearestPoint(targetPoint, points);

//addToMap
var addToMap = [targetPoint, points, nearest];
nearest.properties['marker-color'] = '#F00';
```

Returns **[Feature][6]<[Point][1]>** the closest point in the set to the reference point

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[3]: https://tools.ietf.org/html/rfc7946#section-3.3

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://turfjs.org/docs/api/types/Units

[6]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/nearest-point
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/planepoint

## @turf/planepoint: planepoint

Takes a triangular plane as a polygon and a point within that triangle, and
returns the z-value at that point.

The Polygon should have properties `a`, `b`, and `c`
that define the values at its three corners. Alternatively, the z-values
of each triangle point can be provided by their respective 3rd coordinate
if their values are not provided as properties.

### Parameters

*   `point` **[Coord][1]** the Point for which a z-value will be calculated
*   `triangle` **[Feature][2]<[Polygon][3]>** a Polygon feature with three vertices

### Examples

```javascript
const point = turf.point([-75.3221, 39.529]);
// "a", "b", and "c" values represent the values of the coordinates in order.
const triangle = turf.polygon([[
  [-75.1221, 39.57],
  [-75.58, 39.18],
  [-75.97, 39.86],
  [-75.1221, 39.57]
]], {
  "a": 11,
  "b": 122,
  "c": 44
});

const zValue = turf.planepoint(point, triangle);
point.properties.zValue = zValue;

//addToMap
const addToMap = [triangle, point];
```

Returns **[number][4]** the z-value for `interpolatedPoint`

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/planepoint
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/point-grid

## @turf/point-grid: pointGrid

Creates a grid of points

### Parameters

*   `bbox` **[BBox][1]** extent of grid in \[minX, minY, maxX, maxY] order
*   `cellSide` **[number][2]** the distance between points
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** the units of the cellSide value.  Supports all valid Turf [Units][4] (optional, default `'kilometers'`)
    *   `options.mask` **[Feature][5]<([Polygon][6] | [MultiPolygon][7])>?** if passed a Polygon or MultiPolygon, the grid Points will be created only inside it
    *   `options.properties` **[Object][3]** passed to each point of the grid (optional, default `{}`)

### Examples

```javascript
var extent = [-70.823364, -33.553984, -70.473175, -33.302986];
var cellSide = 3;
var options = {units: 'miles'};

var grid = turf.pointGrid(extent, cellSide, options);

//addToMap
var addToMap = [grid];
```

Returns **[FeatureCollection][8]<[Point][9]>** grid of points

[1]: https://tools.ietf.org/html/rfc7946#section-5

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://github.com/Turfjs/turf/blob/master/packages/turf-helpers/README_UNITS.md

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[8]: https://tools.ietf.org/html/rfc7946#section-3.3

[9]: https://tools.ietf.org/html/rfc7946#section-3.1.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/point-grid
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/point-on-feature

## @turf/point-on-feature: pointOnFeature

Takes a Feature or FeatureCollection and returns a [Point][1] guaranteed to be on the surface of the feature.

*   Given a [Polygon][2], the point will be in the area of the polygon
*   Given a [LineString][3], the point will be along the string
*   Given a [Point][1], the point will the same as the input

### Parameters

*   `geojson` **[GeoJSON][4]** any Feature or FeatureCollection

### Examples

```javascript
var polygon = turf.polygon([[
  [116, -36],
  [131, -32],
  [146, -43],
  [155, -25],
  [133, -9],
  [111, -22],
  [116, -36]
]]);

var pointOnPolygon = turf.pointOnFeature(polygon);

//addToMap
var addToMap = [polygon, pointOnPolygon];
```

Returns **[Feature][5]<[Point][1]>** a point on the surface of `input`

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[4]: https://tools.ietf.org/html/rfc7946#section-3

[5]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/point-on-feature
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/point-to-line-distance

## @turf/point-to-line-distance: pointToLineDistance

Calculates the distance between a given point and the nearest point on a
line. Sometimes referred to as the cross track distance.

### Parameters

*   `pt` **([Feature][1]<[Point][2]> | [Array][3]<[number][4]>)** Feature or Geometry
*   `line` **[Feature][1]<[LineString][5]>** GeoJSON Feature or Geometry
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][7] (optional, default `"kilometers"`)
    *   `options.method` **[string][8]** whether to calculate the distance based on geodesic (spheroid) or
        planar (flat) method. Valid options are 'geodesic' or 'planar'. (optional, default `"geodesic"`)

### Examples

```javascript
var pt = turf.point([0, 0]);
var line = turf.lineString([[1, 1],[-1, 1]]);

var distance = turf.pointToLineDistance(pt, line, {units: 'miles'});
//=69.11854715938406
```

Returns **[number][4]** distance between point and line

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://turfjs.org/docs/api/types/Units

[8]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/point-to-line-distance
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/point-to-polygon-distance

## @turf/point-to-polygon-distance: pointToPolygonDistance

Calculates the distance from a point to the edges of a polygon or multi-polygon.
Returns negative values for points inside the polygon.
Handles polygons with holes and multi-polygons.
A hole is treated as the exterior of the polygon.

### Parameters

*   `point` **([Feature][1]<[Point][2]> | [Point][2] | [Position][3])** Input point
*   `polygonOrMultiPolygon` **([Feature][1]<([Polygon][4] | [MultiPolygon][5])> | [Polygon][4] | [MultiPolygon][5])** Input polygon or multipolygon
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Units of the result e.g. "kilometers", "miles", "meters"
    *   `options.method` **(`"geodesic"` | `"planar"`)** Method of the result



*   Throws **[Error][7]** If input geometries are invalid

Returns **[number][8]** Distance in meters (negative values for points inside the polygon)

[1]: https://tools.ietf.org/html/rfc7946#section-3.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://developer.mozilla.org/docs/Web/API/Position

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Error

[8]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/point-to-polygon-distance
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/points-within-polygon

## @turf/points-within-polygon: pointsWithinPolygon

Finds [Points][1] or [MultiPoint][2] coordinate positions that fall within [(Multi)Polygon(s)][3].

### Parameters

*   `points` **([Feature][4] | [FeatureCollection][5]<([Point][1] | [MultiPoint][2])>)** Point(s) or MultiPoint(s) as input search
*   `polygons` **([FeatureCollection][5] | [Geometry][6] | [Feature][4]<([Polygon][3] | [MultiPolygon][7])>)** (Multi)Polygon(s) to check if points are within

### Examples

```javascript
var points = turf.points([
    [-46.6318, -23.5523],
    [-46.6246, -23.5325],
    [-46.6062, -23.5513],
    [-46.663, -23.554],
    [-46.643, -23.557]
]);

var searchWithin = turf.polygon([[
    [-46.653,-23.543],
    [-46.634,-23.5346],
    [-46.613,-23.543],
    [-46.614,-23.559],
    [-46.631,-23.567],
    [-46.653,-23.560],
    [-46.653,-23.543]
]]);

var ptsWithin = turf.pointsWithinPolygon(points, searchWithin);

//addToMap
var addToMap = [points, searchWithin, ptsWithin]
turf.featureEach(ptsWithin, function (currentFeature) {
  currentFeature.properties['marker-size'] = 'large';
  currentFeature.properties['marker-color'] = '#000';
});
```

Returns **[FeatureCollection][5]<([Point][1] | [MultiPoint][2])>** Point(s) or MultiPoint(s) with positions that land within at least one polygon.  The geometry type will match what was passsed in

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.3

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[4]: https://tools.ietf.org/html/rfc7946#section-3.2

[5]: https://tools.ietf.org/html/rfc7946#section-3.3

[6]: https://tools.ietf.org/html/rfc7946#section-3.1

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.7



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/points-within-polygon
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/polygon-smooth

## @turf/polygon-smooth: polygonSmooth

Smooths a [Polygon][1] or [MultiPolygon][2]. Based on [Chaikin's algorithm][3].
Warning: may create degenerate polygons.

### Parameters

*   `inputPolys` **([FeatureCollection][4]<([Polygon][1] | [MultiPolygon][2])> | [Feature][5]<([Polygon][1] | [MultiPolygon][2])> | [Polygon][1] | [MultiPolygon][2])** (Multi)Polygon(s) to smooth
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.iterations` **[string][7]** The number of times to smooth the polygon. A higher value means a smoother polygon. (optional, default `1`)

### Examples

```javascript
var polygon = turf.polygon([[[11, 0], [22, 4], [31, 0], [31, 11], [21, 15], [11, 11], [11, 0]]]);

var smoothed = turf.polygonSmooth(polygon, {iterations: 3})

//addToMap
var addToMap = [smoothed, polygon];
```

Returns **[FeatureCollection][4]<([Polygon][1] | [MultiPolygon][2])>** FeatureCollection containing the smoothed polygon/multipoylgons

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[3]: https://www.cs.unc.edu/~dm/UNC/COMP258/LECTURES/Chaikins-Algorithm.pdf

[4]: https://tools.ietf.org/html/rfc7946#section-3.3

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/polygon-smooth
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/polygon-tangents

## @turf/polygon-tangents: polygonTangents

Finds the tangents of a [(Multi)Polygon][1] from a [Point][2].

### Parameters

*   `pt` **[Coord][3]** to calculate the tangent points from
*   `polygon` **[Feature][4]<([Polygon][1] | [MultiPolygon][5])>** to get tangents from

### Examples

```javascript
var polygon = turf.polygon([[[11, 0], [22, 4], [31, 0], [31, 11], [21, 15], [11, 11], [11, 0]]]);
var point = turf.point([61, 5]);

var tangents = turf.polygonTangents(point, polygon)

//addToMap
var addToMap = [tangents, point, polygon];
```

Returns **[FeatureCollection][6]<[Point][2]>** Feature Collection containing the two tangent points

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[4]: https://tools.ietf.org/html/rfc7946#section-3.2

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[6]: https://tools.ietf.org/html/rfc7946#section-3.3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/polygon-tangents
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/polygon-to-line

## @turf/polygon-to-line: polygonToLine

Converts a [Polygon][1] to [(Multi)LineString][2] or [MultiPolygon][3] to a
[FeatureCollection][4] of [(Multi)LineString][2].

### Parameters

*   `poly` **[Feature][5]<([Polygon][1] | [MultiPolygon][3])>** Feature to convert
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.properties` **[Object][6]** translates GeoJSON properties to Feature (optional, default `{}`)

### Examples

```javascript
var poly = turf.polygon([[[125, -30], [145, -30], [145, -20], [125, -20], [125, -30]]]);

var line = turf.polygonToLine(poly);

//addToMap
var addToMap = [line];
```

Returns **([FeatureCollection][4] | [Feature][5]<([LineString][2] | MultiLinestring)>)** converted (Multi)Polygon to (Multi)LineString

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[4]: https://tools.ietf.org/html/rfc7946#section-3.3

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/polygon-to-line
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/polygonize

## @turf/polygonize: polygonize

Polygonizes [(Multi)LineString(s)][1] into [Polygons][2].

Implementation of GEOSPolygonize function (`geos::operation::polygonize::Polygonizer`).

Polygonizes a set of lines that represents edges in a planar graph. Edges must be correctly
noded, i.e., they must only meet at their endpoints.

The implementation correctly handles:

*   Dangles: edges which have one or both ends which are not incident on another edge endpoint.
*   Cut Edges (bridges): edges that are connected at both ends but which do not form part of a polygon.

### Parameters

*   `geoJson` **([FeatureCollection][3] | [Geometry][4] | [Feature][5]<([LineString][1] | [MultiLineString][6])>)** Lines in order to polygonize



*   Throws **[Error][7]** if geoJson is invalid.

Returns **[FeatureCollection][3]<[Polygon][2]>** Polygons created

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[3]: https://tools.ietf.org/html/rfc7946#section-3.3

[4]: https://tools.ietf.org/html/rfc7946#section-3.1

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.5

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Error



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/polygonize
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/projection

## @turf/projection: toMercator

Converts a WGS84 GeoJSON object into Mercator (EPSG:3857) projection

### Parameters

*   `geojson` **([GeoJSON][1] | [Position][2])** WGS84 GeoJSON object
*   `options` **[Object][3]?** Optional parameters (optional, default `{}`)

    *   `options.mutate` **[boolean][4]** allows GeoJSON input to be mutated (significant performance increase if true) (optional, default `false`)

### Examples

```javascript
var pt = turf.point([-71,41]);
var converted = turf.toMercator(pt);

//addToMap
var addToMap = [pt, converted];
```

Returns **[GeoJSON][1]** Projected GeoJSON

## @turf/projection: toWgs84

Converts a Mercator (EPSG:3857) GeoJSON object into WGS84 projection

### Parameters

*   `geojson` **([GeoJSON][1] | [Position][2])** Mercator GeoJSON object
*   `options` **[Object][3]?** Optional parameters (optional, default `{}`)

    *   `options.mutate` **[boolean][4]** allows GeoJSON input to be mutated (significant performance increase if true) (optional, default `false`)

### Examples

```javascript
var pt = turf.point([-7903683.846322424, 5012341.663847514]);
var converted = turf.toWgs84(pt);

//addToMap
var addToMap = [pt, converted];
```

Returns **[GeoJSON][1]** Projected GeoJSON

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://developer.mozilla.org/docs/Web/API/Position

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/projection
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/quadrat-analysis

## @turf/quadrat-analysis: quadratAnalysis

Quadrat analysis lays a set of equal-size areas(quadrat) over the study area and counts
the number of features in each quadrat and creates a frequency table.
The table lists the number of quadrats containing no features,
the number containing one feature, two features, and so on,
all the way up to the quadrat containing the most features.
The method then creates the frequency table for the random distribution, usually based on a Poisson distribution.
The method uses the distribution to calculate the probability for 0 feature occuring,
1 feature occuring, 2 features, and so on,
and lists these probabilities in the frequency table.
By comparing the two frequency tables, you can see whether the features create a pattern.
If the table for the observed distribution has more quadrats containing many features than the
table for the random distribution dose, then the features create a clustered pattern.

It is hard to judge the frequency tables are similar or different just by looking at them.
So, we can use serval statistical tests to find out how much the frequency tables differ.
We use Kolmogorov-Smirnov test.This method calculates cumulative probabilities for both distributions,
and then compares the cumulative probabilities at each class level and selects the largest absolute difference D.
Then, the test compares D to the critical value for a confidence level you specify.
If D is greater than the critical value, the difference between  the observed distribution and
the random distribution is significant. The greater the value the bigger the difference.

Traditionally, squares are used for the shape of the quadrats, in a regular grid(square-grid).
Some researchers suggest that the quadrat size equal twice the size of mean area per feature,
which is simply the area of the study area divided by the number of features.

### Parameters

*   `pointFeatureSet` **[FeatureCollection][1]<[Point][2]>** point set to study
*   `options` **[Object][3]** optional parameters (optional, default `{}`)

    *   `options.studyBbox` **\[[number][4], [number][4], [number][4], [number][4]]?** bbox representing the study area
    *   `options.confidenceLevel` **(`20` | `15` | `10` | `5` | `2` | `1`)** a confidence level.
        The unit is percentage . 5 means 95%, value must be in [K\_TABLE][5] (optional, default `20`)

### Examples

```javascript
var bbox = [-65, 40, -63, 42];
var dataset = turf.randomPoint(100, { bbox: bbox });
var result = turf.quadratAnalysis(dataset);
```

Returns **[QuadratAnalysisResult][6]** result

## @turf/quadrat-analysis: K\_TABLE

the confidence level

Type: [Object][3]

### Properties

*   `20` **[number][4]** 1.07275
*   `15` **[number][4]** 1.13795
*   `10` **[number][4]** 1.22385
*   `5` **[number][4]** 1.3581
*   `2` **[number][4]** 1.51743
*   `1` **[number][4]** 1.62762

## @turf/quadrat-analysis: QuadratAnalysisResult

the return type of the quadratAnalysis

Type: [object][3]

### Properties

*   `criticalValue` **[number][4]**&#x20;
*   `maxAbsoluteDifference` **[number][4]**&#x20;
*   `isRandom` **[boolean][7]**&#x20;
*   `observedDistribution` **[Array][8]<[number][4]>** the cumulative distribution of observed features,
    the index represents the number of features in the quadrat.

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[5]: #k_table

[6]: #quadratanalysisresult

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[8]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Array



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/quadrat-analysis
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/random

## @turf/random: randomPosition

Returns a random position within a [bounding box][1].

### Parameters

*   `bbox` **[BBox][1]** a bounding box inside of which positions are placed. (optional, default `[-180,-90,180,90]`)

### Examples

```javascript
var position = turf.randomPosition([-180, -90, 180, 90])
// => position
```

*   Throws **[Error][2]** if bbox is invalid

Returns **[Position][3]** Position \[longitude, latitude]

## @turf/random: randomPoint

Returns a random [point][4].

### Parameters

*   `count` **[number][5]** how many geometries will be generated (optional, default `1`)
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][1]** a bounding box inside of which geometries are placed. (optional, default `[-180,-90,180,90]`)

### Examples

```javascript
var points = turf.randomPoint(25, {bbox: [-180, -90, 180, 90]})
// => points
```

*   Throws **[Error][2]** if bbox is invalid

Returns **[FeatureCollection][7]<[Point][8]>** GeoJSON FeatureCollection of points

## @turf/random: randomPolygon

Returns a random [polygon][9].

### Parameters

*   `count` **[number][5]** how many geometries will be generated (optional, default `1`)
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][1]** a bounding box inside of which geometries are placed. (optional, default `[-180,-90,180,90]`)
    *   `options.num_vertices` **[number][5]** is how many coordinates each LineString will contain. (optional, default `10`)
    *   `options.max_radial_length` **[number][5]** is the maximum number of decimal degrees latitude or longitude that a
        vertex can reach out of the center of the Polygon. (optional, default `10`)

### Examples

```javascript
var polygons = turf.randomPolygon(25, {bbox: [-180, -90, 180, 90]})
// => polygons
```

*   Throws **[Error][2]** if bbox is invalid

Returns **[FeatureCollection][7]<[Polygon][10]>** GeoJSON FeatureCollection of polygons

## @turf/random: randomLineString

Returns a random [LineString][11].

### Parameters

*   `count` **[number][5]** how many geometries will be generated (optional, default `1`)
*   `options` **[Object][6]** Optional parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][1]** a bounding box inside of which geometries are placed. (optional, default `[-180,-90,180,90]`)
    *   `options.num_vertices` **[number][5]** is how many coordinates each LineString will contain. (optional, default `10`)
    *   `options.max_length` **[number][5]** is the maximum number of decimal degrees that a
        vertex can be from its predecessor (optional, default `0.0001`)
    *   `options.max_rotation` **[number][5]** is the maximum number of radians that a
        line segment can turn from the previous segment. (optional, default `Math.PI/8`)

### Examples

```javascript
var lineStrings = turf.randomLineString(25, {bbox: [-180, -90, 180, 90]})
// => lineStrings
```

*   Throws **[Error][2]** if bbox is invalid

Returns **[FeatureCollection][7]<[LineString][11]>** GeoJSON FeatureCollection of linestrings

[1]: https://tools.ietf.org/html/rfc7946#section-5

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Error

[3]: https://developer.mozilla.org/docs/Web/API/Position

[4]: point

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[7]: https://tools.ietf.org/html/rfc7946#section-3.3

[8]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[9]: polygon

[10]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[11]: https://tools.ietf.org/html/rfc7946#section-3.1.4



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/random
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/rectangle-grid

## @turf/rectangle-grid: rectangleGrid

Creates a grid of rectangular polygons with width and height consistent in degrees

### Parameters

*   `bbox` **[BBox][1]** extent of grid in \[minX, minY, maxX, maxY] order.  If the grid does not fill the bbox perfectly, it is centered.
*   `cellWidth` **[number][2]** width of each cell, in units
*   `cellHeight` **[number][2]** height of each cell, in units
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** the units of the cell width and height value.
        Supports all valid Turf [Units][4].
        If you are looking for rectangles with equal width and height in linear units (e.g. kilometers) this is not the module for you.
        The cellWidth and cellHeight is converted from units provided to degrees internally, so the width and height of resulting polygons will be consistent only in degrees. (optional, default `'kilometers'`)
    *   `options.mask` **[Feature][5]<([Polygon][6] | [MultiPolygon][7])>?** if passed a Polygon or MultiPolygon,
        the grid Points will be created only inside it
    *   `options.properties` **[Object][3]** passed to each point of the grid (optional, default `{}`)

### Examples

```javascript
var bbox = [-95, 30 ,-85, 40];
var cellWidth = 50;
var cellHeight = 20;
var options = {units: 'miles'};

var rectangleGrid = turf.rectangleGrid(bbox, cellWidth, cellHeight, options);

//addToMap
var addToMap = [rectangleGrid]
```

Returns **[FeatureCollection][8]<[Polygon][6]>** a grid of polygons

[1]: https://tools.ietf.org/html/rfc7946#section-5

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://github.com/Turfjs/turf/blob/master/packages/turf-helpers/README_UNITS.md

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[8]: https://tools.ietf.org/html/rfc7946#section-3.3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/rectangle-grid
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/rewind

## @turf/rewind: rewind

Rewind [(Multi)LineString][1] or [(Multi)Polygon][2] outer ring counterclockwise and inner rings clockwise (Uses [Shoelace Formula][3]).

### Parameters

*   `geojson` **[GeoJSON][4]** input GeoJSON Polygon
*   `options` **[Object][5]** Optional parameters (optional, default `{}`)

    *   `options.reverse` **[boolean][6]** enable reverse winding (optional, default `false`)
    *   `options.mutate` **[boolean][6]** allows GeoJSON input to be mutated (significant performance increase if true) (optional, default `false`)

### Examples

```javascript
var polygon = turf.polygon([[[121, -29], [138, -29], [138, -18], [121, -18], [121, -29]]]);

var rewind = turf.rewind(polygon);

//addToMap
var addToMap = [rewind];
```

Returns **[GeoJSON][4]** rewind Polygon

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[3]: http://en.wikipedia.org/wiki/Shoelace_formula

[4]: https://tools.ietf.org/html/rfc7946#section-3

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/rewind
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/rhumb-bearing

## @turf/rhumb-bearing: rhumbBearing

Takes two [points][1] and finds the bearing angle between them along a Rhumb line
i.e. the angle measured in degrees start the north line (0 degrees)

### Parameters

*   `start` **[Coord][2]** starting Point
*   `end` **[Coord][2]** ending Point
*   `options` **[Object][3]?** Optional parameters (optional, default `{}`)

    *   `options.final` **[boolean][4]** calculates the final bearing if true (optional, default `false`)

### Examples

```javascript
var point1 = turf.point([-75.343, 39.984], {"marker-color": "#F00"});
var point2 = turf.point([-75.534, 39.123], {"marker-color": "#00F"});

var bearing = turf.rhumbBearing(point1, point2);

//addToMap
var addToMap = [point1, point2];
point1.properties.bearing = bearing;
point2.properties.bearing = bearing;
```

Returns **[number][5]** bearing from north in decimal degrees, between -180 and 180 degrees (positive clockwise)

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/rhumb-bearing
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/rhumb-destination

## @turf/rhumb-destination: rhumbDestination

Returns the destination [Point][1] having travelled the given distance along a Rhumb line from the
origin Point with the (varant) given bearing.

### Parameters

*   `origin` **[Coord][2]** starting point
*   `distance` **[number][3]** distance from the starting point
*   `bearing` **[number][3]** varant bearing angle ranging from -180 to 180 degrees from north
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][5] (optional, default `'kilometers'`)
    *   `options.properties` **[Object][4]** translate properties to destination point (optional, default `{}`)

### Examples

```javascript
var pt = turf.point([-75.343, 39.984], {"marker-color": "F00"});
var distance = 50;
var bearing = 90;
var options = {units: 'miles'};

var destination = turf.rhumbDestination(pt, distance, bearing, options);

//addToMap
var addToMap = [pt, destination]
destination.properties['marker-color'] = '#00F';
```

Returns **[Feature][6]<[Point][1]>** Destination point.

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://turfjs.org/docs/api/types/Units

[6]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/rhumb-destination
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/rhumb-distance

## @turf/rhumb-distance: rhumbDistance

Calculates the distance along a rhumb line between two [points][1] in [Units][2]

### Parameters

*   `from` **[Coord][3]** origin point
*   `to` **[Coord][3]** destination point
*   `options` **[Object][4]?** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][2] (optional, default `'kilometers'`)

### Examples

```javascript
var from = turf.point([-75.343, 39.984]);
var to = turf.point([-75.534, 39.123]);
var options = {units: 'miles'};

var distance = turf.rhumbDistance(from, to, options);

//addToMap
var addToMap = [from, to];
from.properties.distance = distance;
to.properties.distance = distance;
```

Returns **[number][5]** distance between the two points

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://turfjs.org/docs/api/types/Units

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/rhumb-distance
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/sample

## @turf/sample: sample

Takes a [FeatureCollection][1] and returns a FeatureCollection with given number of [features][2] at random.

### Parameters

*   `fc` **[FeatureCollection][1]\<T>**&#x20;
*   `num` **[number][3]** number of features to select
*   `featurecollection` **[FeatureCollection][1]** set of input features

### Examples

```javascript
var points = turf.randomPoint(100, {bbox: [-80, 30, -60, 60]});

var sample = turf.sample(points, 5);

//addToMap
var addToMap = [points, sample]
turf.featureEach(sample, function (currentFeature) {
  currentFeature.properties['marker-size'] = 'large';
  currentFeature.properties['marker-color'] = '#000';
});
```

Returns **[FeatureCollection][1]** a FeatureCollection with `n` features

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/sample
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/sector

## @turf/sector: sector

Creates a circular sector of a circle of given radius and center [Point][1],
between (clockwise) bearing1 and bearing2; 0 bearing is North of center point, positive clockwise.

### Parameters

*   `center` **[Coord][2]** center point
*   `radius` **[number][3]** radius of the circle
*   `bearing1` **[number][3]** angle, in decimal degrees, of the first radius of the sector
*   `bearing2` **[number][3]** angle, in decimal degrees, of the second radius of the sector
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** Supports all valid Turf [Units][5] (optional, default `'kilometers'`)
    *   `options.steps` **[number][3]** number of steps (optional, default `64`)
    *   `options.properties` **Properties** Translate properties to Feature Polygon (optional, default `{}`)

### Examples

```javascript
var center = turf.point([-75, 40]);
var radius = 5;
var bearing1 = 25;
var bearing2 = 45;

var sector = turf.sector(center, radius, bearing1, bearing2);

//addToMap
var addToMap = [center, sector];
```

Returns **[Feature][6]<[Polygon][7]>** sector polygon

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://turfjs.org/docs/api/types/Units

[6]: https://tools.ietf.org/html/rfc7946#section-3.2

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.6



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/sector
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/shortest-path

## @turf/shortest-path: shortestPath

Returns the shortest [path][1] from [start][2] to [end][2] without colliding with
any [Feature][3] in obstacles [FeatureCollection][4]<[Polygon][5]>

### Parameters

*   `start` **[Coord][6]** point
*   `end` **[Coord][6]** point
*   `options` **[Object][7]** optional parameters (optional, default `{}`)

    *   `options.obstacles` **([Polygon][5] | [Feature][3]<[Polygon][5]> | [FeatureCollection][4]<[Polygon][5]>)?** areas which path cannot travel
    *   `options.units` **Units** unit in which resolution & minimum distance will be expressed in; Supports all valid Turf [Units][8]. (optional, default `'kilometers'`)
    *   `options.resolution` **[number][9]** distance between matrix points on which the path will be calculated (optional, default `100`)

### Examples

```javascript
var start = [-5, -6];
var end = [9, -6];
var options = {
  obstacles: turf.polygon([[[0, -7], [5, -7], [5, -3], [0, -3], [0, -7]]]).geometry
};

var path = turf.shortestPath(start, end, options);

//addToMap
var addToMap = [start, end, options.obstacles, path];
```

Returns **[Feature][3]<[LineString][1]>** shortest path between start and end

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.4

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.2

[4]: https://tools.ietf.org/html/rfc7946#section-3.3

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[8]: https://turfjs.org/docs/api/types/Units

[9]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/shortest-path
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/simplify

## @turf/simplify: simplify

Simplifies the geometries in a GeoJSON object. Uses the 2d version of
[simplify-js][1].

### Parameters

*   `geojson` **[GeoJSON][2]** GeoJSON object to be simplified
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.tolerance` **[number][4]** Simplification tolerance (optional, default `1`)
    *   `options.highQuality` **[boolean][5]** Produce a higher-quality simplification using a slower algorithm (optional, default `false`)
    *   `options.mutate` **[boolean][5]** Allow GeoJSON input to be mutated (significant performance improvement if true) (optional, default `false`)

### Examples

```javascript
const geojson = turf.polygon([[
  [-70.603637, -33.399918],
  [-70.614624, -33.395332],
  [-70.639343, -33.392466],
  [-70.659942, -33.394759],
  [-70.683975, -33.404504],
  [-70.697021, -33.419406],
  [-70.701141, -33.434306],
  [-70.700454, -33.446339],
  [-70.694274, -33.458369],
  [-70.682601, -33.465816],
  [-70.668869, -33.472117],
  [-70.646209, -33.473835],
  [-70.624923, -33.472117],
  [-70.609817, -33.468107],
  [-70.595397, -33.458369],
  [-70.587158, -33.442901],
  [-70.587158, -33.426283],
  [-70.590591, -33.414248],
  [-70.594711, -33.406224],
  [-70.603637, -33.399918]
]]);
const result0_01 = turf.simplify(geojson, {tolerance: 0.01 });
const result0_005 = turf.simplify(geojson, {tolerance: 0.005 });

//addToMap
const addToMap = [geojson, result0_01, result0_005]
```

Returns **[GeoJSON][2]** Simplified GeoJSON

[1]: https://mourner.github.io/simplify-js/

[2]: https://tools.ietf.org/html/rfc7946#section-3

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/simplify
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/square-grid

## @turf/square-grid: squareGrid

Creates a grid of square polygons with cell length consistent in degrees

### Parameters

*   `bbox` **[BBox][1]** extent of grid in \[minX, minY, maxX, maxY] order.  If the grid does not fill the bbox perfectly, it is centered.
*   `cellSide` **[number][2]** length of each cell side.
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** the units of the cellSide value.
        Supports all valid Turf [Units][4].
        If you are looking for squares with sides of equal lengths in linear units (e.g. kilometers) this is not the module for you.
        The cellSide is converted from units provided to degrees internally, so the width and height of resulting polygons will be consistent only in degrees. (optional, default `'kilometers'`)
    *   `options.mask` **[Feature][5]<([Polygon][6] | [MultiPolygon][7])>?** if passed a Polygon or MultiPolygon,
        the grid Points will be created only inside it
    *   `options.properties` **[Object][3]** passed to each point of the grid (optional, default `{}`)

### Examples

```javascript
var bbox = [-95, 30 ,-85, 40];
var cellSide = 50;
var options = {units: 'miles'};

var squareGrid = turf.squareGrid(bbox, cellSide, options);

//addToMap
var addToMap = [squareGrid]
```

Returns **[FeatureCollection][8]<[Polygon][6]>** a grid of polygons with equal width and height in degrees.

[1]: https://tools.ietf.org/html/rfc7946#section-5

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://github.com/Turfjs/turf/blob/master/packages/turf-helpers/README_UNITS.md

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[7]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[8]: https://tools.ietf.org/html/rfc7946#section-3.3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/square-grid
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/square

## @turf/square: square

Takes a bounding box and calculates the minimum square bounding box that
would contain the input.

### Parameters

*   `bbox` **[BBox][1]** extent in \[west, south, east, north] order

### Examples

```javascript
const bbox = [-20, -20, -15, 0];
const squared = turf.square(bbox);

//addToMap
const addToMap = [turf.bboxPolygon(bbox), turf.bboxPolygon(squared)]
```

Returns **[BBox][1]** a square surrounding `bbox`

[1]: https://tools.ietf.org/html/rfc7946#section-5



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/square
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/standard-deviational-ellipse

## @turf/standard-deviational-ellipse: standardDeviationalEllipse

Takes a collection of features and returns a standard deviational ellipse,
also known as a “directional distribution.” The standard deviational ellipse
aims to show the direction and the distribution of a dataset by drawing
an ellipse that contains about one standard deviation’s worth (\~ 70%) of the
data.

This module mirrors the functionality of [Directional Distribution][1]
in ArcGIS and the [QGIS Standard Deviational Ellipse Plugin][2]

**Bibliography**

• Robert S. Yuill, “The Standard Deviational Ellipse; An Updated Tool for
Spatial Description,” *Geografiska Annaler* 53, no. 1 (1971): 28–39,
doi:{@link [https://doi.org/10.2307/490885|10.2307/490885}][3].

• Paul Hanly Furfey, “A Note on Lefever’s “Standard Deviational Ellipse,”
*American Journal of Sociology* 33, no. 1 (1927): 94—98,
doi:{@link [https://doi.org/10.1086/214336|10.1086/214336}][4].

### Parameters

*   `points` **[FeatureCollection][5]<[Point][6]>** GeoJSON points
*   `options` **[Object][7]** Optional parameters (optional, default `{}`)

    *   `options.weight` **[string][8]?** the property name used to weight the center
    *   `options.steps` **[number][9]** number of steps for the polygon (optional, default `64`)
    *   `options.properties` **[Object][7]** properties to pass to the resulting ellipse (optional, default `{}`)

### Examples

```javascript
const bbox = [-74, 40.72, -73.98, 40.74];
const points = turf.randomPoint(400, {bbox: bbox});
const sdEllipse = turf.standardDeviationalEllipse(points);

//addToMap
const addToMap = [points, sdEllipse];
```

Returns **[Feature][10]<[Polygon][11]>** an elliptical Polygon that includes approximately 1 SD of the dataset within it.

[1]: http://desktop.arcgis.com/en/arcmap/10.3/tools/spatial-statistics-toolbox/directional-distribution.htm

[2]: http://arken.nmbu.no/~havatv/gis/qgisplugins/SDEllipse/

[3]: https://doi.org/10.2307/490885|10.2307/490885}

[4]: https://doi.org/10.1086/214336|10.1086/214336}

[5]: https://tools.ietf.org/html/rfc7946#section-3.3

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[7]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[8]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[9]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[10]: https://tools.ietf.org/html/rfc7946#section-3.2

[11]: https://tools.ietf.org/html/rfc7946#section-3.1.6



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/standard-deviational-ellipse
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/tag

## @turf/tag: tag

Takes a set of [points][1] and a set of [polygons][2] and/or [multipolygons][3] and performs a spatial join.

### Parameters

*   `points` **[FeatureCollection][4]<[Point][1]>** input points
*   `polygons` **[FeatureCollection][4]<([Polygon][2] | [MultiPolygon][3])>** input (multi)polygons
*   `field` **[string][5]** property in `polygons` to add to joined {<Point>} features
*   `outField` **[string][5]** property in `points` in which to store joined property from `polygons`

### Examples

```javascript
var pt1 = turf.point([-77, 44]);
var pt2 = turf.point([-77, 38]);
var poly1 = turf.polygon([[
  [-81, 41],
  [-81, 47],
  [-72, 47],
  [-72, 41],
  [-81, 41]
]], {pop: 3000});
var poly2 = turf.polygon([[
  [-81, 35],
  [-81, 41],
  [-72, 41],
  [-72, 35],
  [-81, 35]
]], {pop: 1000});

var points = turf.featureCollection([pt1, pt2]);
var polygons = turf.featureCollection([poly1, poly2]);

var tagged = turf.tag(points, polygons, 'pop', 'population');

//addToMap
var addToMap = [tagged, polygons]
```

Returns **[FeatureCollection][4]<[Point][1]>** points with `containingPolyId` property containing values from `polyId`

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[4]: https://tools.ietf.org/html/rfc7946#section-3.3

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/tag
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/tesselate

## @turf/tesselate: tesselate

Tesselates a polygon or multipolygon into a collection of triangle polygons
using [earcut][1].

### Parameters

*   `poly` **[Feature][2]<([Polygon][3] | [MultiPolygon][4])>** the polygon to tesselate

### Examples

```javascript
const poly = turf.polygon([[[11, 0], [22, 4], [31, 0], [31, 11], [21, 15], [11, 11], [11, 0]]]);
const triangles = turf.tesselate(poly);

//addToMap
const addToMap = [poly, triangles]
```

Returns **[FeatureCollection][5]<[Polygon][3]>** collection of polygon tesselations

[1]: https://github.com/mapbox/earcut

[2]: https://tools.ietf.org/html/rfc7946#section-3.2

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[5]: https://tools.ietf.org/html/rfc7946#section-3.3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/tesselate
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/tin

## @turf/tin: tin

Takes a set of [points][1] and creates a
[Triangulated Irregular Network][2],
or a TIN for short, returned as a collection of Polygons. These are often used
for developing elevation contour maps or stepped heat visualizations.

If an optional z-value property is provided then it is added as properties called `a`, `b`,
and `c` representing its value at each of the points that represent the corners of the
triangle.

### Parameters

*   `points` **[FeatureCollection][3]<[Point][1]>** input points
*   `z` **[String][4]?** name of the property from which to pull z values
    This is optional: if not given, then there will be no extra data added to the derived triangles.

### Examples

```javascript
// generate some random point data
var points = turf.randomPoint(30, {bbox: [50, 30, 70, 50]});

// add a random property to each point between 0 and 9
for (var i = 0; i < points.features.length; i++) {
  points.features[i].properties.z = ~~(Math.random() * 9);
}
var tin = turf.tin(points, 'z');

//addToMap
var addToMap = [tin, points]
for (var i = 0; i < tin.features.length; i++) {
  var properties  = tin.features[i].properties;
  properties.fill = '#' + properties.a + properties.b + properties.c;
}
```

Returns **[FeatureCollection][3]<[Polygon][5]>** TIN output

[1]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[2]: http://en.wikipedia.org/wiki/Triangulated_irregular_network

[3]: https://tools.ietf.org/html/rfc7946#section-3.3

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/String

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.6



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/tin
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/transform-rotate

## @turf/transform-rotate: transformRotate

Rotates any geojson Feature or Geometry of a specified angle, around its `centroid` or a given `pivot` point.

### Parameters

*   `geojson` **[GeoJSON][1]** object to be rotated
*   `angle` **[number][2]** of rotation in decimal degrees, positive clockwise
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.pivot` **[Coord][4]** point around which the rotation will be performed (optional, default `'centroid'`)
    *   `options.mutate` **[boolean][5]** allows GeoJSON input to be mutated (significant performance increase if true) (optional, default `false`)

### Examples

```javascript
const poly = turf.polygon([[[0,29],[3.5,29],[2.5,32],[0,29]]]);
const options = {pivot: [0, 25]};
const rotatedPoly = turf.transformRotate(poly, 10, options);

//addToMap
const addToMap = [poly, rotatedPoly];
rotatedPoly.properties = {stroke: '#F00', 'stroke-width': 4};
```

Returns **[GeoJSON][1]** the rotated GeoJSON feature

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[5]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/transform-rotate
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/transform-scale

## @turf/transform-scale: transformScale

Scale GeoJSON objects from a given point by a scaling factor e.g. factor=2
would make each object 200% larger.
If a FeatureCollection is provided, the origin point will be calculated
based on each individual feature *unless* an exact

### Parameters

*   `geojson` **([GeoJSON][1] | [GeometryCollection][2])** objects to be scaled
*   `factor` **[number][3]** of scaling, positive values greater than 0. Numbers between 0 and 1 will shrink the geojson, numbers greater than 1 will expand it, a factor of 1 will not change the geojson.
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.origin` **(Corners | [Coord][5])** Point from which the scaling will occur (string options: sw/se/nw/ne/center/centroid) (optional, default `'centroid'`)
    *   `options.mutate` **[boolean][6]** allows GeoJSON input to be mutated (significant performance improvement if true) (optional, default `false`)

### Examples

```javascript
const poly = turf.polygon([[[0,29],[3.5,29],[2.5,32],[0,29]]]);
const scaledPoly = turf.transformScale(poly, 3);

//addToMap
const addToMap = [poly, scaledPoly];
scaledPoly.properties = {stroke: '#F00', 'stroke-width': 4};
```

Returns **([GeoJSON][1] | [GeometryCollection][2])** scaled GeoJSON

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.8

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.1

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/transform-scale
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/transform-translate

## @turf/transform-translate: transformTranslate

Moves any geojson Feature or Geometry of a specified distance along a Rhumb Line
on the provided direction angle.

Note that this moves the points of your shape individually and can therefore change
the overall shape. How noticable this is depends on the distance and the used projection.

### Parameters

*   `geojson` **([GeoJSON][1] | [GeometryCollection][2])** object to be translated
*   `distance` **[number][3]** length of the motion; negative values determine motion in opposite direction
*   `direction` **[number][3]** of the motion; angle from North in decimal degrees, positive clockwise
*   `options` **[Object][4]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** in which `distance` will be expressed; Supports all valid Turf [Units][5] (optional, default `'kilometers'`)
    *   `options.zTranslation` **[number][3]** length of the vertical motion, same unit of distance (optional, default `0`)
    *   `options.mutate` **[boolean][6]** allows GeoJSON input to be mutated (significant performance increase if true) (optional, default `false`)

### Examples

```javascript
var poly = turf.polygon([[[0,29],[3.5,29],[2.5,32],[0,29]]]);
var translatedPoly = turf.transformTranslate(poly, 100, 35);

//addToMap
var addToMap = [poly, translatedPoly];
translatedPoly.properties = {stroke: '#F00', 'stroke-width': 4};
```

Returns **([GeoJSON][1] | [GeometryCollection][2])** the translated GeoJSON object

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.8

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://turfjs.org/docs/api/types/Units

[6]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/transform-translate
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/triangle-grid

## @turf/triangle-grid: triangleGrid

Creates a grid of triangular polygons.

### Parameters

*   `bbox` **[BBox][1]** extent of grid in \[minX, minY, maxX, maxY] order
*   `cellSide` **[number][2]** dimension of each grid cell.  Two triangles are created in each cell.
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.units` **Units** used in calculating cellSide.  Supports all valid Turf [Units][4] (optional, default `'kilometers'`)
    *   `options.mask` **[Feature][5]<[Polygon][6]>?** if passed a Polygon or MultiPolygon, the grid Points will be created only inside it
    *   `options.properties` **[Object][3]** passed to each point of the grid (optional, default `{}`)

### Examples

```javascript
var bbox = [-95, 30 ,-85, 40];
var cellSide = 50;
var options = {units: 'miles'};

var triangleGrid = turf.triangleGrid(bbox, cellSide, options);

//addToMap
var addToMap = [triangleGrid];
```

Returns **[FeatureCollection][7]<[Polygon][6]>** grid of polygons

[1]: https://tools.ietf.org/html/rfc7946#section-5

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://github.com/Turfjs/turf/blob/master/packages/turf-helpers/README_UNITS.md

[5]: https://tools.ietf.org/html/rfc7946#section-3.2

[6]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[7]: https://tools.ietf.org/html/rfc7946#section-3.3



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/triangle-grid
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/truncate

## @turf/truncate: truncate

Takes a GeoJSON Feature or FeatureCollection and truncates the precision of the geometry.

### Parameters

*   `geojson` **[GeoJSON][1]** any GeoJSON Feature, FeatureCollection, Geometry or GeometryCollection.
*   `options` **[Object][2]** Optional parameters (optional, default `{}`)

    *   `options.precision` **[number][3]** coordinate decimal precision (optional, default `6`)
    *   `options.coordinates` **[number][3]** maximum number of coordinates (primarly used to remove z coordinates) (optional, default `3`)
    *   `options.mutate` **[boolean][4]** allows GeoJSON input to be mutated (significant performance increase if true) (optional, default `false`)

### Examples

```javascript
var point = turf.point([
    70.46923055566859,
    58.11088890802906,
    1508
]);
var options = {precision: 3, coordinates: 2};
var truncated = turf.truncate(point, options);
//=truncated.geometry.coordinates => [70.469, 58.111]

//addToMap
var addToMap = [truncated];
```

Returns **[GeoJSON][1]** layer with truncated geometry

[1]: https://tools.ietf.org/html/rfc7946#section-3

[2]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Number

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Boolean



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/truncate
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/union

## @turf/union: union

Takes a collection of input polygons and returns a combined polygon. If the
input polygons are not contiguous, this function returns a multi-polygon
feature.

### Parameters

*   `features` **[FeatureCollection][1]<([Polygon][2] | [MultiPolygon][3])>** input polygon features
*   `options` **[Object][4]** Optional Parameters (optional, default `{}`)

    *   `options.properties` **[GeoJsonProperties][5]** properties to assign to output feature (optional, default `{}`)

### Examples

```javascript
const poly1 = turf.polygon(
  [
    [
      [-82.574787, 35.594087],
      [-82.574787, 35.615581],
      [-82.545261, 35.615581],
      [-82.545261, 35.594087],
      [-82.574787, 35.594087],
    ],
  ],
  { fill: "#0f0" }
);

const poly2 = turf.polygon(
  [
    [
      [-82.560024, 35.585153],
      [-82.560024, 35.602602],
      [-82.52964, 35.602602],
      [-82.52964, 35.585153],
      [-82.560024, 35.585153],
    ],
  ],
);

const union = turf.union(turf.featureCollection([poly1, poly2]));

//addToMap
const addToMap = { poly1, poly2, union };

poly1.properties.fill = "#0f0";
poly2.properties.fill = "#00f";
union.properties.stroke = "red";
union.properties["stroke-width"] = 4;
union.properties.fill = "transparent";
```

Returns **([Feature][5]<([Polygon][2] | [MultiPolygon][3])> | null)** a combined polygon or multi-polygon feature, or null if there were no input polygons to combine

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[4]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[5]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/union
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/unkink-polygon

## @turf/unkink-polygon: unkinkPolygon

Takes a kinked polygon and returns a feature collection of polygons that have
no kinks.

Uses [simplepolygon][1] internally.

### Parameters

*   `geojson` **([FeatureCollection][2]<([Polygon][3] | [MultiPolygon][4])> | [Feature][5]<([Polygon][3] | [MultiPolygon][4])> | [Polygon][3] | [MultiPolygon][4])** polygons to unkink

### Examples

```javascript
const poly = turf.polygon([[[0, 0], [2, 0], [0, 2], [2, 2], [0, 0]]]);

const result = turf.unkinkPolygon(poly);

//addToMap
const addToMap = [poly, result]
```

Returns **[FeatureCollection][2]<[Polygon][3]>** Unkinked polygons

[1]: https://github.com/mclaeysb/simplepolygon

[2]: https://tools.ietf.org/html/rfc7946#section-3.3

[3]: https://tools.ietf.org/html/rfc7946#section-3.1.6

[4]: https://tools.ietf.org/html/rfc7946#section-3.1.7

[5]: https://tools.ietf.org/html/rfc7946#section-3.2



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/unkink-polygon
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```


## @turf/voronoi

## @turf/voronoi: voronoi

Takes a collection of points and a bounding box, and returns a collection
of Voronoi polygons.

The Voronoi algorithim used comes from the d3-voronoi package.

### Parameters

*   `points` **[FeatureCollection][1]<[Point][2]>** points around which to calculate the Voronoi polygons
*   `options` **[Object][3]** Optional parameters (optional, default `{}`)

    *   `options.bbox` **[BBox][4]** clipping rectangle, in \[minX, minY, maxX, MaxY] order (optional, default `[-180,-85,180,-85]`)

### Examples

```javascript
const options = {
  bbox: [-70, 40, -60, 60]
};
const points = turf.randomPoint(100, options);
const voronoiPolygons = turf.voronoi(points, options);

//addToMap
const addToMap = [voronoiPolygons, points];
```

Returns **[FeatureCollection][1]<[Polygon][5]>** a set of polygons, one per input point

[1]: https://tools.ietf.org/html/rfc7946#section-3.3

[2]: https://tools.ietf.org/html/rfc7946#section-3.1.2

[3]: https://developer.mozilla.org/docs/Web/JavaScript/Reference/Global_Objects/Object

[4]: https://tools.ietf.org/html/rfc7946#section-5

[5]: https://tools.ietf.org/html/rfc7946#section-3.1.6



---

This module is part of the [Turfjs project](https://turfjs.org/), an open source module collection dedicated to geographic algorithms. It is maintained in the [Turfjs/turf](https://github.com/Turfjs/turf) repository, where you can create PRs and issues.

### Installation

Install this single module individually:

```not-for-this-stick
$ npm install @turf/voronoi
```

Or install the all-encompassing @turf/turf module that includes all modules as functions:

```not-for-this-stick
$ npm install @turf/turf
```



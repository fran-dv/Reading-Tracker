<!-- Source: https://data-star.dev/examples/svg_morphing -->
<!-- Fetched: 2026-09-14 -->

# SVG Morphing 

Morphing elements within SVG elements is a little more invloved than standard HTML elements. This is because, as an XML dialect, SVG is [namespaced](https://developer.mozilla.org/en-US/docs/Web/SVG/Guides/Namespaces_crash_course). This means that `[<svg>](https://developer.mozilla.org/en-US/docs/Web/SVG/Element/svg)` elements (as well as `[<math>](https://developer.mozilla.org/en-US/docs/Web/SVG/Element/math)` elements) create their own namespace, separate from the HTML namespace. 

To morph an SVG element, you must either provide a `namespace` data line in the [`datastar-patch-elements`](https://data-star.dev/reference/sse_events#datastar-patch-elements) event:

```text
event: datastar-patch-elements
data: namespace svg
data: elements <circle id="circle" cx="100" r="50" cy="75"></circle>
```

Or, alternatively, ensure that the target element is wrapped in an `<svg>` tag:

```html
<svg id="circle">
    <circle cx="50" cy="100" r="50" fill="red" />
</svg>
```

## Basic Circle Color Change #

This example demonstrates morphing an SVG circle’s color. Click the button to change the circle from red to blue.

Demo

Change Color

```
svgMorphingRouter.Get("/circle_color", func(w http.ResponseWriter, r *http.Request) {
    sse := datastar.NewSSE(w, r)
    color := svgColors[rand.N(len(svgColors))]
    sse.PatchElements(fmt.Sprintf(`<svg id="circle-demo"><circle cx="50" cy="50" r="40" fill="%s" /></svg>`, color))
})
```

## Circle Radius Change #

This example shows how to morph the size of an SVG element. The circle will change to a random radius when you click the button.

Demo

Change Radius

```
svgMorphingRouter.Get("/circle_size", func(w http.ResponseWriter, r *http.Request) {
    sse := datastar.NewSSE(w, r)
    radius := 15 + rand.N(45) // Random radius between 15-60
    sse.PatchElements(fmt.Sprintf(`<svg id="size-demo"><circle cx="50" cy="50" r="%d" fill="green" /></svg>`, radius))
})
```

## Random Shape Transformation #

SVG morphing can handle changing between different shape types. This example morphs to a random shape each time you click.

Demo

Random Shape

```
svgMorphingRouter.Get("/shape_transform", func(w http.ResponseWriter, r *http.Request) {
    sse := datastar.NewSSE(w, r)
    shape := svgShapes[rand.N(len(svgShapes))]
    sse.PatchElements(fmt.Sprintf(`<svg id="shape-demo">%s</svg>`, shape))
})
```

## Multiple Random Elements #

You can morph multiple SVG elements at once. This example updates three circles with random colors and sizes each time you click.

Demo

Randomize All Circles

```
svgMorphingRouter.Get("/multiple_elements", func(w http.ResponseWriter, r *http.Request) {
    sse := datastar.NewSSE(w, r)
    color1 := svgColors[rand.N(len(svgColors))]
    color2 := svgColors[rand.N(len(svgColors))]
    color3 := svgColors[rand.N(len(svgColors))]
    r1 := 10 + rand.N(20) // radius 10-30
    r2 := 10 + rand.N(20)
    r3 := 10 + rand.N(20)
    sse.PatchElements(fmt.Sprintf(`<svg id="multi-demo">
        <circle cx="30" cy="30" r="%d" fill="%s" />
        <circle cx="70" cy="30" r="%d" fill="%s" />
        <circle cx="50" cy="70" r="%d" fill="%s" />
    </svg>`, r1, color1, r2, color2, r3, color3))
})
```

## Animated Sequence #

This example demonstrates a sequence of SVG morphs that happen automatically when triggered, creating an animation effect.

Demo

Start Animation Sequence

```
svgMorphingRouter.Get("/animated_morph", func(w http.ResponseWriter, r *http.Request) {
    sse := datastar.NewSSE(w, r)
    
    // First morph
    sse.PatchElements(`<svg id="animated-demo"><circle cx="50" cy="50" r="30" fill="red" /></svg>`)
    time.Sleep(500 * time.Millisecond)
    
    // Second morph
    sse.PatchElements(`<svg id="animated-demo"><circle cx="50" cy="50" r="45" fill="orange" /></svg>`)
    time.Sleep(500 * time.Millisecond)
    
    // Third morph
    sse.PatchElements(`<svg id="animated-demo"><circle cx="50" cy="50" r="60" fill="yellow" /></svg>`)
    time.Sleep(500 * time.Millisecond)
    
    // Reset
    sse.PatchElements(`<svg id="animated-demo"><circle cx="50" cy="50" r="20" fill="green" /></svg>`)
})
```

## Key Points #

  * SVG elements must be wrapped in an outer `<svg>` container
  * The inner `<svg>` element should have the target ID
  * All SVG element types (circle, rect, path, etc.) can be morphed
  * Multiple SVG elements can be updated in a single morph operation
  * CSS transitions work with SVG morphing for smooth animations

<!-- Source: https://data-star.dev/reference/sse_events -->
<!-- Fetched: 2026-09-14 -->

# SSE Events

Responses to [backend actions](https://data-star.dev/reference/actions#backend-actions) with a content type of `text/event-stream` can contain zero or more Datastar [SSE events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events).

> The backend [SDKs](https://data-star.dev/reference/sdks) can handle the formatting of SSE events for you, or you can format them yourself.

## Event Types #

### `datastar-patch-elements` #

Patches one or more elements in the DOM. By default, Datastar morphs elements by matching top-level elements based on their ID.

```text
event: datastar-patch-elements
data: elements <div id="foo">Hello world!</div>
```

In the example above, the element `<div id="foo">Hello world!</div>` will be morphed into the target element with ID `foo`. Note that SSE events must be followed by two newline characters.

> Be sure to place IDs on top-level elements to be morphed, as well as on elements within them that you’d like to preserve state on (event listeners, CSS transitions, etc.).

Morphing elements within SVG elements requires special handling due to XML namespaces. See the [SVG morphing example](https://data-star.dev/examples/svg_morphing).

Additional `data` lines can be added to the response to override the default behavior.

Key| Description  
---|---  
`data: selector #foo`| Selects the target element of the patch using a CSS selector. Not required when using the `outer` or `replace` modes.  
`data: mode outer`| Morphs the outer HTML of the elements. This is the default (and recommended) mode.  
`data: mode inner`| Morphs the inner HTML of the elements.  
`data: mode replace`| Replaces the outer HTML of the elements.  
`data: mode prepend`| Prepends the elements to the target’s children.  
`data: mode append`| Appends the elements to the target’s children.  
`data: mode before`| Inserts the elements before the target as siblings.  
`data: mode after`| Inserts the elements after the target as siblings.  
`data: mode remove`| Removes the target elements from DOM.  
`data: namespace svg`| Patch elements into the DOM using an `svg` namespace.  
`data: namespace mathml`| Patch elements into the DOM using a `mathml` namespace.  
`data: useViewTransition true`| Whether to use view transitions when patching elements. Defaults to `false`.  
`data: viewTransitionSelector #main`| Specifies the CSS selector for the element to use for view transitions when patching elements. Defaults to `document`.  
`data: elements`| The HTML elements to patch.  
  
```text
event: datastar-patch-elements
data: elements <div id="foo">Hello world!</div>
```

Elements can be removed using the `remove` mode and providing a `selector`.

```text
event: datastar-patch-elements
data: selector #foo
data: mode remove
```

Elements can span multiple lines. Sample output showing non-default options:

```text
event: datastar-patch-elements
data: selector #foo
data: mode inner
data: useViewTransition true
data: viewTransitionSelector #main
data: elements <div>
data: elements        Hello world!
data: elements </div>
```

Elements can be patched using `svg` and `mathml` namespaces by specifying the `namespace` data line.

```text
event: datastar-patch-elements
data: namespace svg
data: elements <circle id="circle" cx="100" r="50" cy="75"></circle>
```

### `datastar-patch-signals` #

Patches signals into the existing signals on the page. The `onlyIfMissing` line determines whether to update each signal with the new value only if a signal with that name does not yet exist. The `signals` line should be a valid `data-signals` attribute.

```text
event: datastar-patch-signals
data: signals {foo: 1, bar: 2}
```

Signals can be removed by setting their values to `null`.

```text
event: datastar-patch-signals
data: signals {foo: null, bar: null}
```

Sample output showing non-default options:

```text
event: datastar-patch-signals
data: onlyIfMissing true
data: signals {foo: 1, bar: 2}
```

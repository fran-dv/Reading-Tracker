<!-- Source: https://data-star.dev/examples/lazy_load -->
<!-- Fetched: 2026-09-14 -->

# Lazy Load 

Demo

Loading...

## Explanation #

This example shows how to lazily load an element on a page. We start with an initial state that looks like this:

```html
<div id="graph" data-init="@get('/examples/lazy_load/graph')">
    Loading...
</div>
```

Which shows a progress indicator as we are loading the graph. The graph is loaded by patching an element with the same ID.

```html
<div id="graph">
    <img src="/images/examples/tokyo.png" />
</div>
```

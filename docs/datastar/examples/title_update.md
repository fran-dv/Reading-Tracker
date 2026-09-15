<!-- Source: https://data-star.dev/examples/title_update -->
<!-- Fetched: 2026-09-14 -->

# Title Update 

Demo

Look at the title change in the browser tab!

## Explanation #

A user in the Discord channel was asking about needing a plugin similar to htmx’s head support to update title or head elements. With Datastar this is unnecessary as you can just update the title directly with a patch elements event.

```text
event: datastar-patch-elements
data: selector title
data: elements <title>08:30:36</title>
```

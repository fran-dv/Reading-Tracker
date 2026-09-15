<!-- Source: https://data-star.dev/how_tos/prevent_sse_connections_closing -->
<!-- Fetched: 2026-09-14 -->

# How to prevent SSE connections closing

When a page is hidden (in a background tab, for example), the default behavior is for the SSE connection to be closed, and reopened when the page becomes visible again. This is to save resources on both the client and server.

To keep the connection open even when the page is hidden, you can set the [`openWhenHidden`](https://data-star.dev/reference/actions#openWhenHidden) option to `true`.

```html
<button data-on:click="@get('/endpoint', {openWhenHidden: true})"></button>
```

### CQRS Pattern #

When using the [CQRS pattern](https://martinfowler.com/bliki/CQRS.html), it’s best to design event streams with interruptions in mind, since they can occur for many reasons beyond just tab switching. The simplest way to ensure resilience is to use a “fat morph” approach: send the complete desired state of the main content area with each update instead of incremental changes like append, which are much more vulnerable to interruptions.

Here’s a simple example of a CQRS approach in which the main content area is always kept up to date. This way, you can leave `openWhenHidden` as is, and if the SSE connection is interrupted for any reason, the next event will contain the complete and correct state of the main content area.

```html
<div data-init="@get('/cqrs_endpoint')"></div>
<div id="main">
    ...
</div>
```

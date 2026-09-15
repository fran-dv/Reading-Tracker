<!-- Source: https://data-star.dev/examples/templ_counter -->
<!-- Fetched: 2026-09-14 -->

# Templ Counter 

Demo

Increment Global: 203Increment User: 0

## HTML #

```html
<div data-init="@get('/examples/templ_counter/updates')">
    <!-- Global Counter -->
    <button
        id="global"
        class="info"
        data-on:click="@patch('/examples/templ_counter/global')"
    >
        Global Clicks: 0
    </button>

    <!-- User Counter -->
    <button
        id="user"
        class="success"
        data-on:click="@patch('/examples/templ_counter/user')"
    >
        User Clicks: 0
    </button>
</div>
```

## Explanation #

This example demonstrates two counters - a global counter shared across all users and a user-specific counter. The counters are updated via server-sent events (SSE) and increment when clicked.

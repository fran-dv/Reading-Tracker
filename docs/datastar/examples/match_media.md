<!-- Source: https://data-star.dev/examples/match_media -->
<!-- Fetched: 2026-09-14 -->

# Match Media [Pro](https://data-star.dev/pro "Datastar Pro")

Demo

System dark-mode match: ``

****

## Explanation #

`data-match-media` keeps a signal synced with a media query match state. In this example, `$isDark` tracks `prefers-color-scheme: dark` and class bindings switch styling.

The query can be written without surrounding quotes or parentheses; the plugin normalizes it before calling `window.matchMedia`.

For more complex queries, use a quoted query string with explicit media-query syntax and parentheses so the exact browser query is preserved.

### Usage Example #

```html
<div
    data-match-media:is-dark="prefers-color-scheme: dark"
    class="match-media-card"
    data-class:dark="$isDark"
>
    <p>System dark-mode match: <code data-text="$isDark"></code></p>
</div>
```

<!-- Source: https://data-star.dev/examples/progress_bar -->
<!-- Fetched: 2026-09-14 -->

# Progress Bar 

Demo

0%

## HTML #

```html
<div id="progress-bar"
     data-init="@get('/examples/progress_bar/updates', {openWhenHidden: true})"
>
    <svg
        width="200"
        height="200"
        viewbox="-25 -25 250 250"
        style="transform: rotate(-90deg)"
    >
        <circle
            r="90"
            cx="100"
            cy="100"
            fill="transparent"
            stroke="#e0e0e0"
            stroke-width="16px"
            stroke-dasharray="565.48px"
            stroke-dashoffset="565px"
        ></circle>
        <circle
            r="90"
            cx="100"
            cy="100"
            fill="transparent"
            stroke="#6bdba7"
            stroke-width="16px"
            stroke-linecap="round"
            stroke-dashoffset="282px"
            stroke-dasharray="565.48px"
        ></circle>
        <text
            x="44px"
            y="115px"
            fill="#6bdba7"
            font-size="52px"
            font-weight="bold"
            style="transform:rotate(90deg) translate(0px, -196px)"
        >50%</text>
    </svg>
    
    <div data-on:click="@get('/examples/progress_bar/updates', {openWhenHidden: true})">
        <!-- When progress is 100% -->
        <button>
            Completed! Try again?
        </button>
    </div>
</div>
```

## Explanation #

This example shows an updating progress graphic using SSE. The server sends down a new progress bar svg every 500 milliseconds causing the client to update. After the progress is complete, the server sends down a button allowing the user to restart the progress bar.

### Note #

The `openWhenHidden` option is used to keep the connection open even when the progress bar is not visible. This is useful for when the user navigates away from the page and then returns.

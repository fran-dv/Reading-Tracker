<!-- Source: https://data-star.dev/examples/on_signal_patch -->
<!-- Fetched: 2026-09-14 -->

# On Signal Patch 

Demo

Update Message Increment Counter Clear All Changes

### Current Values

Counter: 

Message: 

### Counter Changes Only

```

```

### All Signal Changes

```

```

## Explanation #

```html
<div data-signals="{counter: 0, message: 'Hello World', allChanges: [], counterChanges: []}">
    <div class="actions">
        <button data-on:click="$message = `Updated: ${performance.now().toFixed(2)}`">
            Update Message
        </button>
        <button data-on:click="$counter++">
            Increment Counter
        </button>
        <button
            class="error"
            data-on:click="$allChanges.length = 0; $counterChanges.length = 0"
        >
            Clear All Changes
        </button>
    </div>
    <div>
        <h3>Current Values</h3>
        <p>Counter: <span data-text="$counter"></span></p>
        <p>Message: <span data-text="$message"></span></p>
    </div>
    <div
        data-on-signal-patch="$counterChanges.push(patch)"
        data-on-signal-patch-filter="{include: /^counter$/}"
    >
        <h3>Counter Changes Only</h3>
        <pre data-json-signals__terse="{include: /^counterChanges/}"></pre>
    </div>
    <div
        data-on-signal-patch="$allChanges.push(patch)"
        data-on-signal-patch-filter="{exclude: /allChanges|counterChanges/}"
    >
        <h3>All Signal Changes</h3>
        <pre data-json-signals__terse="{include: /^allChanges/}"></pre>
    </div>
</div>
```

The [`data-on-signal-patch`](https://data-star.dev/reference/attributes#data-on-signal-patch) plugin allows you to execute an expression whenever signals are patched. This is useful for tracking changes, updating dependent values, or triggering side effects.

You can filter which signals to watch using the `data-on-signal-patch-filter` attribute with include/exclude patterns, as seen above.

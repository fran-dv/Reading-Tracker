<!-- Source: https://data-star.dev/examples/custom_plugin -->
<!-- Fetched: 2026-09-14 -->

# Custom Plugin 

Demo

Alert using an action Alert using an attribute

## Explanation #

Custom actions, attributes, and watchers can be implemented using the plugin API (documentation is in progress). This example implements a simple alert action and attribute.

### Action #

An `action` plugin can be implemented as follows.

```
action({
    name: 'alert',
    apply(ctx, value) {
        alert(value)
    }
})
```

Setting the `name` to `alert` results in the syntax `@alert`.

```html
<button data-on:click="@alert('Hello from an action')">
    Alert using an action
</button>
```

### Attribute #

An `attribute` plugin can be implemented as follows.

```javascript
attribute({
    name: 'alert',
    requirement: {
        key: 'denied',
        value: 'must',
    },
    returnsValue: true,
    apply({ el, rx }) {
        const callback = () => alert(rx())
        el.addEventListener('click', callback)
        return () => el.removeEventListener('click', callback)
    }
})
```

Setting the `name` to `alert` results in the syntax `data-alert`.

The attribute shouldn’t take a key and needs a value, so `key` is `denied` and `value` is a `must`. The attribute expects a value to be returned from the expression so we set `returnsValue` to `true`.

On `apply`, we create an event listener that alerts the value returned from the expression when the element is clicked. We return a function that removes the event listener on `cleanup`.

```html
<button data-alert="'Hello from an attribute'">
    Alert using an attribute
</button>
```

<!-- Source: https://data-star.dev/examples/web_component -->
<!-- Fetched: 2026-09-14 -->

# Web Component 

Demo

Reversed 

## Explanation #

This is an example of two-way binding with a web component that reverses a string. Normally, the web component would output the reversed value, but in this example, all it does is perform the logic and dispatch an event containing the result, which is then displayed.

```html
<label>
    Reversed
    <input type="text" value="Your Name" data-bind:_name/>
</label>
<span data-signals:_reversed data-text="$_reversed"></span>
<reverse-component
    data-on:reverse="$_reversed = evt.detail.value"
    data-attr:name="$_name"
></reverse-component>
```

The `name` attribute value is bound to the `$_name` signal's value, and an event listener modifies the `$_reversed` signal's value sent in the `reverse` event. The web component observes changes to the `name` attribute and responds by reversing the string and dispatching a `reverse` event containing the resulting value.

```javascript
class ReverseComponent extends HTMLElement {
    static get observedAttributes() {
        return ["name"];
    }

    attributeChangedCallback(name, oldValue, newValue) {
        const value = [...newValue].toReversed().join("");
        this.dispatchEvent(new CustomEvent("reverse", { detail: { value } }));
    }
}

customElements.define("reverse-component", ReverseComponent);
```

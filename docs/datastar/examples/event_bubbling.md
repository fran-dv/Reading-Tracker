<!-- Source: https://data-star.dev/examples/event_bubbling -->
<!-- Fetched: 2026-09-14 -->

# Event Bubbling 

Demo

Key pressed: 

KEY  
ELSE CM OM FETCH SET EXEC TEST  
ALARM 3 2 1 ENTER CLEAR

## HTML #

```html
<div id="demo">
  Key pressed: <span data-text="$key"></span>
  <div id="event-bubbling-container" data-on:click="$key = evt.target.closest('button[data-id]')?.dataset.id ?? $key">
    <button data-id="KEY ELSE" class="gray">KEY<br/>ELSE</button>
    <button data-id="CM">CM</button>
    <button data-id="OM">OM</button>
    <button data-id="FETCH">FETCH</button>
    <button data-id="SET">SET</button>
    <button data-id="EXEC">EXEC</button>
    <button data-id="TEST ALARM" class="gray">TEST<br/>ALARM</button>
    <button data-id="3">3</button>
    <button data-id="2">2</button>
    <button data-id="1">1</button>
    <button data-id="ENTER">ENTER</button>
    <button data-id="CLEAR">CLEAR</button>
  </div>
</div>

<style>
  #event-bubbling-container {
    pointer-events: none;

    button {
      user-select: none;

      * {
        pointer-events: none;
        user-select: none;
      }
    }
  }
</style>
```

## Explanation #

This example shows how [event bubbling](https://developer.mozilla.org/en-US/docs/Learn_web_development/Core/Scripting/Event_bubbling) can be leveraged using Datastar. A `data-on:click` attribute on the parent container of the buttons. The listener is on the container, but `evt.target` can be a nested element inside a button, so we resolve the nearest matching button first with `evt.target.closest('button[data-id]')?.dataset.id`. This allows us to handle all button clicks with a single event listener.

Note the `pointer-events: none;` style on the button container to prevent container clicks, and `pointer-events: none;` on button contents (plus `user-select: none;`) so nested elements like `<br>` don’t become the click target.

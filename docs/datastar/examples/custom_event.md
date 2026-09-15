<!-- Source: https://data-star.dev/examples/custom_event -->
<!-- Fetched: 2026-09-14 -->

# Custom Event 

Demo

## HTML #

```html
<p
    id="foo"
    data-signals:_event-details
    data-on:myevent="$_eventDetails = evt.detail"
    data-text="`Last Event Details: ${$_eventDetails}`"
></p>
<script>
    const foo = document.getElementById("foo");
    setInterval(() => {
        foo.dispatchEvent(
            new CustomEvent("myevent", {
                detail: JSON.stringify({
                    eventTime: new Date().toLocaleTimeString(),
                }),
            })
        );
    }, 1000);
</script>
```

## Explanation #

The `data-on` attribute can listen to any event, including custom events. In this example, we are listening to a custom event myevent on the foo element. When the event is triggered, the `$_eventDetails` signal is set to the event’s details.

This is primarily used when interacting with Web Components or other custom elements that emit custom events.

### Note #

There is an extra variable `evt` available in the event handler that contains the event object. This is used to access the event details like `evt.detail` in this example.

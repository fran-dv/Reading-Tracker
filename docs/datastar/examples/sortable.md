<!-- Source: https://data-star.dev/examples/sortable -->
<!-- Fetched: 2026-09-14 -->

# Sortable 

Demo

Item 1 Item 2 Item 3 Item 4 Item 5

## Explanation #

Datastar allows you to listen for custom events using `data-on` and react to them by modifying signals.

```html
<div data-signals:order-info="'Initial order'" data-text="$orderInfo"></div>
<div id="sortContainer" data-on:reordered="$orderInfo = event.detail.orderInfo">
    <button>Item 1</button>
    <button>Item 2</button>
    <button>Item 3</button>
    <button>Item 4</button>
    <button>Item 5</button>
</div>

<script type="module">
    import Sortable from 'https://cdn.jsdelivr.net/npm/sortablejs/+esm'
    new Sortable(sortContainer, {
        animation: 150,
        ghostClass: 'opacity-25',
        onEnd: (evt) => {
            sortContainer.dispatchEvent(
                new CustomEvent('reordered', {detail: {
                    orderInfo: `Moved from position ${evt.oldIndex + 1} to ${evt.newIndex + 1}`
                }})
            )
        }
    })
</script>
```

We create an `orderInfo` signal and modify it whenever a `reordered` event is triggered.

We instruct the [SortableJS](https://sortablejs.github.io/Sortable/) library to dispatch a custom event `reordered` whenever the sortable list is changed. This event contains the order information that we can use to update the `orderInfo` signal.

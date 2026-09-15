<!-- Source: https://data-star.dev/examples/lazy_tabs -->
<!-- Fetched: 2026-09-14 -->

# Lazy Tabs 

Demo

Tab 0Tab 1Tab 2Tab 3Tab 4Tab 5Tab 6Tab 7

Rerum exercitationem quasi quis et aspernatur. Vel ipsa sapiente provident et at. Adipisci ab eligendi cumque assumenda ad. Voluptate explicabo ratione tenetur quo vitae. Aperiam dignissimos mollitia illum velit rerum. Deserunt aut ut incidunt cupiditate alias. Ipsam provident soluta dolorem placeat quod. Quidem sed voluptate nam iusto doloribus.

## HTML #

```html
<div id="demo">
    <div role="tablist">
        <button
            role="tab"
            aria-selected="true"
            data-on:click="@get('/examples/lazy_tabs/0')"
        >
            Tab 0
        </button>
        <button
            role="tab"
            aria-selected="false"
            data-on:click="@get('/examples/lazy_tabs/1')"
        >
            Tab 1
        </button>
        <button
            role="tab"
            aria-selected="false"
            data-on:click="@get('/examples/lazy_tabs/2')"
        >
            Tab 2
        </button>
        <!-- More tabs... -->
    </div>
    <div role="tabpanel">
        <p>Lorem ipsum dolor sit amet...</p>
        <p>Consectetur adipiscing elit...</p>
        <!-- Tab content -->
    </div>
</div>
```

## Explanation #

This example shows how easy it is to implement tabs using Datastar. Following the principles of Hypertext As The Engine Of Application State, the selected tab is a part of the application state. Therefore, to display and select tabs in your application, simply include the tab markup in the returned HTML fragment.

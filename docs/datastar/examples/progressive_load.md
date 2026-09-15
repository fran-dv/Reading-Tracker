<!-- Source: https://data-star.dev/examples/progressive_load -->
<!-- Fetched: 2026-09-14 -->

# Progressive Load 

Demonstrates how to progressively load different sections of a page using SSE events. 

Demo

Load

Each part is loaded randomly and progressively.

## HTML #

```html
<div>
    <div class="actions">
        <button
            id="load-button"
            data-signals:load-disabled="false"
            data-on:click="$loadDisabled=true; @get('/examples/progressive_load/updates')"
            data-attr:disabled="$loadDisabled"
            data-indicator:progressive-Load
        >
            Load
        </button>
        <!-- Indicator element -->
    </div>
    <p>
        Each part is loaded randomly and progressively.
    </p>
</div>
<div id="Load">
    <header id="header">Welcome to my blog</header>
    <section id="article">
        <h4>This is my article</h4>
        <section id="articleBody">
            <p>
                Lorem ipsum dolor sit amet...
            </p>
        </section>
    </section>
    <section id="comments">
        <h5>Comments</h5>
        <p>
            This is the comments section. It will also be progressively loaded as you scroll down.
        </p>
        <ul id="comments-list">
            <li id="1">
                <img src="https://avatar.iran.liara.run/username?username=example" alt="Avatar" class="avatar"/>
                This is a comment...
            </li>
            <!-- More comments loaded progressively -->
        </ul>
    </section>
    <div id="footer">Hope you like it</div>
</div>
```

## Explanation #

This is a response to [Dan Abramov's article on progressive JSON](https://overreacted.io/progressive-json/). I think it's overcomplicated and shows a lack of understanding of how powerful native hypermedia is.

### Note #

This example shows how to progressively load a page using Datastar. The page is divided into sections. We already have examples of [infinite scroll](https://data-star.dev/examples/infinite_scroll) and [progress bar](https://data-star.dev/examples/progress_bar), but this example shows how to progressively load a page in a more structured way.

It's truly baffling to me the amount of complexity that React developers tend to introduce. Hypermedia is a powerful tool that allows you to progressively load content in a way that is simple and efficient. This example shows how to use Datastar's server-sent events (SSE) to progressively load a page in a way that is easy to understand and maintain.

Nothing is faster than direct HTML morphing without a virtual DOM. – let the browser do the heavy lifting. This example shows how to use Datastar to progressively load a page in a way that is simple and efficient while only using a one-time cost CDN shim.

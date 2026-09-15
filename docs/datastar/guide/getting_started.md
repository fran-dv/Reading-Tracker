<!-- Source: https://data-star.dev/guide/getting_started -->
<!-- Fetched: 2026-09-14 -->

# Getting Started

Datastar simplifies frontend development, allowing you to build backend-driven, interactive UIs using a [hypermedia-first](https://hypermedia.systems/hypermedia-a-reintroduction/) approach that extends and enhances HTML.

Datastar provides backend reactivity like [htmx](https://htmx.org/) and frontend reactivity like [Alpine.js](https://alpinejs.dev/) in a lightweight frontend framework that doesn’t require any npm packages or other dependencies. It provides two primary functions:

  1. Modify the DOM and state by sending events from your backend.
  2. Build reactivity into your frontend using standard `data-*` HTML attributes.

## Installation #

The quickest way to use Datastar is to include it using a `script` tag that fetches it from a CDN.

```html
<script type="module" src="https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.3/bundles/datastar.js"></script>
```

Hosting the file yourself is recommended. Download the [script](https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.3/bundles/datastar.js) or create your own bundle using the [bundler](https://data-star.dev/pro/bundler), then include it from the appropriate path.

```html
<script type="module" src="/path/to/datastar.js"></script>
```

To import Datastar using a package manager such as npm, Deno, or Bun, you can use an import statement.

```javascript
// @ts-expect-error (only required for TypeScript projects)
import 'https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.3/bundles/datastar.js'
```

## `data-*` #

At the core of Datastar are `[data-*](https://developer.mozilla.org/en-US/docs/Web/HTML/How_to/Use_data_attributes)` HTML attributes (hence the name). They allow you to add reactivity to your frontend and interact with your backend in a declarative way.

> The Datastar [VSCode extension](https://marketplace.visualstudio.com/items?itemName=starfederation.datastar-vscode) and [IntelliJ plugin](https://plugins.jetbrains.com/plugin/26072-datastar) provide autocomplete, diagnostics, hover documentation, and syntax highlighting.

The [`data-on`](https://data-star.dev/reference/attributes#data-on) attribute can be used to attach an event listener to an element and execute an expression whenever the event is triggered. The value of the attribute is a [Datastar expression](https://data-star.dev/guide/datastar_expressions) in which JavaScript can be used.

```html
<button data-on:click="alert('I’m sorry, Dave. I’m afraid I can’t do that.')">
    Open the pod bay doors, HAL.
</button>
```

Demo

Open the pod bay doors, HAL. 

We’ll explore more data attributes in the [next section of the guide](https://data-star.dev/guide/reactive_signals).

## Patching Elements #

With Datastar, the backend _drives_ the frontend by **patching** (adding, updating and removing) HTML elements in the DOM.

Datastar receives elements from the backend and manipulates the DOM using a morphing strategy (by default). Morphing ensures that only modified parts of the DOM are updated, and that only data attributes that have changed are [reapplied](https://data-star.dev/reference/attributes#attribute-evaluation-order), preserving state and improving performance.

Datastar provides [actions](https://data-star.dev/reference/actions#backend-actions) for sending requests to the backend. The [`@get()`](https://data-star.dev/reference/actions#get) action sends a `GET` request to the provided URL using a [fetch](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API) request.

```html
<button data-on:click="@get('/endpoint')">
    Open the pod bay doors, HAL.
</button>
<div id="hal"></div>
```

> Actions in Datastar are helper functions that have the syntax `@actionName()`. Read more about actions in the [reference](https://data-star.dev/reference/actions).

If the response has a `content-type` of `text/html`, the top-level HTML elements will be morphed into the existing DOM based on the element IDs. 

```html
<div id="hal">
    I’m sorry, Dave. I’m afraid I can’t do that.
</div>
```

We call this a “Patch Elements” event because multiple elements can be patched into the DOM at once.

Demo

Open the pod bay doors, HAL. `Waiting for an order...`

In the example above, the DOM must contain an element with a `hal` ID in order for morphing to work. Other [patching strategies](https://data-star.dev/reference/sse_events#datastar-patch-elements) are available, but morph is the best and simplest choice in most scenarios.

If the response has a `content-type` of `text/event-stream`, it can contain zero or more [SSE events](https://data-star.dev/reference/sse_events). The example above can be replicated using a `datastar-patch-elements` SSE event. Note that SSE events must be followed by two newline characters.

```text
event: datastar-patch-elements
data: elements <div id="hal">
data: elements     I’m sorry, Dave. I’m afraid I can’t do that.
data: elements </div>
```

Because we can send as many events as we want in a stream, and because it can be a long-lived connection, we can extend the example above to first send HAL’s response and then, after a few seconds, reset the text.

```text
event: datastar-patch-elements
data: elements <div id="hal">
data: elements     I’m sorry, Dave. I’m afraid I can’t do that.
data: elements </div>

event: datastar-patch-elements
data: elements <div id="hal">
data: elements     Waiting for an order...
data: elements </div>
```

Demo

Open the pod bay doors, HAL. `Waiting for an order...`

Here’s the code to generate the SSE events above using the SDKs.

**clojure**

```clojure
;; Import the SDK's api and your adapter
(require
 '[starfederation.datastar.clojure.api :as d*]
 '[starfederation.datastar.clojure.adapter.http-kit :refer [->sse-response on-open]])

;; in a ring handler
(defn handler [request]
  ;; Create an SSE response
  (->sse-response request
                  {on-open
                   (fn [sse]
                     ;; Patches elements into the DOM
                     (d*/patch-elements! sse
                                         "<div id=\"hal\">I’m sorry, Dave. I’m afraid I can’t do that.</div>")
                     (Thread/sleep 1000)
                     (d*/patch-elements! sse
                                         "<div id=\"hal\">Waiting for an order...</div>"))}))
```

**csharp**

```csharp
using StarFederation.Datastar.DependencyInjection;

// Adds Datastar as a service
builder.Services.AddDatastar();

app.MapGet("/", async (IDatastarService datastarService) =>
{
    // Patches elements into the DOM.
    await datastarService.PatchElementsAsync(@"<div id=""hal"">I’m sorry, Dave. I’m afraid I can’t do that.</div>");

    await Task.Delay(TimeSpan.FromSeconds(1));

    await datastarService.PatchElementsAsync(@"<div id=""hal"">Waiting for an order...</div>");
});
```

**go**

```go
import (
    "github.com/starfederation/datastar-go/datastar"
    time
)

// Creates a new `ServerSentEventGenerator` instance.
sse := datastar.NewSSE(w,r)

// Patches elements into the DOM.
sse.PatchElements(
    `<div id="hal">I’m sorry, Dave. I’m afraid I can’t do that.</div>`
)

time.Sleep(1 * time.Second)

sse.PatchElements(
    `<div id="hal">Waiting for an order...</div>`
)
```

**java**

```java
import starfederation.datastar.utils.ServerSentEventGenerator;

// Creates a new `ServerSentEventGenerator` instance.
AbstractResponseAdapter responseAdapter = new HttpServletResponseAdapter(response);
ServerSentEventGenerator generator = new ServerSentEventGenerator(responseAdapter);

// Patches elements into the DOM.
generator.send(PatchElements.builder()
    .data("<div id=\"hal\">I’m sorry, Dave. I’m afraid I can’t do that.</div>")
    .build()
);

Thread.sleep(1000);

generator.send(PatchElements.builder()
    .data("<div id=\"hal\">Waiting for an order...</div>")
    .build()
);
```

**kotlin**

```kotlin
val generator = ServerSentEventGenerator(response)

generator.patchElements(
    elements = """<div id="hal">I’m sorry, Dave. I’m afraid I can’t do that.</div>""",
)

Thread.sleep(ONE_SECOND)

generator.patchElements(
    elements = """<div id="hal">Waiting for an order...</div>""",
)
```

**php**

```php
use starfederation\datastar\ServerSentEventGenerator;

// Creates a new `ServerSentEventGenerator` instance.
$sse = new ServerSentEventGenerator();

// Patches elements into the DOM.
$sse->patchElements(
    '<div id="hal">I’m sorry, Dave. I’m afraid I can’t do that.</div>'
);

sleep(1);

$sse->patchElements(
    '<div id="hal">Waiting for an order...</div>'
);
```

**python**

```python
from datastar_py import ServerSentEventGenerator as SSE
from datastar_py.sanic import datastar_response

@app.get('/open-the-bay-doors')
@datastar_response
async def open_doors(request):
    yield SSE.patch_elements('<div id="hal">I’m sorry, Dave. I’m afraid I can’t do that.</div>')
    await asyncio.sleep(1)
    yield SSE.patch_elements('<div id="hal">Waiting for an order...</div>')
```

**ruby**

```ruby
require 'datastar'

# Create a Datastar::Dispatcher instance

datastar = Datastar.new(request:, response:)

# In a Rack handler, you can instantiate from the Rack env
# datastar = Datastar.from_rack_env(env)

# Start a streaming response
datastar.stream do |sse|
  # Patches elements into the DOM.
  sse.patch_elements %(<div id="hal">I’m sorry, Dave. I’m afraid I can’t do that.</div>)

  sleep 1
  
  sse.patch_elements %(<div id="hal">Waiting for an order...</div>)
end
```

**rust**

```rust
use async_stream::stream;
use datastar::prelude::*;
use std::thread;
use std::time::Duration;

Sse(stream! {
    // Patches elements into the DOM.
    yield PatchElements::new("<div id='hal'>I’m sorry, Dave. I’m afraid I can’t do that.</div>").into();

    thread::sleep(Duration::from_secs(1));
    
    yield PatchElements::new("<div id='hal'>Waiting for an order...</div>").into();
})
```

**typescript**

```typescript
// Creates a new `ServerSentEventGenerator` instance (this also sends required headers)
ServerSentEventGenerator.stream(req, res, (stream) => {
    // Patches elements into the DOM.
    stream.patchElements(`<div id="hal">I’m sorry, Dave. I’m afraid I can’t do that.</div>`);

    setTimeout(() => {
        stream.patchElements(`<div id="hal">Waiting for an order...</div>`);
    }, 1000);
});
```

**zig**

```zig
const datastar = @import("datastar");

// Example of using patchElements
fn openTheDoorsHal(http: *datastar.HTTPRequest) !void {
    // Use NewSSESync here for synchronous writes over the SSE connection
    var sse = try http.NewSSESync();
    defer sse.close();

    try sse.patchElements(
        \\<div id="hal">I’m sorry, Dave. I’m afraid I can’t do that.</div>
        , .{});

    http.io.sleep(.fromSeconds(1), .real) catch {};

    try sse.patchElements(
        \\<div id="hal">Waiting for an order...</div>
        , .{});
}
```

> In addition to your browser’s dev tools, the [Datastar Inspector](https://data-star.dev/pro#datastar-inspector) can be used to monitor and inspect SSE events received by Datastar.

We’ll cover event streams and [SSE events](https://data-star.dev/reference/sse_events) in more detail [later in the guide](https://data-star.dev/guide/backend_requests), but as you can see, they are just plain text events with a special syntax, made simpler by the [SDKs](https://data-star.dev/reference/sdks).

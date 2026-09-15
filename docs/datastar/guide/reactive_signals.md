<!-- Source: https://data-star.dev/guide/reactive_signals -->
<!-- Fetched: 2026-09-14 -->

# Reactive Signals

In a hypermedia approach, the backend drives state to the frontend and acts as the primary source of truth. It’s up to the backend to determine what actions the user can take next by patching appropriate elements in the DOM.

Sometimes, however, you may need access to frontend state that’s driven by user interactions. Click, input and keydown events are some of the more common user events that you’ll want your frontend to be able to react to.

Datastar uses _signals_ to manage frontend state. You can think of signals as reactive variables that automatically track and propagate changes in and to [Datastar expressions](https://data-star.dev/guide/datastar_expressions). Signals are denoted using the `$` prefix.

## Data Attributes #

Datastar allows you to add reactivity to your frontend and interact with your backend in a declarative way using [custom `data-*` attributes](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Global_attributes/data-*).

> The Datastar [VSCode extension](https://marketplace.visualstudio.com/items?itemName=starfederation.datastar-vscode) and [IntelliJ plugin](https://plugins.jetbrains.com/plugin/26072-datastar) provide autocomplete, diagnostics, hover documentation, and syntax highlighting.

### `data-bind` #

The [`data-bind`](https://data-star.dev/reference/attributes#data-bind) attribute sets up two-way data binding on any HTML element that receives user input or selections. These include `input`, `textarea`, `select`, `checkbox` and `radio` elements, as well as web components whose value can be made reactive.

```html
<input data-bind:foo />
```

This creates a new signal that can be called using `$foo`, and binds it to the element’s value. If either is changed, the other automatically updates.

You can accomplish the same thing passing the signal name as a _value_. This syntax can be more convenient to use with some templating languages.

```html
<input data-bind="foo" />
```

According to the [HTML spec](https://developer.mozilla.org/en-US/docs/Web/HTML/Global_attributes/data-*), all [`data-*`](https://developer.mozilla.org/en-US/docs/Web/HTML/How_to/Use_data_attributes) attributes are case-insensitive. When Datastar processes these attributes, hyphenated names are automatically converted to camel case by removing hyphens and uppercasing the letter following each hyphen. For example, `data-bind:foo-bar` creates a signal named `$fooBar`.

```html
<!-- Both of these create the signal `$fooBar` -->
<input data-bind:foo-bar />
<input data-bind="fooBar" />
```

Read more about [attribute casing](https://data-star.dev/reference/attributes#attribute-casing) in the reference.

### `data-text` #

The [`data-text`](https://data-star.dev/reference/attributes#data-text) attribute sets the text content of an element to the value of a signal. The `$` prefix is required to denote a signal.

```html
<input data-bind:foo-bar />
<div data-text="$fooBar"></div>
```

Demo

```

```

The value of the `data-text` attribute is a [Datastar expression](https://data-star.dev/guide/datastar_expressions) that is evaluated, meaning that we can use JavaScript in it.

```html
<input data-bind:foo-bar />
<div data-text="$fooBar.toUpperCase()"></div>
```

Demo

```

```

### `data-computed` #

The [`data-computed`](https://data-star.dev/reference/attributes#data-computed) attribute creates a new signal that is derived from a reactive expression. The computed signal is read-only, and its value is automatically updated when any signals in the expression are updated.

```html
<input data-bind:foo-bar />
<div data-computed:repeated="$fooBar.repeat(2)" data-text="$repeated"></div>
```

This results in the `$repeated` signal’s value always being equal to the value of the `$fooBar` signal repeated twice. Computed signals are useful for memoizing expressions containing other signals.

Demo

```

```

### `data-show` #

The [`data-show`](https://data-star.dev/reference/attributes#data-show) attribute can be used to show or hide an element based on whether an expression evaluates to `true` or `false`.

```html
<input data-bind:foo-bar />
<button data-show="$fooBar != ''">
    Save
</button>
```

This results in the button being visible only when the input value is _not_ an empty string. This could also be shortened to `data-show="$fooBar"`.

Demo

Save

Since the button is visible until Datastar processes the `data-show` attribute, it’s a good idea to set its initial style to `display: none` to prevent a flash of unwanted content.

```html
<input data-bind:foo-bar />
<button data-show="$fooBar != ''" style="display: none">
    Save
</button>
```

### `data-class` #

The [`data-class`](https://data-star.dev/reference/attributes#data-class) attribute allows us to add or remove an element’s class based on an expression.

```html
<input data-bind:foo-bar />
<button data-class:success="$fooBar != ''">
    Save
</button>
```

If the expression evaluates to `true`, the `success` class is added to the element, otherwise it is removed.

Demo

Save

Unlike the `data-bind` attribute, in which hyphenated names are converted to camel case, the `data-class` attribute converts the class name to kebab case. For example, `data-class:font-bold` adds or removes the `font-bold` class.

```html
<button data-class:font-bold="$fooBar == 'strong'">
    Save
</button>
```

The `data-class` attribute can also be used to add or remove multiple classes from an element using a set of key-value pairs, where the keys represent class names and the values represent expressions.

```html
<button data-class="{success: $fooBar != '', 'font-bold': $fooBar == 'strong'}">
    Save
</button>
```

Note how the `font-bold` key must be wrapped in quotes because it contains a hyphen.

### `data-attr` #

The [`data-attr`](https://data-star.dev/reference/attributes#data-attr) attribute can be used to bind the value of any HTML attribute to an expression.

```html
<input data-bind:foo />
<button data-attr:disabled="$foo == ''">
    Save
</button>
```

This results in a `disabled` attribute being given the value `true` whenever the input is an empty string.

Demo

Save

The `data-attr` attribute also converts the attribute name to kebab case, since HTML attributes are typically written in kebab case. For example, `data-attr:aria-hidden` sets the value of the `aria-hidden` attribute.

```html
<button data-attr:aria-hidden="$foo">Save</button>
```

The `data-attr` attribute can also be used to set the values of multiple attributes on an element using a set of key-value pairs, where the keys represent attribute names and the values represent expressions.

```html
<button data-attr="{disabled: $foo == '', 'aria-hidden': $foo}">Save</button>
```

Note how the `aria-hidden` key must be wrapped in quotes because it contains a hyphen.

### `data-signals` #

Signals are globally accessible from anywhere in the DOM. So far, we’ve created signals on the fly using `data-bind` and `data-computed`. If a signal is used without having been created, it will be created automatically and its value set to an empty string.

Another way to create signals is using the [`data-signals`](https://data-star.dev/reference/attributes#data-signals) attribute, which patches (adds, updates or removes) one or more signals into the existing signals.

```html
<div data-signals:foo-bar="1"></div>
```

Signals can be nested using dot-notation.

```html
<div data-signals:form.baz="2"></div>
```

Like the `data-bind` attribute, hyphenated names used with `data-signals` are automatically converted to camel case by removing hyphens and uppercasing the letter following each hyphen.

```html
<div data-signals:foo-bar="1"
     data-text="$fooBar"
></div>
```

The `data-signals` attribute can also be used to patch multiple signals using a set of key-value pairs, where the keys represent signal names and the values represent expressions. Nested signals can be created using nested objects.

```html
<div data-signals="{fooBar: 1, form: {baz: 2}}"></div>
```

### `data-on` #

The [`data-on`](https://data-star.dev/reference/attributes#data-on) attribute can be used to attach an event listener to an element and run an expression whenever the event is triggered.

```html
<input data-bind:foo />
<button data-on:click="$foo = ''">
    Reset
</button>
```

This results in the `$foo` signal’s value being set to an empty string whenever the button element is clicked. This can be used with any valid event name such as `data-on:keydown`, `data-on:mouseover`, etc. 

Demo

Reset

Custom events can also be used. Like the `data-class` attribute, the `data-on` attribute converts the event name to kebab case. For example, `data-on:custom-event` listens for the `custom-event` event.

```html
<div data-on:my-event="$foo = ''">
    <input data-bind:foo />
</div>
```

These are just _some_ of the attributes available in Datastar. For a complete list, see the [attribute reference](https://data-star.dev/reference/attributes).

## Frontend Reactivity #

Datastar’s data attributes enable declarative signals and expressions, providing a simple yet powerful way to add reactivity to the frontend.

Datastar expressions are strings that are evaluated by Datastar [attributes](https://data-star.dev/reference/attributes) and [actions](https://data-star.dev/reference/actions). While they are similar to JavaScript, there are some important differences that are explained in the [next section of the guide](https://data-star.dev/guide/datastar_expressions).

```html
<div data-signals:hal="'...'">
    <button data-on:click="$hal = 'Affirmative, Dave. I read you.'">
        HAL, do you read me?
    </button>
    <div data-text="$hal"></div>
</div>
```

Demo

HAL, do you read me? ```

```

See if you can figure out what the code below does based on what you’ve learned so far, _before_ trying the demo below it.

```html
<div
    data-signals="{response: '', answer: 'bread'}"
    data-computed:correct="$response.toLowerCase() == $answer"
>
    <div id="question">What do you put in a toaster?</div>
    <button data-on:click="$response = prompt('Answer:') ?? ''">BUZZ</button>
    <div data-show="$response != ''">
        You answered “<span data-text="$response"></span>”.
        <span data-show="$correct">That is correct ✅</span>
        <span data-show="!$correct">
        The correct answer is “
        <span data-text="$answer"></span>
        ” 🤷
        </span>
    </div>
</div>
```

Demo

What do you put in a toaster?

BUZZ

You answered “”. That is correct ✅ The correct answer is “bread” 🤷

> The [Datastar Inspector](https://data-star.dev/pro#datastar-inspector) can be used to inspect and filter current signals and view signal patch events in real-time.

## Patching Signals #

Remember that in a hypermedia approach, the backend drives state to the frontend. Just like with elements, frontend signals can be **patched** (added, updated and removed) from the backend using [backend actions](https://data-star.dev/reference/actions#backend-actions).

```html
<div data-signals:hal="'...'">
    <button data-on:click="@get('/endpoint')">
        HAL, do you read me?
    </button>
    <div data-text="$hal"></div>
</div>
```

If a response has a `content-type` of `application/json`, the signal values are patched into the frontend signals.

We call this a “Patch Signals” event because multiple signals can be patched (using [JSON Merge Patch RFC 7396](https://datatracker.ietf.org/doc/rfc7396/)) into the existing signals.

```
{"hal": "Affirmative, Dave. I read you."}
```

Demo

HAL, do you read me? ``

Reset 

If the response has a `content-type` of `text/event-stream`, it can contain zero or more [SSE events](https://data-star.dev/reference/sse_events). The example above can be replicated using a `datastar-patch-signals` SSE event.

```text
event: datastar-patch-signals
data: signals {hal: 'Affirmative, Dave. I read you.'}
```

Because we can send as many events as we want in a stream, and because it can be a long-lived connection, we can extend the example above to first set the `hal` signal to an “affirmative” response and then, after a second, reset the signal.

```text
event: datastar-patch-signals
data: signals {hal: 'Affirmative, Dave. I read you.'}

// Wait 1 second

event: datastar-patch-signals
data: signals {hal: '...'}
```

Demo

HAL, do you read me?  ``

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
                     ;; Patches signal.
                     (d*/patch-signals! sse "{hal: 'Affirmative, Dave. I read you.'}")
                     (Thread/sleep 1000)
                     (d*/patch-signals! sse "{hal: '...'}"))}))
```

**csharp**

```csharp
using StarFederation.Datastar.DependencyInjection;

// Adds Datastar as a service
builder.Services.AddDatastar();

app.MapGet("/hal", async (IDatastarService datastarService) =>
{
    // Patches signals.
    await datastarService.PatchSignalsAsync(new { hal = "Affirmative, Dave. I read you" });

    await Task.Delay(TimeSpan.FromSeconds(3));

    await datastarService.PatchSignalsAsync(new { hal = "..." });
});
```

**go**

```go
import (
    "github.com/starfederation/datastar-go/datastar"
)

// Creates a new `ServerSentEventGenerator` instance.
sse := datastar.NewSSE(w, r)

// Patches signals
sse.PatchSignals([]byte(`{hal: 'Affirmative, Dave. I read you.'}`))

time.Sleep(1 * time.Second)

sse.PatchSignals([]byte(`{hal: '...'}`))
```

**java**

```java
import starfederation.datastar.utils.ServerSentEventGenerator;

// Creates a new `ServerSentEventGenerator` instance.
AbstractResponseAdapter responseAdapter = new HttpServletResponseAdapter(response);
ServerSentEventGenerator generator = new ServerSentEventGenerator(responseAdapter);

// Patches signals.
generator.send(PatchSignals.builder()
    .data("{\"hal\": \"Affirmative, Dave. I read you.\"}")
    .build()
);

Thread.sleep(1000);

generator.send(PatchSignals.builder()
    .data("{\"hal\": \"...\"}")
    .build()
);
```

**kotlin**

```kotlin
val generator = ServerSentEventGenerator(response)

generator.patchSignals(
    signals = """{"hal": "Affirmative, Dave. I read you."}""",
)

Thread.sleep(ONE_SECOND)

generator.patchSignals(
    signals = """{"hal": "..."}""",
)
```

**php**

```php
use starfederation\datastar\ServerSentEventGenerator;

// Creates a new `ServerSentEventGenerator` instance.
$sse = new ServerSentEventGenerator();

// Patches signals.
$sse->patchSignals(['hal' => 'Affirmative, Dave. I read you.']);

sleep(1);

$sse->patchSignals(['hal' => '...']);
```

**python**

```python
from datastar_py import ServerSentEventGenerator as SSE
from datastar_py.sanic import datastar_response

@app.get('/do-you-read-me')
@datastar_response
async def open_doors(request):
    yield SSE.patch_signals({"hal": "Affirmative, Dave. I read you."})
    await asyncio.sleep(1)
    yield SSE.patch_signals({"hal": "..."})
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
  # Patches signals
  sse.patch_signals(hal: 'Affirmative, Dave. I read you.')

  sleep 1
  
  sse.patch_signals(hal: '...')
end
```

**rust**

```rust
use async_stream::stream;
use datastar::prelude::*;
use std::thread;
use std::time::Duration;

Sse(stream! {
    // Patches signals.
    yield PatchSignals::new("{hal: 'Affirmative, Dave. I read you.'}").into();

    thread::sleep(Duration::from_secs(1));
    
    yield PatchSignals::new("{hal: '...'}").into();
})
```

**typescript**

```typescript
// Creates a new `ServerSentEventGenerator` instance (this also sends required headers)
ServerSentEventGenerator.stream(req, res, (stream) => {
    // Patches signals.
    stream.patchSignals({'hal': 'Affirmative, Dave. I read you.'});

    setTimeout(() => {
        stream.patchSignals({'hal': '...'});
    }, 1000);
});
```

**zig**

```zig
const datastar = @import("datastar");

// Example of using patchSignals
fn doYouReadMeHAL(http: *datastar.HTTPRequest) !void {
    var sse = try http.NewSSESync();
    defer sse.close();

    try sse.patchSignals(.{
        .hal = "Affirmative, Dave. I read you.",
    }, .{});

    http.io.sleep(.fromSeconds(1), .real) catch {};

    try sse.patchSignals(.{
        .hal = "...",
    }, .{});
}
```

> In addition to your browser’s dev tools, the [Datastar Inspector](https://data-star.dev/pro#datastar-inspector) can be used to monitor and inspect SSE events received by Datastar.

We’ll cover event streams and [SSE events](https://data-star.dev/reference/sse_events) in more detail [later in the guide](https://data-star.dev/guide/backend_requests), but as you can see, they are just plain text events with a special syntax, made simpler by the [SDKs](https://data-star.dev/reference/sdks).

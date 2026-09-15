<!-- Source: https://data-star.dev/guide/backend_requests -->
<!-- Fetched: 2026-09-14 -->

# Backend Requests

Between [attributes](https://data-star.dev/reference/attributes) and [actions](https://data-star.dev/reference/actions), Datastar provides you with everything you need to build hypermedia-driven applications. Using this approach, the backend drives state to the frontend and acts as the single source of truth, determining what actions the user can take next.

## Sending Signals #

By default, all signals (except for local signals whose keys begin with an underscore) are sent in an object with every backend request. When using a `GET` request, the signals are sent as a `datastar` query parameter, otherwise they are sent as a JSON body.

By sending **all** signals in every request, the backend has full access to the frontend state. This is by design. It is **not** recommended to send partial signals, but if you must, you can use the [`filterSignals`](https://data-star.dev/reference/actions#filterSignals) option to filter the signals sent to the backend.

### Nesting Signals #

Signals can be nested, making it easier to target signals in a more granular way on the backend.

Using dot-notation:

```html
<div data-signals:foo.bar="1"></div>
```

Using object syntax:

```html
<div data-signals="{foo: {bar: 1}}"></div>
```

Using two-way binding:

```html
<input data-bind:foo.bar />
```

A practical use-case of nested signals is when you have repetition of state on a page. The following example tracks the open/closed state of a menu on both desktop and mobile devices, and the [toggleAll()](https://data-star.dev/reference/actions#toggleAll) action to toggle the state of all menus at once.

```html
<div data-signals="{menu: {isOpen: {desktop: false, mobile: false}}}">
    <button data-on:click="@toggleAll({include: /^menu\.isOpen\./})">
        Open/close menu
    </button>
</div>
```

## Reading Signals #

To read signals from the backend, JSON decode the `datastar` query param for `GET` requests, and the request body for all other methods.

All [SDKs](https://data-star.dev/reference/sdks) provide a helper function to read signals. Here’s how you would read the nested signal `foo.bar` from an incoming request.

No example found for Clojure

**csharp**

```csharp
using StarFederation.Datastar.DependencyInjection;

// Adds Datastar as a service
builder.Services.AddDatastar();

public record Signals
{
    [JsonPropertyName("foo")] [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public FooSignals? Foo { get; set; } = null;

    public record FooSignals
    {
        [JsonPropertyName("bar")] [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
        public string? Bar { get; set; }
    }
}

app.MapGet("/read-signals", async (IDatastarService datastarService) =>
{
    Signals? mySignals = await datastarService.ReadSignalsAsync<Signals>();
    var bar = mySignals?.Foo?.Bar;
});
```

**go**

```go
import ("github.com/starfederation/datastar-go/datastar")

type Signals struct {
    Foo struct {
        Bar string `json:"bar"`
    } `json:"foo"`
}

signals := &Signals{}
if err := datastar.ReadSignals(request, signals); err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
}
```

No example found for Java

**kotlin**

```kotlin
@Serializable
data class Signals(
    val foo: String,
)

val jsonUnmarshaller: JsonUnmarshaller<Signals> = { json -> Json.decodeFromString(json) }

val request: Request =
    postRequest(
        body =
            """
            {
                "foo": "bar"
            }
            """.trimIndent(),
    )

val signals = readSignals(request, jsonUnmarshaller)
```

**php**

```php
use starfederation\datastar\ServerSentEventGenerator;

// Reads all signals from the request.
$signals = ServerSentEventGenerator::readSignals();
```

**python**

```python
from datastar_py.fastapi import datastar_response, read_signals

@app.get("/updates")
@datastar_response
async def updates(request: Request):
    # Retrieve a dictionary with the current state of the signals from the frontend
    signals = await read_signals(request)
```

**ruby**

```ruby
# Setup with request
datastar = Datastar.new(request:, response:)

# Read signals
some_signal = datastar.signals[:some_signal]
```

No example found for Rust

No example found for TypeScript

**zig**

```zig
const datastar = @import("datastar");

pub const Signals = struct {
    foo: struct {
        bar: []const u8,
    },
};

pub fn handleIncomingRequest(http: *datastar.HTTPRequest) !void {
    const signals = try datastar.readSignals(Signals);
    // ... use signals
}
```

## SSE Events #

Datastar can stream zero or more [Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) (SSE) from the web server to the browser. There’s no special backend plumbing required to use SSE, just some special syntax. Fortunately, SSE is straightforward and [provides us with some advantages](https://data-star.dev/essays/event_streams_all_the_way_down), in addition to allowing us to send multiple events in a single response (in contrast to sending `text/html` or `application/json` responses).

First, set up your backend in the language of your choice. Familiarize yourself with [sending SSE events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#sending_events_from_the_server), or use one of the backend [SDKs](https://data-star.dev/reference/sdks) to get up and running even faster. We’re going to use the SDKs in the examples below, which set the appropriate headers and format the events for us.

The following code would exist in a controller action endpoint in your backend.

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
                                         "<div id=\"question\">What do you put in a toaster?</div>")

                     ;; Patches signals
                     (d*/patch-signals! sse "{response: '', answer: 'bread'}"))}))
```

**csharp**

```csharp
using StarFederation.Datastar.DependencyInjection;

// Adds Datastar as a service
builder.Services.AddDatastar();

app.MapGet("/", async (IDatastarService datastarService) =>
{
    // Patches elements into the DOM.
    await datastarService.PatchElementsAsync(@"<div id=""question"">What do you put in a toaster?</div>");

    // Patches signals.
    await datastarService.PatchSignalsAsync(new { response = "", answer = "bread" });
});
```

**go**

```go
import ("github.com/starfederation/datastar-go/datastar")

// Creates a new `ServerSentEventGenerator` instance.
sse := datastar.NewSSE(w,r)

// Patches elements into the DOM.
sse.PatchElements(
    `<div id="question">What do you put in a toaster?</div>`
)

// Patches signals.
sse.PatchSignals([]byte(`{response: '', answer: 'bread'}`))
```

**java**

```java
import starfederation.datastar.utils.ServerSentEventGenerator;

// Creates a new `ServerSentEventGenerator` instance.
AbstractResponseAdapter responseAdapter = new HttpServletResponseAdapter(response);
ServerSentEventGenerator generator = new ServerSentEventGenerator(responseAdapter);

// Patches elements into the DOM.
generator.send(PatchElements.builder()
    .data("<div id=\"question\">What do you put in a toaster?</div>")
    .build()
);

// Patches signals.
generator.send(PatchSignals.builder()
    .data("{\"response\": \"\", \"answer\": \"\"}")
    .build()
);
```

**kotlin**

```kotlin
val generator = ServerSentEventGenerator(response)

generator.patchElements(
    elements = """<div id="question">What do you put in a toaster?</div>""",
)

generator.patchSignals(
    signals = """{"response": "", "answer": "bread"}""",
)
```

**php**

```php
use starfederation\datastar\ServerSentEventGenerator;

// Creates a new `ServerSentEventGenerator` instance.
$sse = new ServerSentEventGenerator();

// Patches elements into the DOM.
$sse->patchElements(
    '<div id="question">What do you put in a toaster?</div>'
);

// Patches signals.
$sse->patchSignals(['response' => '', 'answer' => 'bread']);
```

**python**

```python
from datastar_py import ServerSentEventGenerator as SSE
from datastar_py.litestar import DatastarResponse

async def endpoint():
    return DatastarResponse([
        SSE.patch_elements('<div id="question">What do you put in a toaster?</div>'),
        SSE.patch_signals({"response": "", "answer": "bread"})
    ])
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
  # Patches elements into the DOM
  sse.patch_elements %(<div id="question">What do you put in a toaster?</div>)

  # Patches signals
  sse.patch_signals(response: '', answer: 'bread')
end
```

**rust**

```rust
use datastar::prelude::*;
use async_stream::stream;

Sse(stream! {
    // Patches elements into the DOM.
    yield PatchElements::new("<div id='question'>What do you put in a toaster?</div>").into();

    // Patches signals.
    yield PatchSignals::new("{response: '', answer: 'bread'}").into();
})
```

**typescript**

```typescript
// Creates a new `ServerSentEventGenerator` instance (this also sends required headers)
ServerSentEventGenerator.stream(req, res, (stream) => {
      // Patches elements into the DOM.
     stream.patchElements(`<div id="question">What do you put in a toaster?</div>`);

     // Patches signals.
     stream.patchSignals({'response':  '', 'answer': 'bread'});
});
```

**zig**

```zig
const datastar = @import("datastar");

var sse = http.NewSSE();
defer sse.close();

// Patches elements into the DOM.
try sse.PatchElements(
    \\<div id="question">What do you put in a toaster?</div>
    , .{});
)

// Patches signals.
try sse.patchSignals(.{
    .response: "",
    .answer: "bread"
}, .{});
```

The `PatchElements()` function updates the provided HTML element into the DOM, replacing the element with `id="question"`. An element with the ID `question` must _already_ exist in the DOM.

The `PatchSignals()` function updates the `response` and `answer` signals into the frontend signals.

With our backend in place, we can now use the `data-on:click` attribute to trigger the [`@get()`](https://data-star.dev/reference/actions#get) action, which sends a `GET` request to the `/actions/quiz` endpoint on the server when a button is clicked.

```html
<div
    data-signals="{response: '', answer: ''}"
    data-computed:correct="$response.toLowerCase() == $answer"
>
    <div id="question"></div>
    <button data-on:click="@get('/actions/quiz')">Fetch a question</button>
    <button
        data-show="$answer != ''"
        data-on:click="$response = prompt('Answer:') ?? ''"
    >
        BUZZ
    </button>
    <div data-show="$response != ''">
        You answered “<span data-text="$response"></span>”.
        <span data-show="$correct">That is correct ✅</span>
        <span data-show="!$correct">
        The correct answer is “<span data-text="$answer"></span>” 🤷
        </span>
    </div>
</div>
```

Now when the `Fetch a question` button is clicked, the server will respond with an event to modify the `question` element in the DOM and an event to modify the `response` and `answer` signals. We’re driving state from the backend!

Demo

...

Fetch a question BUZZ

You answered “”. That is correct ✅ The correct answer is “” 🤷

### `data-indicator` #

The [`data-indicator`](https://data-star.dev/reference/attributes#data-indicator) attribute sets the value of a signal to `true` while the request is in flight, otherwise `false`. We can use this signal to show a loading indicator, which may be desirable for slower responses.

```html
<div id="question"></div>
<button
    data-on:click="@get('/actions/quiz')"
    data-indicator:fetching
>
    Fetch a question
</button>
<div data-class:loading="$fetching" class="indicator"></div>
```

Demo

...

Fetch a question

## Backend Actions #

We’re not limited to sending just `GET` requests. Datastar provides [backend actions](https://data-star.dev/reference/actions#backend-actions) for each of the methods available: `@get()`, `@post()`, `@put()`, `@patch()` and `@delete()`.

Here’s how we can send an answer to the server for processing, using a `POST` request.

```html
<button data-on:click="@post('/actions/quiz')">
    Submit answer
</button>
```

One of the benefits of using SSE is that we can send multiple events (patch elements and patch signals) in a single response.

**clojure**

```clojure
(d*/patch-elements! sse "<div id=\"question\">...</div>")
(d*/patch-elements! sse "<div id=\"instructions\">...</div>")
(d*/patch-signals! sse "{answer: '...', prize: '...'}")
```

**csharp**

```csharp
datastarService.PatchElementsAsync(@"<div id=""question"">...</div>");
datastarService.PatchElementsAsync(@"<div id=""instructions"">...</div>");
datastarService.PatchSignalsAsync(new { answer = "...", prize = "..." } );
```

**go**

```go
sse.PatchElements(`<div id="question">...</div>`)
sse.PatchElements(`<div id="instructions">...</div>`)
sse.PatchSignals([]byte(`{answer: '...', prize: '...'}`))
```

**java**

```java
generator.send(PatchElements.builder()
    .data("<div id=\"question\">...</div>")
    .build()
);
generator.send(PatchElements.builder()
    .data("<div id=\"instructions\">...</div>")
    .build()
);
generator.send(PatchSignals.builder()
    .data("{\"answer\": \"...\", \"prize\": \"...\"}")
    .build()
);
```

**kotlin**

```kotlin
generator.patchElements(
    elements = """<div id="question">...</div>""",
)
generator.patchElements(
    elements = """<div id="instructions">...</div>""",
)
generator.patchSignals(
    signals = """{"answer": "...", "prize": "..."}""",
)
```

**php**

```php
$sse->patchElements('<div id="question">...</div>');
$sse->patchElements('<div id="instructions">...</div>');
$sse->patchSignals(['answer' => '...', 'prize' => '...']);
```

**python**

```python
return DatastarResponse([
    SSE.patch_elements('<div id="question">...</div>'),
    SSE.patch_elements('<div id="instructions">...</div>'),
    SSE.patch_signals({"answer": "...", "prize": "..."})
])
```

**ruby**

```ruby
datastar.stream do |sse|
  sse.patch_elements('<div id="question">...</div>')
  sse.patch_elements('<div id="instructions">...</div>')
  sse.patch_signals(answer: '...', prize: '...')
end
```

**rust**

```rust
yield PatchElements::new("<div id='question'>...</div>").into()
yield PatchElements::new("<div id='instructions'>...</div>").into()
yield PatchSignals::new("{answer: '...', prize: '...'}").into()
```

**typescript**

```typescript
stream.patchElements('<div id="question">...</div>');
stream.patchElements('<div id="instructions">...</div>');
stream.patchSignals({'answer': '...', 'prize': '...'});
```

**zig**

```zig
try sse.PatchElements("<div id='question'>...</div>", .{});
try sse.PatchElements("<div id='instructions'>...</div>", .{});
try sse.PatchSignals(.{.answer: " ...", .prize: " ..."}, .{});
```

> In addition to your browser’s dev tools, the [Datastar Inspector](https://data-star.dev/pro#datastar-inspector) can be used to monitor and inspect SSE events received by Datastar.

Read more about SSE events in the [reference](https://data-star.dev/reference/sse_events).

## Congratulations #

You’ve actually read the entire guide! You should now know how to use Datastar to build reactive applications that communicate with the backend using backend requests and SSE events.

Feel free to dive into the [reference](https://data-star.dev/reference) and explore the [examples](https://data-star.dev/examples) next, to learn more about what you can do with Datastar.

If you’re wondering how to best use Datastar to build maintainable, scalable, high-performance web apps, read (and re-read) the [Tao of Datastar](https://data-star.dev/guide/the_tao_of_datastar).

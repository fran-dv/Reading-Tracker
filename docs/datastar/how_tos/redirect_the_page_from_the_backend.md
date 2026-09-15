<!-- Source: https://data-star.dev/how_tos/redirect_the_page_from_the_backend -->
<!-- Fetched: 2026-09-14 -->

# How to redirect the page from the backend

Redirecting to another page is a common task that can be done from the backend by patching a `script` tag into the DOM using a [`datastar-patch-elements`](https://data-star.dev/reference/sse_events#datastar-patch-elements) SSE event. Since this results in a browser redirect, existing signals will _not_ persist to the new page.

## Goal #

Our goal is to indicate to the user that they will be redirected, wait 3 seconds, and then redirect them to `/guide`, all from the backend.

Demo

Click to be redirected from the backend

## Steps #

We’ll place a `data-on:click` attribute on a button and use the `get` action to send a `GET` request to the backend. We’ll include an empty indicator `div` to show the user that they will be redirected.

```html
<button data-on:click="@get('/endpoint')">
    Click to be redirected from the backend
</button>
<div id="indicator"></div>
```

We’ll set up our backend to first send a `datastar-patch-elements` event with a populated indicator fragment, then wait 3 seconds, and then send another `datastar-patch-elements` SSE event to append a `script` tag that redirects the page.

```text
event: datastar-patch-elements
data: elements <div id="indicator">Redirecting in 3 seconds...</div>

// Wait 3 seconds

event: datastar-patch-elements
data: selector body
data: mode append
data: elements <script>window.location.href = "/guide"</script>
```

All SDKs provide an `ExecuteScript` helper function that wraps the provided code in a `script` tag and patches it into the DOM.

**clojure**

```clojure
(require
  '[starfederation.datastar.clojure.api :as d*]
  '[starfederation.datastar.clojure.adapter.http-kit :refer [->sse-response on-open]]
  '[some.hiccup.library :refer [html]])

(defn handle [ring-request]
  (->sse-response ring-request
    {on-open
      (fn [sse]
        (d*/patch-elements! sse
          (html [:div#indicator "Redirecting in 3 seconds..."]))
        (Thread/sleep 3000)
        (d*/execute-script! sse "window.location = \"/guide\"")
        (d*/close-sse! sse)}))
```

**csharp**

```csharp
using StarFederation.Datastar.DependencyInjection;

app.MapGet("/redirect", async (IDatastarService datastarService) =>
{
    await datastarService.PatchElementsAsync("""<div id="indicator">Redirecting in 3 seconds...</div>""");
    await Task.Delay(TimeSpan.FromSeconds(3));
    await datastarService.ExecuteScriptAsync("""window.location = "/guide";""");
});
```

**go**

```go
import (
    "time"
    "github.com/starfederation/datastar-go/datastar"
)

sse := datastar.NewSSE(w, r)
sse.PatchElements(`
    <div id="indicator">Redirecting in 3 seconds...</div>
`)
time.Sleep(3 * time.Second)
sse.ExecuteScript(`
    window.location = "/guide"
`)
```

No example found for Java

**kotlin**

```kotlin
val generator = ServerSentEventGenerator(response)

generator.patchElements(
    elements =
        """
        <div id="indicator">Redirecting in 3 seconds...</div>
        """.trimIndent(),
)

Thread.sleep(3 * ONE_SECOND)

generator.executeScript(
    script = "window.location.href = '/success'",
)
```

**php**

```php
use starfederation\datastar\ServerSentEventGenerator;

$sse = new ServerSentEventGenerator();
$sse->patchElements(`
    <div id="indicator">Redirecting in 3 seconds...</div>
`);
sleep(3);
$sse->executeScript(`
    window.location = "/guide"
`);
```

**python**

```python
from datastar_py import ServerSentEventGenerator as SSE
from datastar_py.sanic import datastar_response

@app.get("/redirect")
@datastar_response
async def redirect_from_backend():
    yield SSE.patch_elements('<div id="indicator">Redirecting in 3 seconds...</div>')
    await asyncio.sleep(3)
    yield SSE.execute_script('window.location = "/guide"')
```

**ruby**

```ruby
datastar = Datastar.new(request:, response:)

datastar.stream do |sse|
  sse.patch_elements '<div id="indicator">Redirecting in 3 seconds...</div>'
  sleep 3
  sse.execute_script 'window.location = "/guide"'
end
```

**rust**

```rust
use datastar::prelude::*;
use async_stream::stream;
use core::time::Duration;

Sse(stream! {
    yield PatchElements::new("<div id='indicator'>Redirecting in 3 seconds...</div>").into();
    tokio::time::sleep(core::time::Duration::from_secs(3)).await;
    yield ExecuteScript::new("window.location = '/guide'").into();
});
```

**typescript**

```typescript
import { createServer } from "node:http";
import { ServerSentEventGenerator } from "../npm/esm/node/serverSentEventGenerator.js";

const server = createServer(async (req, res) => {

  ServerSentEventGenerator.stream(req, res, async (sse) => {
    sse.patchElements(`
      <div id="indicator">Redirecting in 3 seconds...</div>
    `);

    setTimeout(() => {
      sse.executeScript(`window.location = "/guide"`);
    }, 3000);
  });
});
```

No example found for Zig

Note that in Firefox, if a redirect happens within a `script` tag then the URL is _replaced_ , rather than _pushed_ , meaning that the previous URL won’t show up in the back history (or back/forward navigation).

To work around this, you can wrap the redirect in a `setTimeout` function call. See [issue #529](https://github.com/starfederation/datastar/issues/529) for reference.

**clojure**

```clojure
(require
  '[starfederation.datastar.clojure.api :as d*]
  '[starfederation.datastar.clojure.adapter.http-kit :refer [->sse-response on-open]]
  '[some.hiccup.library :refer [html]])

(defn handle [ring-request]
  (->sse-response ring-request
    {on-open
      (fn [sse]
        (d*/patch-elements! sse
          (html [:div#indicator "Redirecting in 3 seconds..."]))
        (Thread/sleep 3000)
        (d*/execute-script! sse
          "setTimeout(() => window.location = \"/guide\")"
        (d*/close-sse! sse))}))
```

**csharp**

```csharp
using StarFederation.Datastar.DependencyInjection;

app.MapGet("/redirect", async (IDatastarService datastarService) =>
{
    await datastarService.PatchElementsAsync("""<div id="indicator">Redirecting in 3 seconds...</div>""");
    await Task.Delay(TimeSpan.FromSeconds(3));
    await datastarService.ExecuteScriptAsync("""setTimeout(() => window.location = "/guide");""");
});
```

**go**

```go
import (
    "time"
    "github.com/starfederation/datastar-go/datastar"
)

sse := datastar.NewSSE(w, r)
sse.PatchElements(`
    <div id="indicator">Redirecting in 3 seconds...</div>
`)
time.Sleep(3 * time.Second)
sse.ExecuteScript(`
    setTimeout(() => window.location = "/guide")
`)
```

No example found for Java

**kotlin**

```kotlin
val generator = ServerSentEventGenerator(response)

generator.patchElements(
    elements =
        """
        <div id="indicator">Redirecting in 3 seconds...</div>
        """.trimIndent(),
)

Thread.sleep(3 * ONE_SECOND)

generator.executeScript(
    script = "setTimeout(() => window.location = '/guide')",
)
```

**php**

```php
use starfederation\datastar\ServerSentEventGenerator;

$sse = new ServerSentEventGenerator();
$sse->patchElements(`
    <div id="indicator">Redirecting in 3 seconds...</div>
`);
sleep(3);
$sse->executeScript(`
    setTimeout(() => window.location = "/guide")
`);
```

**python**

```python
from datastar_py import ServerSentEventGenerator as SSE
from datastar_py.sanic import datastar_response

@app.get("/redirect")
@datastar_response
async def redirect_from_backend():
    yield SSE.patch_elements('<div id="indicator">Redirecting in 3 seconds...</div>')
    await asyncio.sleep(3)
    yield SSE.execute_script('setTimeout(() => window.location = "/guide")')
```

**ruby**

```ruby
datastar = Datastar.new(request:, response:)

datastar.stream do |sse|
  sse.patch_elements '<div id="indicator">Redirecting in 3 seconds...</div>'

  sleep 3

  sse.execute_script <<~JS
    setTimeout(() => {
      window.location = '/guide'
    })
  JS
end
```

**rust**

```rust
use datastar::prelude::*;
use async_stream::stream;
use core::time::Duration;

Sse(stream! {
    yield PatchElements::new("<div id='indicator'>Redirecting in 3 seconds...</div>").into();
    tokio::time::sleep(core::time::Duration::from_secs(3)).await;
    yield ExecuteScript::new("setTimeout(() => window.location = '/guide')").into();
});
```

**typescript**

```typescript
import { createServer } from "node:http";
import { ServerSentEventGenerator } from "../npm/esm/node/serverSentEventGenerator.js";

const server = createServer(async (req, res) => {

  ServerSentEventGenerator.stream(req, res, async (sse) => {
    sse.patchElements(`
      <div id="indicator">Redirecting in 3 seconds...</div>
    `);

    setTimeout(() => {
      sse.executeScript(`setTimeout(() => window.location = "/guide")`);
    }, 3000);
  });
});
```

No example found for Zig

Some SDKs provide a helper method that automatically wraps the statement in a `setTimeout` function call, so you don’t have to worry about doing so (you’re welcome!).

**clojure**

```clojure
(require
  '[starfederation.datastar.clojure.api :as d*]
  '[starfederation.datastar.clojure.adapter.http-kit :refer [->sse-response on-open]]
  '[some.hiccup.library :refer [html]])

(defn handler [ring-request]
  (->sse-response ring-request
    {on-open
      (fn [sse]
        (d*/patch-elements! sse
          (html [:div#indicator "Redirecting in 3 seconds..."]))
        (Thread/sleep 3000)
        (d*/redirect! sse "/guide")
        (d*/close-sse! sse))}))
```

**csharp**

```csharp
using StarFederation.Datastar.DependencyInjection;
using StarFederation.Datastar.Scripts;

app.MapGet("/redirect", async (IDatastarService datastarService) =>
{
    await datastarService.PatchElementsAsync("""<div id="indicator">Redirecting in 3 seconds...</div>""");
    await Task.Delay(TimeSpan.FromSeconds(3));
    await datastarService.Redirect("/guide");
});
```

**go**

```go
import (
    "time"
    "github.com/starfederation/datastar-go/datastar"
)

sse := datastar.NewSSE(w, r)
sse.PatchElements(`
    <div id="indicator">Redirecting in 3 seconds...</div>
`)
time.Sleep(3 * time.Second)
sse.Redirect("/guide")
```

No example found for Java

**kotlin**

```kotlin
val generator = ServerSentEventGenerator(response)

generator.patchElements(
    elements =
        """
        <div id="indicator">Redirecting in 3 seconds...</div>
        """.trimIndent(),
)

Thread.sleep(3 * ONE_SECOND)

generator.redirect("/guide")
```

**php**

```php
use starfederation\datastar\ServerSentEventGenerator;

$sse = new ServerSentEventGenerator();
$sse->patchElements(`
    <div id="indicator">Redirecting in 3 seconds...</div>
`);
sleep(3);
$sse->location('/guide');
```

**python**

```python
from datastar_py import ServerSentEventGenerator as SSE
from datastar_py.sanic import datastar_response

@app.get("/redirect")
@datastar_response
async def redirect_from_backend():
    yield SSE.patch_elements('<div id="indicator">Redirecting in 3 seconds...</div>')
    await asyncio.sleep(3)
    yield SSE.redirect("/guide")
```

**ruby**

```ruby
datastar = Datastar.new(request:, response:)

datastar.stream do |sse|
  sse.patch_elements '<div id="indicator">Redirecting in 3 seconds...</div>'

  sleep 3

  sse.redirect '/guide'
end
```

No example found for Rust

No example found for TypeScript

No example found for Zig

## Conclusion #

Redirecting to another page can be done from the backend thanks to the ability to patch `script` tags into the DOM using the [`datastar-patch-elements`](https://data-star.dev/reference/sse_events#datastar-patch-elements) SSE event, or to execute JavaScript using an SDK.

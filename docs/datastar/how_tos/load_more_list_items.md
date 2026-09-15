<!-- Source: https://data-star.dev/how_tos/load_more_list_items -->
<!-- Fetched: 2026-09-14 -->

# How to load more list items

Loading more list items into the DOM from the backend is a common alternative to pagination. What makes it different is that we need to append the new items to the existing list, rather than replace them.

## Goal #

Our goal is to incrementally append list items into a specific part of the DOM, each time a button is clicked. Once five items are visible, the button should be removed.

Demo

  * Item 1

Click to load another item

## Steps #

We’ll give the list item container and the button unique IDs, so that we can target them individually.

We’ll use a `data-signals` attribute to set the initial `offset` to `1`, and a `data-on:click` button that will send a `GET` request to the backend.

```html
<div id="list">
<div>Item 1</div>
</div>
<button id="load-more" 
        data-signals:offset="1" 
        data-on:click="@get('/how_tos/load_more/data')">
Click to load another item
</button>
```

The backend will receive the `offset` signal and, if not above the max number of allowed items, will return the next item to be appended to the list.

We’ll set up our backend to send a [`datastar-patch-elements`](https://data-star.dev/reference/sse_events#datastar-patch-elements) event with the `selector` option set to `#list` and the `mode` option set to `append`. This tells Datastar to _append_ the elements _into_ the `#list` container (rather than the default behaviour of replacing it).

```text
event: datastar-patch-elements
data: selector #list
data: mode append
data: elements <div>Item 2</div>
```

In addition, we’ll send a [`datastar-patch-signals`](https://data-star.dev/reference/sse_events#datastar-patch-signals) event to update the `offset`.

```text
event: datastar-patch-signals
data: signals {offset: 2}
```

In the case when all five list items have been shown, we’ll remove the button from the DOM entirely.

```text
event: datastar-patch-elements
data: selector #load-more
data: mode remove
```

Here’s how it might look using the SDKs.

**clojure**

```clojure
(require
  '[starfederation.datastar.clojure.api :as d*]
  '[starfederation.datastar.clojure.adapter.http-kit :refer [->sse-response on-open]]
  '[some.hiccup.library :refer [html]]
  '[some.json.library :refer [read-json-str write-json-str]]))

(def max-offset 5)

(defn handler [ring-request]
  (->sse-response ring-request
    {on-open
     (fn [sse]
       (let [d*-signals (-> ring-request d*/get-signals read-json-str)
             offset (get d*-signals "offset")
             limit 1
             new-offset (+ offset limit)]

         (d*/patch-elements! sse
                             (html [:div "Item " new-offset])
                             {d*/selector   "#list"
                              d*/merge-mode d*/mm-append})

         (if (< new-offset max-offset)
           (d*/patch-signals! sse (write-json-str {"offset" new-offset}))
           (d*/remove-fragment! sse "#load-more"))

         (d*/close-sse! sse)))}))
```

**csharp**

```csharp
using System.Text.Json;
using StarFederation.Datastar;
using StarFederation.Datastar.DependencyInjection;

public class Program
{
    public record OffsetSignals(int offset);

    public static void Main(string[] args)
    {
        var builder = WebApplication.CreateBuilder(args);
        builder.Services.AddDatastar();
        var app = builder.Build();

        app.MapGet("/more", async (IDatastarService datastarService) =>
        {
            var max = 5;
            var limit = 1;
            var signals = await datastarService.ReadSignalsAsync<OffsetSignals>();
            var offset = signals.offset;
            if (offset < max)
            {
                var newOffset = offset + limit;
                await datastarService.PatchElementsAsync($"<div>Item {newOffset}</div>", new()
                {
                    Selector = "#list",
                    PatchMode = PatchElementsMode.Append,
                });
                if (newOffset < max)
                    await datastarService.PatchSignalsAsync(new OffsetSignals(newOffset));
                else
                    await datastarService.RemoveElementAsync("#load-more");
            }
        });

        app.Run();
    }
}
```

**go**

```go
import (
    "fmt"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/starfederation/datastar-go/datastar"
)

type OffsetSignals struct {
    Offset int `json:"offset"`
}

signals := &OffsetSignals{}
if err := datastar.ReadSignals(r, signals); err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
}

max := 5
limit := 1
offset := signals.Offset

sse := datastar.NewSSE(w, r)

if offset < max {
    newOffset := offset + limit
    sse.PatchElements(fmt.Sprintf(`<div>Item %d</div>`, newOffset),
        datastar.WithSelectorID("list"),
        datastar.WithModeAppend(),
    )
    if newOffset < max {
        sse.PatchSignals([]byte(fmt.Sprintf(`{offset: %d}`, newOffset)))
    } else {
        sse.RemoveElements(`#load-more`)
    }
}
```

No example found for Java

**kotlin**

```kotlin
@Serializable
data class OffsetSignals(
    val offset: Int,
)

val signals =
    readSignals(
        request,
        { json: String -> Json.decodeFromString<OffsetSignals>(json) },
    )

val max = 5
val limit = 1
val offset = signals.offset

val generator = ServerSentEventGenerator(response)

if (offset < max) {
    val newOffset = offset + limit

    generator.patchElements(
        elements = "<div>Item $newOffset</div>",
        options =
            PatchElementsOptions(
                selector = "#list",
                mode = ElementPatchMode.Append,
            ),
    )

    if (newOffset < max) {
        generator.patchSignals(
            signals = """{"offset": $newOffset}""",
        )
    } else {
        generator.patchElements(
            options =
                PatchElementsOptions(
                    selector = "#load-more",
                    mode = ElementPatchMode.Remove,
                ),
        )
    }
}
```

**php**

```php
use starfederation\datastar\enums\ElementPatchMode;
use starfederation\datastar\ServerSentEventGenerator;

$signals = ServerSentEventGenerator::readSignals();

$max = 5;
$limit = 1;
$offset = $signals['offset'] ?? 1;

$sse = new ServerSentEventGenerator();

if ($offset < $max) {
    $newOffset = $offset + $limit;
    $sse->patchElements("<div>Item $newOffset</div>", [
        'selector' => '#list',
        'mode' => ElementPatchMode::Append,
    ]);
    if (newOffset < $max) {
        $sse->patchSignals(['offset' => $newOffset]);
    } else {
        $sse->removeElements('#load-more');
    }
}
```

**python**

```python
from datastar_py import ServerSentEventGenerator as SSE
from datastar_py.consts import ElementPatchMode
from datastar_py.fastapi import datastar_response, ReadSignals

MAX_ITEMS = 5

@app.get("/how_tos/load_more/data")
@datastar_response
async def load_data(signals: ReadSignals):
    if signals["offset"] < MAX_ITEMS:
        new_offset = signals["offset"] + 1
        yield SSE.patch_elements(
            f"<div>Item {new_offset}</div>",
            mode=ElementPatchMode.APPEND,
            selector="#list"
        )
        if new_offset < MAX_ITEMS:
            yield SSE.patch_signals({"offset": new_offset})
        else:
            yield SSE.remove_elements("#load-more")
```

No example found for Ruby

No example found for Rust

No example found for TypeScript

No example found for Zig

## Conclusion #

While using the default mode of `outer` is generally recommended, appending to a list is a good example of when to use the `append` mode.

<!-- Source: https://data-star.dev/how_tos/poll_the_backend_at_regular_intervals -->
<!-- Fetched: 2026-09-14 -->

# How to poll the backend at regular intervals

Polling is a pull-based mechanism for fetching data from the server at regular intervals. It is useful when you want to refresh the UI on the frontend, based on real-time data from the backend. 

This in contrast to a push-based mechanism, in which a long-lived SSE connection is kept open between the client and the server, and the server pushes updates to the client whenever necessary. Push-based mechanisms are more efficient than polling, and can be achieved using Datastar, but may be less desirable for some backends.

In PHP, for example, keeping long-lived SSE connections is fine for a dashboard in which users are authenticated, as the number of connections are limited. For a public-facing website, however, it is not recommended to open many long-lived connections, due to the architecture of most PHP servers.

## Goal #

Our goal is to poll the backend at regular intervals (starting at 5 second intervals) and update the UI accordingly. The backend will determine changes to the DOM and be able to control the rate at which the frontend polls based on some criteria. For this example, we will simply output the server time, increasing the polling frequency to 1 second during the last 10 seconds of every minute. The criteria could of course be anything such as the number of times previously polled, the user’s role, load on the server, etc.

Demo

## Steps #

The `data-on-interval` attribute allows us to run an expression at a regular interval. We’ll use it to send a `GET` request to the backend, and use the `__duration` modifier to set the interval duration.

```html
<div id="time"
     data-on-interval__duration.5s="@get('/endpoint')"
></div>
```

In addition to the interval, we could also run the expression immediately by adding `.leading` to the modifier.

```html
<div id="time"
     data-on-interval__duration.5s.leading="@get('/endpoint')"
></div>
```

Most of the time, however, we’d just render the current time on page load using a backend templating language.

```html
<div id="time"
     data-on-interval__duration.5s="@get('/endpoint')"
>
     {{ now }}
</div>
```

Now our backend can respond to each request with a [`datastar-patch-elements`](https://data-star.dev/reference/sse_events#datastar-patch-elements) event with an updated version of the element.

```text
event: datastar-patch-elements
data: elements <div id="time" data-on-interval__duration.5s="@get('/endpoint')">
data: elements     {{ now }}
data: elements </div>
```

Be careful not to add `.leading` to the modifier in the response, as it will cause the frontend to immediately send another request.

Here’s how it might look using the SDKs.

**clojure**

```clojure
(require
  '[starfederation.datastar.clojure.api :as d*]
  '[starfederation.datastar.clojure.adapter.http-kit :refer [->sse-response on-open]])
  '[some.hiccup.library :refer [html]])

(import
  'java.time.format.DateTimeFormatter
  'java.time.LocalDateTime)

(def formatter (DateTimeFormatter/ofPattern "YYYY-MM-DD HH:mm:ss"))

(defn handle [ring-request]
   (->sse-response ring-request
     {on-open
      (fn [sse]
        (d*/patch-elements! sse
          (html [:div#time {:data-on-interval__duration.5s (d*/sse-get "/endpoint")}
                  (LocalDateTime/.format (LocalDateTime/now) formatter)])))}))

        (d*/close-sse! sse))}))
```

**csharp**

```csharp
using StarFederation.Datastar.DependencyInjection;

app.MapGet("/endpoint", async (IDatastarService datastarService) =>
{
    var currentTime = DateTime.Now.ToString("yyyy-MM-dd hh:mm:ss");
    await datastarService.PatchElementsAsync($"""
        <div id="time" data-on-interval__duration.5s="@get('/endpoint')">
            {currentTime}
        </div>
    """);
});
```

**go**

```go
import (
    "time"
    "github.com/starfederation/datastar-go/datastar"
)

currentTime := time.Now().Format("2006-01-02 15:04:05")

sse := datastar.NewSSE(w, r)
sse.PatchElements(fmt.Sprintf(`
    <div id="time" data-on-interval__duration.5s="@get('/endpoint')">
        %s
    </div>
`, currentTime))
```

No example found for Java

**kotlin**

```kotlin
val now: LocalDateTime = currentTime()

val generator = ServerSentEventGenerator(response)

generator.patchElements(
    elements =
        """
        <div id="time" data-on-interval__duration.5s="@get('/endpoint')">
            $now
        </div>
        """.trimIndent(),
)
```

**php**

```php
use starfederation\datastar\ServerSentEventGenerator;

$currentTime = date('Y-m-d H:i:s');

$sse = new ServerSentEventGenerator();
$sse->patchElements(`
    <div id="time"
         data-on-interval__duration.5s="@get('/endpoint')"
    >
        $currentTime
    </div>
`);
```

**python**

```python
from datastar_py import ServerSentEventGenerator as SSE
from datastar_py.sanic import DatastarResponse

@app.get("/endpoint")
async def endpoint():
    current_time = datetime.now()

    return DatastarResponse(SSE.patch_elements(f"""
        <div id="time" data-on-interval__duration.5s="@get('/endpoint')">
            {current_time:%Y-%m-%d %H:%M:%S}
        </div>
    """))
```

**ruby**

```ruby
datastar = Datastar.new(request:, response:)

current_time = Time.now.strftime('%Y-%m-%d %H:%M:%S')

datastar.patch_elements <<~FRAGMENT
    <div id="time"
         data-on-interval__duration.5s="@get('/endpoint')"
    >
        #{current_time}
    </div>
FRAGMENT
```

**rust**

```rust
use datastar::prelude::*;
use chrono::Local;
use async_stream::stream;

let current_time = Local::now().format("%Y-%m-%d %H:%M:%S").to_string();

Sse(stream! {
    yield PatchElements::new(
        format!(
            "<div id='time' data-on-interval__duration.5s='@get(\"/endpoint\")'>{}</div>",
            current_time
        )
    ).into();
})
```

**typescript**

```typescript
import { createServer } from "node:http";
import { ServerSentEventGenerator } from "../npm/esm/node/serverSentEventGenerator.js";

const server = createServer(async (req, res) => {
  const currentTime = new Date().toISOString();
  
  ServerSentEventGenerator.stream(req, res, (sse) => {
    sse.patchElements(`
       <div id="time"
          data-on-interval__duration.5s="@get('/endpoint')"
       >
         ${currentTime}
       </div>
    `);
  });
});
```

No example found for Zig

Our second requirement was that the polling frequency should increase to 1 second during the last 10 seconds of every minute. To make this possible, we’ll calculate and output the interval duration based on the current seconds of the minute.

**clojure**

```clojure
(require
  '[starfederation.datastar.clojure.api :as d*]
  '[starfederation.datastar.clojure.adapter.http-kit :refer [->sse-response on-open]])
  '[some.hiccup.library :refer [html]])

(import
  'java.time.format.DateTimeFormatter
  'java.time.LocalDateTime)

(def date-time-formatter (DateTimeFormatter/ofPattern "YYYY-MM-DD HH:mm:ss"))
(def seconds-formatter (DateTimeFormatter/ofPattern "ss"))

(defn handle [ring-request]
  (->sse-response ring-request
    {on-open
     (fn [sse]
       (let [now (LocalDateTime/now)
             current-time (LocalDateTime/.format now date-time-formatter)
             seconds (LocalDateTime/.format now seconds-formatter)
             duration (if (neg? (compare seconds "50"))
                         "5"
                         "1")]
         (d*/patch-elements! sse
           (html [:div#time {(str "data-on-interval__duration." duration "s")
                             (d*/sse-get "/endpoint")}
                   current-time]))))}))

         (d*/close-sse! sse))}))
```

**csharp**

```csharp
using StarFederation.Datastar.DependencyInjection;

app.MapGet("/endpoint", async (IDatastarService datastarService) =>
{
    var currentTime = DateTime.Now.ToString("yyyy-MM-dd hh:mm:ss");
    var currentSeconds = DateTime.Now.Second;
    var duration = currentSeconds < 50 ? 5 : 1;
    await datastarService.PatchElementsAsync($"""
        <div id="time" data-on-interval__duration.{duration}s="@get('/endpoint')">
            {currentTime}
        </div>
    """);
});
```

**go**

```go
import (
    "time"
    "github.com/starfederation/datastar-go/datastar"
)

currentTime := time.Now().Format("2006-01-02 15:04:05")
currentSeconds := time.Now().Format("05")
duration := 1
if currentSeconds < "50" {
    duration = 5
}

sse := datastar.NewSSE(w, r)
sse.PatchElements(fmt.Sprintf(`
    <div id="time" data-on-interval__duration.%ds="@get('/endpoint')">
        %s
    </div>
`, duration, currentTime))
```

No example found for Java

**kotlin**

```kotlin
val now: LocalDateTime = currentTime()
val currentSeconds = now.second
val duration = if (currentSeconds < 50) 5 else 1

val generator = ServerSentEventGenerator(response)

generator.patchElements(
    elements =
        """
        <div id="time" data-on-interval__duration.${duration}s="@get('/endpoint')">
            $now
        </div>
        """.trimIndent(),
)
```

**php**

```php
use starfederation\datastar\ServerSentEventGenerator;

$currentTime = date('Y-m-d H:i:s');
$currentSeconds = date('s');
$duration = $currentSeconds < 50 ? 5 : 1;

$sse = new ServerSentEventGenerator();
$sse->patchElements(`
    <div id="time"
         data-on-interval__duration.${duration}s="@get('/endpoint')"
    >
        $currentTime
    </div>
`);
```

**python**

```python
from datastar_py import ServerSentEventGenerator as SSE
from datastar_py.sanic import DatastarResponse

@app.get("/endpoint")
async def endpoint():
    current_time = datetime.now()
    duration = 5 if current_time.seconds < 50 else 1

    return DatastarResponse(SSE.patch_elements(f"""
        <div id="time" data-on-interval__duration.{duration}s="@get('/endpoint')">
            {current_time:%Y-%m-%d %H:%M:%S}
        </div>
    """))
```

**ruby**

```ruby
datastar = Datastar.new(request:, response:)

now = Time.now
current_time = now.strftime('%Y-%m-%d %H:%M:%S')
current_seconds = now.strftime('%S').to_i
duration = current_seconds < 50 ? 5 : 1

datastar.patch_elements <<~FRAGMENT
    <div id="time"
         data-on-interval__duration.#{duration}s="@get('/endpoint')"
    >
        #{current_time}
    </div>
FRAGMENT
```

**rust**

```rust
use datastar::prelude::*;
use chrono::Local;
use async_stream::stream;

let current_time = Local::now().format("%Y-%m-%d %H:%M:%S").to_string();
let current_seconds = Local::now().second();
let duration = if current_seconds < 50 {
    5
} else {
    1
};

Sse(stream! {
    yield PatchElements::new(
        format!(
            "<div id='time' data-on-interval__duration.{}s='@get(\"/endpoint\")'>{}</div>",
            duration,
            current_time,
        )
    ).into();
})
```

**typescript**

```typescript
import { createServer } from "node:http";
import { ServerSentEventGenerator } from "../npm/esm/node/serverSentEventGenerator.js";

const server = createServer(async (req, res) => {
  const currentTime = new Date();
  const duration = currentTime.getSeconds > 50 ? 5 : 1;

  ServerSentEventGenerator.stream(req, res, (sse) => {
    sse.patchElements(`
       <div id="time"
          data-on-interval__duration.${duration}s="@get('/endpoint')"
       >
         ${currentTime.toISOString()}
       </div>
    `);
  });
});
```

No example found for Zig

## Conclusion #

Using this approach, we not only end up with a way to poll the backend at regular intervals, but we can also control the rate at which the frontend polls based on whatever criteria our backend requires.

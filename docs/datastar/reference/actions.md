<!-- Source: https://data-star.dev/reference/actions -->
<!-- Fetched: 2026-09-14 -->

# Actions

Datastar provides actions (helper functions) that can be used in Datastar expressions.

> The `@` prefix designates actions that are safe to use in expressions. This is a security feature that prevents arbitrary JavaScript from being executed in the browser. Datastar uses [`Function()` constructors](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Function/Function) to create and execute these actions in a secure and controlled sandboxed environment.

### `@peek()` #

> `@peek(callable: () => any)`

Allows accessing signals without subscribing to their changes in expressions.

```html
<div data-text="$foo + @peek(() => $bar)"></div>
```

In the example above, the expression in the `data-text` attribute will be re-evaluated whenever `$foo` changes, but it will _not_ be re-evaluated when `$bar` changes, since it is evaluated inside the `@peek()` action.

### `@setAll()` #

> `@setAll(value: any, filter?: {include: RegExp, exclude?: RegExp})`

Sets the value of all matching signals (or all signals if no filter is used) to the expression provided in the first argument. The second argument is an optional filter object with an `include` property that accepts a regular expression to match signal paths. You can optionally provide an `exclude` property to exclude specific patterns.

> The [Datastar Inspector](https://data-star.dev/pro#datastar-inspector) can be used to inspect and filter current signals and view signal patch events in real-time.

```html
<!-- Sets the `foo` signal only -->
<div data-signals:foo="false">
    <button data-on:click="@setAll(true, {include: /^foo$/})"></button>
</div>

<!-- Sets all signals starting with `user.` -->
<div data-signals="{user: {name: '', nickname: ''}}">
    <button data-on:click="@setAll('johnny', {include: /^user\./})"></button>
</div>

<!-- Sets all signals except those ending with `_temp` -->
<div data-signals="{data: '', data_temp: '', info: '', info_temp: ''}">
    <button data-on:click="@setAll('reset', {include: /.*/, exclude: /_temp$/})"></button>
</div>
```

### `@toggleAll()` #

> `@toggleAll(filter?: {include: RegExp, exclude?: RegExp})`

Toggles the boolean value of all matching signals (or all signals if no filter is used). The argument is an optional filter object with an `include` property that accepts a regular expression to match signal paths. You can optionally provide an `exclude` property to exclude specific patterns.

> The [Datastar Inspector](https://data-star.dev/pro#datastar-inspector) can be used to inspect and filter current signals and view signal patch events in real-time.

```html
<!-- Toggles the `foo` signal only -->
<div data-signals:foo="false">
    <button data-on:click="@toggleAll({include: /^foo$/})"></button>
</div>

<!-- Toggles all signals starting with `is` -->
<div data-signals="{isOpen: false, isActive: true, isEnabled: false}">
    <button data-on:click="@toggleAll({include: /^is/})"></button>
</div>

<!-- Toggles signals starting with `settings.` -->
<div data-signals="{settings: {darkMode: false, autoSave: true}}">
    <button data-on:click="@toggleAll({include: /^settings\./})"></button>
</div>
```

## Backend Actions #

### `@get()` #

> `@get(uri: string, options={ })`

Sends a `GET` request to the backend using the [Fetch API](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API). The URI can be any valid endpoint, and the response type any of the allowed response types, or a `204 No Content` response if the response body is empty.

```html
<button data-on:click="@get('/endpoint')"></button>
```

By default, requests are sent with a `Datastar-Request: true` header, and a `{datastar: *}` object containing all existing signals, except those beginning with an underscore. This behavior can be changed using the `filterSignals` option, which allows you to include or exclude specific signals using regular expressions.

> When using a `get` request, the signals are sent as a query parameter, otherwise they are sent as a JSON body.

When a page is hidden (in a background tab, for example), the default behavior for `get` requests is for the SSE connection to be closed, and reopened when the page becomes visible again. To keep the connection open when the page is hidden, set the `openWhenHidden` option to `true`.

```html
<button data-on:click="@get('/endpoint', {openWhenHidden: true})"></button>
```

It’s possible to send form encoded requests by setting the `contentType` option to `form`. This sends requests using `application/x-www-form-urlencoded` encoding.

```html
<button data-on:click="@get('/endpoint', {contentType: 'form'})"></button>
```

It’s also possible to send requests using `multipart/form-data` encoding by specifying it in the `form` element’s [`enctype`](https://developer.mozilla.org/en-US/docs/Web/API/HTMLFormElement/enctype) attribute. This should be used when uploading files. See the [form data example](https://data-star.dev/examples/form_data).

```html
<form enctype="multipart/form-data">
    <input type="file" name="file" />
    <button data-on:click="@post('/endpoint', {contentType: 'form'})"></button>
</form>
```

### `@post()` #

> `@post(uri: string, options={ })`

Works the same as `@get()` but sends a `POST` request to the backend.

```html
<button data-on:click="@post('/endpoint')"></button>
```

### `@put()` #

> `@put(uri: string, options={ })`

Works the same as `@get()` but sends a `PUT` request to the backend.

```html
<button data-on:click="@put('/endpoint')"></button>
```

### `@patch()` #

> `@patch(uri: string, options={ })`

Works the same as `@get()` but sends a `PATCH` request to the backend.

```html
<button data-on:click="@patch('/endpoint')"></button>
```

### `@delete()` #

> `@delete(uri: string, options={ })`

Works the same as `@get()` but sends a `DELETE` request to the backend.

```html
<button data-on:click="@delete('/endpoint')"></button>
```

### Options #

All of the actions above take a second argument of options.

  * `contentType` – The type of content to send. A value of `json` sends all signals in a JSON request. A value of `form` tells the action to look for the closest form to the element on which it is placed (unless a `selector` option is provided), perform validation on the form elements, and send them to the backend using a form request (no signals are sent). Defaults to `json`.
  * `filterSignals` – A filter object with an `include` property that accepts a regular expression to match signal paths (defaults to all signals: `/.*/`), and an optional `exclude` property to exclude specific signal paths (defaults to all signals that do not have a `_` prefix: `/(^_|\._).*/`).

> The [Datastar Inspector](https://data-star.dev/pro#datastar-inspector) can be used to inspect and filter current signals and view signal patch events in real-time.

  * `selector` – Optionally specifies a form to send when the `contentType` option is set to `form`. If the value is `null`, the closest form is used. Defaults to `null`.
  * `headers` – An object containing headers to send with the request.
  * `openWhenHidden` – Whether to keep the connection open when the page is hidden. Useful for dashboards but can cause a drain on battery life and other resources when enabled. Defaults to `false` for `get` requests, and `true` for all other HTTP methods.
  * `payload` – Allows the fetch payload to be overridden with a custom object.
  * `retry` – Determines when to retry requests. Can be `'auto'` (default, retries on network errors only), `'error'` (retries on `4xx` and `5xx` responses), `'always'` (retries on all non-`204` responses except redirects), or `'never'` (disables retries). Defaults to `'auto'`.
  * `retryInterval` – The retry interval in milliseconds. Defaults to `1000` (one second).
  * `retryScaler` – A numeric multiplier applied to scale retry wait times. Defaults to `2`.
  * `retryMaxWait` – The maximum allowable wait time in milliseconds between retries. Defaults to `30000` (30 seconds).
  * `retryMaxCount` – The maximum number of retry attempts. Defaults to `10`.
  * `requestCancellation` – Controls request cancellation behavior. Can be `'auto'` (default, cancels in-flight requests to the same URL using the same HTTP method), `'cleanup'` (cancels requests on element or attribute cleanup, and cancels in-flight requests to the same URL using the same HTTP method), `'disabled'` (allows concurrent requests), or an `AbortController` instance for custom control. Defaults to `'auto'`.

```html
<button data-on:click="@get('/endpoint', {
    filterSignals: {include: /^foo\./},
    headers: {
        'X-Csrf-Token': 'JImikTbsoCYQ9oGOcvugov0Awc5LbqFsZW6ObRCxuq',
    },
    openWhenHidden: true,
    requestCancellation: 'disabled',
})"></button>
```

### Request Cancellation #

By default, a new fetch request automatically cancels any in-flight request with the same HTTP method and URL argument, regardless of which element initiated it.

For example, if a user rapidly clicks a button that triggers a backend action, only the most recent request remains active:

```html
<!-- Repeated clicks cancel the preceding in-flight GET request to this URL -->
<button data-on:click="@get('/slow-endpoint')">
    Load Data
</button>
```

Automatic cancellation is based on HTTP method and URL. Requests using different methods or URLs can run concurrently, without automatically cancelling each other.

You can control this behavior using the `requestCancellation` option:

```html
<!-- Disable automatic cancellation for this request -->
<button data-on:click="@get('/endpoint', {requestCancellation: 'disabled'})">
    Allow Multiple
</button>

<!-- Create a controller for each request so it can be cancelled manually -->
<button data-on:click="
    $controller = new AbortController(); 
    @get('/endpoint', {requestCancellation: $controller})
">
    Start Request
</button>
<button data-on:click="$controller?.abort()">
    Cancel Request
</button>
```

### Response Handling #

Backend actions automatically handle different response content types:

  * `text/event-stream` – Standard SSE responses with [Datastar SSE events](https://data-star.dev/reference/sse_events).
  * `text/html` – HTML elements to patch into the DOM.
  * `application/json` – JSON encoded signals to patch.
  * `text/javascript` – JavaScript code to execute in the browser.

#### `text/html`

When returning HTML (`text/html`), the server can optionally include the following response headers:

  * `datastar-selector` – A CSS selector for the target elements to patch
  * `datastar-mode` – How to patch the elements (`outer`, `inner`, `remove`, `replace`, `prepend`, `append`, `before`, `after`). Defaults to `outer`.
  * `datastar-use-view-transition` – Whether to use the [View Transition API](https://developer.mozilla.org/en-US/docs/Web/API/View_Transitions_API) when patching elements.

```
response.headers.set('Content-Type', 'text/html')
response.headers.set('datastar-selector', '#my-element')
response.headers.set('datastar-mode', 'inner')
response.body = '<p>New content</p>'
```

#### `application/json`

When returning JSON (`application/json`), the server can optionally include the following response header:

  * `datastar-only-if-missing` – If set to `true`, only patch signals that don’t already exist.

```
response.headers.set('Content-Type', 'application/json')
response.headers.set('datastar-only-if-missing', 'true')
response.body = JSON.stringify({ foo: 'bar' })
```

#### `text/javascript`

When returning JavaScript (`text/javascript`), the server can optionally include the following response header:

  * `datastar-script-attributes` – Sets the script element’s attributes using a JSON encoded string.

```
response.headers.set('Content-Type', 'text/javascript')
response.headers.set('datastar-script-attributes', JSON.stringify({ type: 'module' }))
response.body = 'console.log("Hello from server!");'
```

### Events #

All of the actions above trigger `datastar-fetch` events during the fetch request lifecycle. The event type determines the stage of the request.

  * `started` – Triggered when the fetch request is started.
  * `finished` – Triggered when the fetch request is finished.
  * `error` – Triggered when the fetch request encounters an error.
  * `retrying` – Triggered when the fetch request is retrying.
  * `retries-failed` – Triggered when all fetch retries have failed.

```html
<div data-on:datastar-fetch="
    evt.detail.type === 'error' && console.log('Fetch error encountered')
"></div>
```

## Pro Actions #

### `@clipboard()` #[Pro](https://data-star.dev/pro "Datastar Pro")

> `@clipboard(text: string, isBase64?: boolean)`

Copies the provided text to the clipboard. If the second parameter is `true`, the text is treated as [Base64](https://developer.mozilla.org/en-US/docs/Glossary/Base64) encoded, and is decoded before copying.

> Base64 encoding is useful when copying content that contains special characters, quotes, or code fragments that might not be valid within HTML attributes. This prevents parsing errors and ensures the content is safely embedded in `data-*` attributes.

```html
<!-- Copy plain text -->
<button data-on:click="@clipboard('Hello, world!')"></button>

<!-- Copy base64 encoded text (will decode before copying) -->
<button data-on:click="@clipboard('SGVsbG8sIHdvcmxkIQ==', true)"></button>
```

### `@fit()` #[Pro](https://data-star.dev/pro "Datastar Pro")

> `@fit(v: number, oldMin: number, oldMax: number, newMin: number, newMax: number, shouldClamp=false, shouldRound=false)`

Linearly interpolates a value from one range to another. This is useful for converting between different scales, such as mapping a slider value to a percentage or converting temperature units.

The optional `shouldClamp` parameter ensures the result stays within the new range, and `shouldRound` rounds the result to the nearest integer.

```html
<!-- Convert a 0-100 slider to 0-255 RGB value -->
<div>
    <input type="range" min="0" max="100" value="50" data-bind:slider-value>
    <div data-computed:rgb-value="@fit($sliderValue, 0, 100, 0, 255)">
        RGB Value: <span data-text="$rgbValue"></span>
    </div>
</div>

<!-- Convert Celsius to Fahrenheit -->
<div>
    <input type="number" data-bind:celsius value="20" />
    <div data-computed:fahrenheit="@fit($celsius, 0, 100, 32, 212)">
        <span data-text="$celsius"></span>°C = <span data-text="$fahrenheit.toFixed(1)"></span>°F
    </div>
</div>

<!-- Map mouse position to element opacity (clamped) -->
<div
    data-signals:mouse-x="0"
    data-computed:opacity="@fit($mouseX, 0, window.innerWidth, 0, 1, true)"
    data-on:mousemove__window="$mouseX = evt.clientX"
    data-attr:style="'opacity: ' + $opacity"
>
    Move your mouse horizontally to change opacity
</div>
```

### `@intl()` #[Pro](https://data-star.dev/pro "Datastar Pro")

> `@intl(type: string, value: any, options?: Record<string, any>, locale?: string | string[])`

Provides internationalized, locale-aware formatting for dates, numbers, and other values using the [Intl namespace object](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl).

```html
<!-- Converts a number to a formatted USD currency string in the user’s locale -->
<div data-text="@intl('number', 1000000, {style: 'currency', currency: 'USD'})"></div>

<!-- Converts a date to a formatted string in the specified locale -->
<div data-text="@intl('datetime', new Date(), {weekday: 'long', year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit'}, 'de-AT')"></div>
```

The `type` parameter specifies the type of value to format. Possible values:

  * `datetime` – formats dates and times using [Intl.DateTimeFormat](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/DateTimeFormat)
  * `number` – formats numbers using [Intl.NumberFormat](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/NumberFormat)
  * `pluralRules` – determines plural rules using [Intl.PluralRules](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/PluralRules)
  * `relativeTime` – formats relative time using [Intl.RelativeTimeFormat](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/RelativeTimeFormat)
  * `list` – formats lists using [Intl.ListFormat](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/ListFormat)
  * `displayNames` – gets display names for languages, regions, etc. using [Intl.DisplayNames](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/DisplayNames)

The `options` parameter can be one of several [Intl option types](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl#options_argument) depending on the type parameter:

  * [Intl.DateTimeFormatOptions](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/DateTimeFormat/DateTimeFormat#options) – when type is `datetime`
  * [Intl.NumberFormatOptions](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/NumberFormat/NumberFormat#options) – when type is `number`
  * [Intl.PluralRulesOptions](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/PluralRules/PluralRules#options) – when type is `pluralRules`
  * [Intl.RelativeTimeFormatOptions](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/RelativeTimeFormat/RelativeTimeFormat#options) – when type is `relativeTime`
  * [Intl.ListFormatOptions](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/ListFormat/ListFormat#options) – when type is `list`
  * [Intl.DisplayNamesOptions](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/DisplayNames/DisplayNames#options) – when type is `displayNames`

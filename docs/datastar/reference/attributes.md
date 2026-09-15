<!-- Source: https://data-star.dev/reference/attributes -->
<!-- Fetched: 2026-09-14 -->

# Attributes

Data attributes are evaluated in the order they appear in the DOM, have special casing rules, can be aliased to avoid conflicts with other libraries, can contain Datastar expressions, and have runtime error handling.

> The Datastar [VSCode extension](https://marketplace.visualstudio.com/items?itemName=starfederation.datastar-vscode) and [IntelliJ plugin](https://plugins.jetbrains.com/plugin/26072-datastar) provide autocomplete, diagnostics, hover documentation, and syntax highlighting.

### `data-attr` #

Sets the value of any HTML attribute to an expression, and keeps it in sync.

```html
<div data-attr:aria-label="$foo"></div>
```

The `data-attr` attribute can also be used to set the values of multiple attributes on an element using a set of key-value pairs, where the keys represent attribute names and the values represent expressions.

```html
<div data-attr="{'aria-label': $foo, disabled: $bar}"></div>
```

### `data-bind` #

Creates a signal (if one doesn’t already exist) and sets up two-way data binding between it and an element’s current bound state. When the signal changes, Datastar writes that value to the element. When one of the bind events fires, Datastar reads the element’s current bound property/value and writes that back to the signal.

The `data-bind` attribute can be placed on any HTML element on which data can be input or choices selected (`input`, `select`, `textarea` elements, and web components). Native elements use their built-in bind semantics automatically. Generic custom elements default to binding through `value` and listening on `change`.

`data-bind` does **not** inspect the event payload. It only uses the configured event as a signal to re-read the element’s current bound property/value. If you need to pull data from `event` itself, use `data-on:*` instead.

```html
<input data-bind:foo />
```

The signal name can be specified in the key (as above), or in the value (as below). This can be useful depending on the templating language you are using.

```html
<input data-bind="foo" />
```

Attribute casing rules apply to the signal name.

```html
<!-- Both of these create the signal `$fooBar` -->
<input data-bind:foo-bar />
<input data-bind="fooBar" />
```

The initial value of the signal is set to the value of the element, unless a signal has already been defined. So in the example below, `$fooBar` is set to `baz`.

```html
<input data-bind:foo-bar value="baz" />
```

Whereas in the example below, `$fooBar` inherits the value `fizz` of the predefined signal.

```html
<div data-signals:foo-bar="'fizz'">
    <input data-bind:foo-bar value="baz" />
</div>
```

#### Predefined Signal Types

When you predefine a signal, its **type** is preserved during binding. Whenever the element’s value changes, the signal value is automatically converted to match the original type. 

For example, in the code below, `$fooBar` is set to the **number** `10` (not the string `"10"`) when the option is selected.

```html
<div data-signals:foo-bar="0">
    <select data-bind:foo-bar>
        <option value="10">10</option>
    </select>
</div>
```

In the same way, you can assign multiple input values to a single signal by predefining it as an **array**. In the example below, `$fooBar` becomes `["fizz", "baz"]` when both checkboxes are checked, and `["", ""]` when neither is checked.

```html
<div data-signals:foo-bar="[]">
    <input data-bind:foo-bar type="checkbox" value="fizz" />
    <input data-bind:foo-bar type="checkbox" value="baz" />
</div>
```

#### File Uploads

Input fields of type `file` will automatically encode file contents in base64. This means that a form is not required.

```html
<input type="file" data-bind:files multiple />
```

The resulting signal is in the format `{ name: string, contents: string, mime: string }[]`. See the [file upload example](https://data-star.dev/examples/file_upload).

> If you want files to be uploaded to the server, rather than be converted to signals, use a form and with `multipart/form-data` in the [`enctype`](https://developer.mozilla.org/en-US/docs/Web/API/HTMLFormElement/enctype) attribute. See the [backend actions](https://data-star.dev/reference/actions#backend-actions) reference.

#### Modifiers

Modifiers allow you to modify behavior when binding signals using a key.

  * `__case` – Converts the casing of the signal name.
    * `.camel` – Camel case: `mySignal` (default)
    * `.kebab` – Kebab case: `my-signal`
    * `.snake` – Snake case: `my_signal`
    * `.pascal` – Pascal case: `MySignal`
  * `__prop` – Binds to a specific property instead of the default binding. Must _not_ be a read-only property.
    * Example: `data-bind:is-checked__prop.checked`
  * `__event` – Defines which events sync the element property back to the signal.
    * Example: `data-bind:query__event.input.change`

Native form controls use their built-in binding semantics automatically. Generic custom elements default to `value` and `change`. Use `__prop` and `__event` when a custom element’s live state is stored somewhere else.

```html
<my-toggle data-bind:is-checked__prop.checked__event.change></my-toggle>
```

### `data-class` #

Adds or removes a class to or from an element based on an expression.

```html
<div data-class:font-bold="$foo == 'strong'"></div>
```

If the expression evaluates to `true`, the `font-bold` class is added to the element; otherwise, it is removed.

The `data-class` attribute can also be used to add or remove multiple classes from an element using a set of key-value pairs, where the keys represent class names and the values represent expressions.

```html
<div data-class="{success: $foo != '', 'font-bold': $foo == 'strong'}"></div>
```

#### Modifiers

Modifiers allow you to modify behavior when defining a class name using a key.

  * `__case` – Converts the casing of the class.
    * `.camel` – Camel case: `myClass`
    * `.kebab` – Kebab case: `my-class` (default)
    * `.snake` – Snake case: `my_class`
    * `.pascal` – Pascal case: `MyClass`

```html
<div data-class:my-class__case.camel="$foo"></div>
```

### `data-computed` #

Creates a signal that is computed based on an expression. The computed signal is read-only, and its value is automatically updated when any signals in the expression are updated.

```html
<div data-computed:foo="$bar + $baz"></div>
```

Computed signals are useful for memoizing expressions containing other signals. Their values can be used in other expressions.

```html
<div data-computed:foo="$bar + $baz"></div>
<div data-text="$foo"></div>
```

> Computed signal expressions must not be used for performing actions (changing other signals, actions, JavaScript functions, etc.). If you need to perform an action in response to a signal change, use the `data-effect` attribute.

The `data-computed` attribute can also be used to create computed signals using a set of key-value pairs, where the keys represent signal names and the values are callables (usually arrow functions) that return a reactive value.

```html
<div data-computed="{foo: () => $bar + $baz}"></div>
```

#### Modifiers

Modifiers allow you to modify behavior when defining computed signals using a key.

  * `__case` – Converts the casing of the signal name.
    * `.camel` – Camel case: `mySignal` (default)
    * `.kebab` – Kebab case: `my-signal`
    * `.snake` – Snake case: `my_signal`
    * `.pascal` – Pascal case: `MySignal`

```html
<div data-computed:my-signal__case.kebab="$bar + $baz"></div>
```

### `data-effect` #

Executes an expression on page load and whenever any signals in the expression change. This is useful for performing side effects, such as updating other signals, making requests to the backend, or manipulating the DOM.

```html
<div data-effect="$foo = $bar + $baz"></div>
```

### `data-ignore` #

Datastar walks the entire DOM and applies plugins to each element it encounters. It’s possible to tell Datastar to ignore an element and its descendants by placing a `data-ignore` attribute on it. This can be useful for preventing naming conflicts with third-party libraries, or when you are unable to [escape user input](https://data-star.dev/reference/security#escape-user-input).

```html
<div data-ignore data-show-thirdpartylib="">
    <div>
        Datastar will not process this element.
    </div>
</div>
```

#### Modifiers

  * `__self` – Only ignore the element itself, not its descendants.

### `data-ignore-morph` #

Similar to the `data-ignore` attribute, the `data-ignore-morph` attribute tells the `PatchElements` watcher to skip processing an element and its children when morphing elements.

```html
<div data-ignore-morph>
    This element will not be morphed.
</div>
```

> To remove the `data-ignore-morph` attribute from an element, simply patch the element with the `data-ignore-morph` attribute removed.

### `data-indicator` #

Creates a signal and sets its value to `true` while a fetch request is in flight, otherwise `false`. The signal can be used to show a loading indicator.

```html
<button data-on:click="@get('/endpoint')"
        data-indicator:fetching
></button>
```

This can be useful for showing a loading spinner, disabling a button, etc.

```html
<button data-on:click="@get('/endpoint')"
        data-indicator:fetching
        data-attr:disabled="$fetching"
></button>
<div data-show="$fetching">Loading...</div>
```

The signal name can be specified in the key (as above), or in the value (as below). This can be useful depending on the templating language you are using.

```html
<button data-indicator="fetching"></button>
```

When using `data-indicator` with a fetch request initiated in a `data-init` attribute, you should ensure that the indicator signal is created before the fetch request is initialized.

```html
<div data-indicator:fetching data-init="@get('/endpoint')"></div>
```

#### Modifiers

Modifiers allow you to modify behavior when defining indicator signals using a key.

  * `__case` – Converts the casing of the signal name.
    * `.camel` – Camel case: `mySignal` (default)
    * `.kebab` – Kebab case: `my-signal`
    * `.snake` – Snake case: `my_signal`
    * `.pascal` – Pascal case: `MySignal`

### `data-init` #

Runs an expression when the attribute is initialized. This can happen on page load, when an element is patched into the DOM, and any time the attribute is modified (via a backend action or otherwise).

> The expression contained in the `data-init` attribute is executed when the element attribute is loaded into the DOM. This can happen on page load, when an element is patched into the DOM, and any time the attribute is modified (via a backend action or otherwise).

```html
<div data-init="$count = 1"></div>
```

#### Modifiers

Modifiers allow you to add a delay to the event listener.

  * `__delay` – Delay the event listener.
    * `.500ms` – Delay for 500 milliseconds (accepts any integer).
    * `.1s` – Delay for 1 second (accepts any integer).
  * `__viewtransition` – Wraps the expression in `document.startViewTransition()` when the View Transition API is available.

```html
<div data-init__delay.500ms="$count = 1"></div>
```

### `data-json-signals` #

Sets the text content of an element to a reactive JSON stringified version of signals. Useful when troubleshooting an issue.

```html
<!-- Display all signals -->
<pre data-json-signals></pre>
```

You can optionally provide a filter object to include or exclude specific signals using regular expressions.

```html
<!-- Only show signals that include "user" in their path -->
<pre data-json-signals="{include: /user/}"></pre>

<!-- Show all signals except those ending in "temp" -->
<pre data-json-signals="{exclude: /temp$/}"></pre>

<!-- Combine include and exclude filters -->
<pre data-json-signals="{include: /^app/, exclude: /password/}"></pre>
```

#### Modifiers

Modifiers allow you to modify the output format.

  * `__terse` – Outputs a more compact JSON format without extra whitespace. Useful for displaying filtered data inline.

```html
<!-- Display filtered signals in a compact format -->
<pre data-json-signals__terse="{include: /counter/}"></pre>
```

### `data-on` #

Attaches an event listener to an element, executing an expression whenever the event is triggered.

```html
<button data-on:click="$foo = ''">Reset</button>
```

An `evt` variable that represents the event object is available in the expression.

```html
<div data-on:my-event="$foo = evt.detail"></div>
```

The `data-on` attribute works with [events](https://developer.mozilla.org/en-US/docs/Web/Events) and [custom events](https://developer.mozilla.org/en-US/docs/Web/Events/Creating_and_triggering_events). The `data-on:submit` event listener prevents the default submission behavior of forms.

#### Modifiers

Modifiers allow you to modify behavior when events are triggered. Some modifiers have tags to further modify the behavior.

  * `__once` * – Only trigger the event listener once.
  * `__passive` * – Do not call `preventDefault` on the event listener.
  * `__capture` * – Use a capture event listener.
  * `__case` – Converts the casing of the event.
    * `.camel` – Camel case: `myEvent`
    * `.kebab` – Kebab case: `my-event` (default)
    * `.snake` – Snake case: `my_event`
    * `.pascal` – Pascal case: `MyEvent`
  * `__delay` – Delay the event listener.
    * `.500ms` – Delay for 500 milliseconds (accepts any integer).
    * `.1s` – Delay for 1 second (accepts any integer).
  * `__debounce` – Debounce the event listener.
    * `.500ms` – Debounce for 500 milliseconds (accepts any integer).
    * `.1s` – Debounce for 1 second (accepts any integer).
    * `.leading` – Debounce with leading edge (must come after timing).
    * `.notrailing` – Debounce without trailing edge (must come after timing).
  * `__throttle` – Throttle the event listener.
    * `.500ms` – Throttle for 500 milliseconds (accepts any integer).
    * `.1s` – Throttle for 1 second (accepts any integer).
    * `.noleading` – Throttle without leading edge (must come after timing).
    * `.trailing` – Throttle with trailing edge (must come after timing).
  * `__viewtransition` – Wraps the expression in `document.startViewTransition()` when the View Transition API is available.
  * `__window` – Attaches the event listener to the `window` element.
  * `__document` – Attaches the event listener to the `document` element. Useful for events that are only available on `document` and that do not bubble.
  * `__outside` – Triggers when the event is outside the element.
  * `__prevent` – Calls `preventDefault` on the event listener.
  * `__stop` – Calls `stopPropagation` on the event listener.

_* Only works with built-in events._

```html
<button data-on:click__window__debounce.500ms.leading="$foo = ''"></button>
<div data-on:my-event__case.camel="$foo = ''"></div>
```

### `data-on-intersect` #

Runs an expression when the element intersects with the viewport.

```html
<div data-on-intersect="$intersected = true"></div>
```

#### Modifiers

Modifiers allow you to modify the element intersection behavior and the timing of the event listener.

  * `__once` – Only triggers the event once.
  * `__exit` – Only triggers the event when the element exits the viewport.
  * `__half` – Triggers when half of the element is visible.
  * `__full` – Triggers when the full element is visible.
  * `__threshold` – Triggers when the element is visible by a certain percentage.
    * `.25` – Triggers when 25% of the element is visible.
    * `.75` – Triggers when 75% of the element is visible.
  * `__delay` – Delay the event listener.
    * `.500ms` – Delay for 500 milliseconds (accepts any integer).
    * `.1s` – Delay for 1 second (accepts any integer).
  * `__debounce` – Debounce the event listener.
    * `.500ms` – Debounce for 500 milliseconds (accepts any integer).
    * `.1s` – Debounce for 1 second (accepts any integer).
    * `.leading` – Debounce with leading edge (must come after timing).
    * `.notrailing` – Debounce without trailing edge (must come after timing).
  * `__throttle` – Throttle the event listener.
    * `.500ms` – Throttle for 500 milliseconds (accepts any integer).
    * `.1s` – Throttle for 1 second (accepts any integer).
    * `.noleading` – Throttle without leading edge (must come after timing).
    * `.trailing` – Throttle with trailing edge (must come after timing).
  * `__viewtransition` – Wraps the expression in `document.startViewTransition()` when the View Transition API is available.

```html
<div data-on-intersect__once__full="$fullyIntersected = true"></div>
```

### `data-on-interval` #

Runs an expression at a regular interval. The interval duration defaults to one second and can be modified using the `__duration` modifier.

```html
<div data-on-interval="$count++"></div>
```

#### Modifiers

Modifiers allow you to modify the interval duration.

  * `__duration` – Sets the interval duration.
    * `.500ms` – Interval duration of 500 milliseconds (accepts any integer).
    * `.1s` – Interval duration of 1 second (default).
    * `.leading` – Execute the first interval immediately.
  * `__viewtransition` – Wraps the expression in `document.startViewTransition()` when the View Transition API is available.

```html
<div data-on-interval__duration.500ms="$count++"></div>
```

### `data-on-signal-patch` #

Runs an expression whenever any signals are patched. This is useful for tracking changes, updating computed values, or triggering side effects when data updates.

```html
<div data-on-signal-patch="console.log('A signal changed!')"></div>
```

The `patch` variable is available in the expression and contains the signal patch details.

```html
<div data-on-signal-patch="console.log('Signal patch:', patch)"></div>
```

You can filter which signals to watch using the `data-on-signal-patch-filter` attribute.

#### Modifiers

Modifiers allow you to modify the timing of the event listener.

  * `__delay` – Delay the event listener.
    * `.500ms` – Delay for 500 milliseconds (accepts any integer).
    * `.1s` – Delay for 1 second (accepts any integer).
  * `__debounce` – Debounce the event listener.
    * `.500ms` – Debounce for 500 milliseconds (accepts any integer).
    * `.1s` – Debounce for 1 second (accepts any integer).
    * `.leading` – Debounce with leading edge (must come after timing).
    * `.notrailing` – Debounce without trailing edge (must come after timing).
  * `__throttle` – Throttle the event listener.
    * `.500ms` – Throttle for 500 milliseconds (accepts any integer).
    * `.1s` – Throttle for 1 second (accepts any integer).
    * `.noleading` – Throttle without leading edge (must come after timing).
    * `.trailing` – Throttle with trailing edge (must come after timing).

```html
<div data-on-signal-patch__debounce.500ms="doSomething()"></div>
```

### `data-on-signal-patch-filter` #

Filters which signals to watch when using the `data-on-signal-patch` attribute.

The `data-on-signal-patch-filter` attribute accepts an object with `include` and/or `exclude` properties that are regular expressions.

```html
<!-- Only react to counter signal changes -->
<div data-on-signal-patch-filter="{include: /^counter$/}"></div>

<!-- React to all changes except those ending with "changes" -->
<div data-on-signal-patch-filter="{exclude: /changes$/}"></div>

<!-- Combine include and exclude filters -->
<div data-on-signal-patch-filter="{include: /user/, exclude: /password/}"></div>
```

### `data-preserve-attr` #

Preserves the value of an attribute when morphing DOM elements.

```html
<details open data-preserve-attr="open">
    <summary>Title</summary>
    Content
</details>
```

You can preserve multiple attributes by separating them with a space.

```html
<details open class="foo" data-preserve-attr="open class">
    <summary>Title</summary>
    Content
</details>
```

### `data-ref` #

Creates a new signal that is a reference to the element on which the data attribute is placed.

```html
<div data-ref:foo></div>
```

The signal name can be specified in the key (as above), or in the value (as below). This can be useful depending on the templating language you are using.

```html
<div data-ref="foo"></div>
```

The signal value can then be used to reference the element.

```
$foo is a reference to a <span data-text="$foo.tagName"></span> element
```

#### Modifiers

Modifiers allow you to modify behavior when defining references using a key.

  * `__case` – Converts the casing of the signal name.
    * `.camel` – Camel case: `mySignal` (default)
    * `.kebab` – Kebab case: `my-signal`
    * `.snake` – Snake case: `my_signal`
    * `.pascal` – Pascal case: `MySignal`

```html
<div data-ref:my-signal__case.kebab></div>
```

### `data-show` #

Shows or hides an element based on whether an expression evaluates to `true` or `false`. For anything with custom requirements, use `data-class` instead.

```html
<div data-show="$foo"></div>
```

To prevent flickering of the element before Datastar has processed the DOM, you can add a `display: none` style to the element to hide it initially.

```html
<div data-show="$foo" style="display: none"></div>
```

### `data-signals` #

Patches (adds, updates or removes) one or more signals into the existing signals. Values defined later in the DOM tree override those defined earlier.

```html
<div data-signals:foo="1"></div>
```

Signals can be nested using dot-notation.

```html
<div data-signals:foo.bar="1"></div>
```

The `data-signals` attribute can also be used to patch multiple signals using a set of key-value pairs, where the keys represent signal names and the values represent expressions.

```html
<div data-signals="{foo: {bar: 1, baz: 2}}"></div>
```

The value above is written in JavaScript object notation, but JSON, which is a subset and which most templating languages have built-in support for, is also allowed.

Setting a signal’s value to `null` or `undefined` removes the signal.

```html
<div data-signals="{foo: null}"></div>
```

Keys used in `data-signals:*` are converted to camel case, so the signal name `mySignal` must be written as `data-signals:my-signal` or `data-signals="{mySignal: 1}"`.

Signals beginning with an underscore are _not_ included in requests to the backend by default. You can opt to include them by modifying the value of the [`filterSignals`](https://data-star.dev/reference/actions#filterSignals) option.

> Signal names cannot begin with nor contain a double underscore (`__`), due to its use as a modifier delimiter.

#### Modifiers

Modifiers allow you to modify behavior when patching signals using a key.

  * `__case` – Converts the casing of the signal name.
    * `.camel` – Camel case: `mySignal` (default)
    * `.kebab` – Kebab case: `my-signal`
    * `.snake` – Snake case: `my_signal`
    * `.pascal` – Pascal case: `MySignal`
  * `__ifmissing` – Only patches signals if their keys do not already exist. This is useful for setting defaults without overwriting existing values.

```html
<div data-signals:my-signal__case.kebab="1"
     data-signals:foo__ifmissing="1"
></div>
```

### `data-style` #

Sets the value of inline CSS styles on an element based on an expression, and keeps them in sync.

```html
<div data-style:display="$hiding && 'none'"></div>
<div data-style:background-color="$red ? 'red' : 'blue'"></div>
```

The `data-style` attribute can also be used to set multiple style properties on an element using a set of key-value pairs, where the keys represent CSS property names and the values represent expressions.

```html
<div data-style="{
    display: $hiding ? 'none' : 'flex',
    'background-color': $red ? 'red' : 'green'
}"></div>
```

Empty string, `null`, `undefined`, or `false` values will restore the original inline style value if one existed, or remove the style property if there was no initial value. This allows you to use the logical AND operator (`&&`) for conditional styles: `$condition && 'value'` will apply the style when the condition is true and restore the original value when false.

```html
<!-- When $x is false, color remains red from inline style -->
<div style="color: red;" data-style:color="$x && 'green'"></div>

<!-- When $hiding is true, display becomes none; when false, reverts to flex from inline style -->
<div style="display: flex;" data-style:display="$hiding && 'none'"></div>
```

The plugin tracks initial inline style values and restores them when data-style expressions become falsy or during cleanup. This ensures existing inline styles are preserved and only the dynamic changes are managed by Datastar.

### `data-text` #

Binds the text content of an element to an expression.

```html
<div data-text="$foo"></div>
```

## Pro Attributes #

The Pro attributes add functionality to the free open source Datastar framework. These attributes are available under a [commercial license](https://data-star.dev/pro#license) that helps fund our open source work.

### `data-animate` #[Pro](https://data-star.dev/pro "Datastar Pro")

Allows you to animate element attributes over time. Animated attributes are updated reactively whenever signals used in the expression change.

### `data-custom-validity` #[Pro](https://data-star.dev/pro "Datastar Pro")

Allows you to add custom validity to an element using an expression. The expression must evaluate to a string that will be set as the custom validity message. If the string is empty, the input is considered valid. If the string is non-empty, the input is considered invalid and the string is used as the reported message.

```html
<form>
    <input data-bind:foo name="foo" />
    <input data-bind:bar name="bar"
           data-custom-validity="$foo === $bar ? '' : 'Values must be the same.'"
    />
    <button>Submit form</button>
</form>
```

### `data-match-media` #[Pro](https://data-star.dev/pro "Datastar Pro")

Sets a signal to whether a media query matches and keeps it in sync whenever the query changes.

```html
<div
    data-match-media:is-dark="'prefers-color-scheme: dark'"
    data-computed:theme="$isDark ? 'dark' : 'light'"
></div>
```

The query value can be written as `prefers-color-scheme: dark` or `(prefers-color-scheme: dark)`, with or without surrounding quotes.

For more complex queries, pass a quoted query string with explicit media-query syntax (including parentheses) exactly as you want it evaluated by `window.matchMedia`.

See the [match media example](https://data-star.dev/examples/match_media).

#### Modifiers

Modifiers allow you to modify behavior when defining signals using a key.

  * `__case` – Converts the casing of the signal name.
    * `.camel` – Camel case: `mySignal` (default)
    * `.kebab` – Kebab case: `my-signal`
    * `.snake` – Snake case: `my_signal`
    * `.pascal` – Pascal case: `MySignal`

```html
<div data-match-media:is-dark__case.kebab="'prefers-color-scheme: dark'"></div>
```

### `data-on-raf` #[Pro](https://data-star.dev/pro "Datastar Pro")

Runs an expression on every [`requestAnimationFrame`](https://developer.mozilla.org/en-US/docs/Web/API/Window/requestAnimationFrame) event.

```html
<div data-on-raf="$count++"></div>
```

#### Modifiers

Modifiers allow you to modify the timing of the event listener.

  * `__throttle` – Throttle the event listener.
    * `.500ms` – Throttle for 500 milliseconds (accepts any integer).
    * `.1s` – Throttle for 1 second (accepts any integer).
    * `.noleading` – Throttle without leading edge (must come after timing).
    * `.trailing` – Throttle with trailing edge (must come after timing).

```html
<div data-on-raf__throttle.10ms="$count++"></div>
```

### `data-on-resize` #[Pro](https://data-star.dev/pro "Datastar Pro")

Runs an expression whenever an element’s dimensions change.

```html
<div data-on-resize="$count++"></div>
```

#### Modifiers

Modifiers allow you to modify the timing of the event listener.

  * `__debounce` – Debounce the event listener.
    * `.500ms` – Debounce for 500 milliseconds (accepts any integer).
    * `.1s` – Debounce for 1 second (accepts any integer).
    * `.leading` – Debounce with leading edge (must come after timing).
    * `.notrailing` – Debounce without trailing edge (must come after timing).
  * `__throttle` – Throttle the event listener.
    * `.500ms` – Throttle for 500 milliseconds (accepts any integer).
    * `.1s` – Throttle for 1 second (accepts any integer).
    * `.noleading` – Throttle without leading edge (must come after timing).
    * `.trailing` – Throttle with trailing edge (must come after timing).

```html
<div data-on-resize__debounce.10ms="$count++"></div>
```

### `data-persist` #[Pro](https://data-star.dev/pro "Datastar Pro")

Persists signals in [local storage](https://developer.mozilla.org/en-US/docs/Web/API/Window/localStorage). This is useful for storing values between page loads.

```html
<div data-persist></div>
```

The signals to be persisted can be filtered by providing a value that is an object with `include` and/or `exclude` properties that are regular expressions.

```html
<div data-persist="{include: /foo/, exclude: /bar/}"></div>
```

You can use a custom storage key by adding it after `data-persist:`. By default, signals are stored using the key `datastar`.

```html
<div data-persist:mykey></div>
```

#### Modifiers

Modifiers allow you to modify the storage target.

  * `__session` – Persists signals in [session storage](https://developer.mozilla.org/en-US/docs/Web/API/Window/sessionStorage) instead of local storage.

```html
<!-- Persists signals using a custom key `mykey` in session storage -->
<div data-persist:mykey__session></div>
```

### `data-query-string` #[Pro](https://data-star.dev/pro "Datastar Pro")

Syncs query string params to signal values on page load, and syncs signal values to query string params on change.

```html
<div data-query-string></div>
```

The signals to be synced can be filtered by providing a value that is an object with `include` and/or `exclude` properties that are regular expressions.

```html
<div data-query-string="{include: /foo/, exclude: /bar/}"></div>
```

#### Modifiers

Modifiers allow you to enable history support.

  * `__filter` – Filters out empty values when syncing signal values to query string params.
  * `__history` – Enables history support – each time a matching signal changes, a new entry is added to the browser’s history stack. Signal values are restored from the query string params on popstate events.

```html
<div data-query-string__filter__history></div>
```

### `data-replace-url` #[Pro](https://data-star.dev/pro "Datastar Pro")

Replaces the URL in the browser without reloading the page. The value can be a relative or absolute URL, and is an evaluated expression.

```html
<div data-replace-url="`/page${page}`"></div>
```

### `data-scroll-into-view` #[Pro](https://data-star.dev/pro "Datastar Pro")

Scrolls the element into view. Useful when updating the DOM from the backend, and you want to scroll to the new content.

```html
<div data-scroll-into-view></div>
```

#### Modifiers

Modifiers allow you to modify scrolling behavior.

  * `__smooth` – Scrolling is animated smoothly.
  * `__instant` – Scrolling is instant.
  * `__auto` – Scrolling is determined by the computed `scroll-behavior` CSS property.
  * `__hstart` – Scrolls to the left of the element.
  * `__hcenter` – Scrolls to the horizontal center of the element.
  * `__hend` – Scrolls to the right of the element.
  * `__hnearest` – Scrolls to the nearest horizontal edge of the element.
  * `__vstart` – Scrolls to the top of the element.
  * `__vcenter` – Scrolls to the vertical center of the element.
  * `__vend` – Scrolls to the bottom of the element.
  * `__vnearest` – Scrolls to the nearest vertical edge of the element.
  * `__focus` – Focuses the element after scrolling.

### `data-view-transition` #[Pro](https://data-star.dev/pro "Datastar Pro")

Sets the `view-transition-name` style attribute explicitly.

```html
<div data-view-transition="$foo"></div>
```

Page level transitions are automatically handled by an injected meta tag. Inter-page elements are automatically transitioned if the [View Transition API](https://developer.mozilla.org/en-US/docs/Web/API/View_Transitions_API) is available in the browser and `useViewTransitions` is `true`.

## Attribute Evaluation Order #

Elements are evaluated by walking the DOM in a depth-first manner, and attributes are applied in the order they appear in the element. This is important in some cases, such as when using `data-indicator` with a fetch request initiated in a `data-init` attribute, in which the indicator signal must be created before the fetch request is initialized.

```html
<div data-indicator:fetching data-init="@get('/endpoint')"></div>
```

Data attributes are evaluated and applied on page load (after Datastar has initialized), and are reapplied after any DOM patches that add, remove, or change them. Note that [morphing elements](https://data-star.dev/reference/sse_events#datastar-patch-elements) preserves existing attributes unless they are explicitly changed in the DOM, meaning they will only be reapplied if the attribute itself is changed.

## Attribute Casing #

[According to the HTML spec](https://developer.mozilla.org/en-US/docs/Web/HTML/Global_attributes/data-*), all `data-*` attributes (not Datastar the framework, but any time a data attribute appears in the DOM) are case-insensitive. When Datastar processes these attributes, hyphenated names are automatically converted to [camel case](https://developer.mozilla.org/en-US/docs/Glossary/Camel_case) by removing hyphens and uppercasing the letter following each hyphen.

Datastar handles casing of data attribute key suffixes containing hyphens in two ways:

  1. The keys used in attributes that define signals (`data-bind:*`, `data-signals:*`, `data-computed:*`, etc.), are converted to camel case (the recommended casing for signals) by removing hyphens and uppercasing the letter following each hyphen. For example, `data-signals:my-signal` defines a signal named `mySignal`, and you would use the signal in a [Datastar expression](https://data-star.dev/guide/datastar_expressions) as `$mySignal`.
  2. The keys suffixes used by all other attributes are, by default, converted to [kebab case](https://developer.mozilla.org/en-US/docs/Glossary/Kebab_case). For example, `data-class:text-blue-700` adds or removes the class `text-blue-700`, and `data-on:rocket-launched` would react to the event named `rocket-launched`.

You can use the `__case` modifier to convert between `camelCase`, `kebab-case`, `snake_case`, and `PascalCase`, or alternatively use object syntax when available.

For example, if listening for an event called `widgetLoaded`, you would use `data-on:widget-loaded__case.camel`.

## Aliasing Attributes #

It is possible to alias `data-*` attributes to a custom alias (`data-alias-*`, for example) using the [bundler](https://data-star.dev/pro/bundler). A custom alias should _only_ be used if you have a conflict with a legacy library and `data-ignore` cannot be used.

We maintain a `data-star-*` aliased version that can be included as follows.

```html
<script type="module" src="https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.3/bundles/datastar-aliased.js"></script>
```

## Datastar Expressions #

Datastar expressions used in `data-*` attributes parse signals, converting all dollar signs followed by valid signal name characters into their corresponding signal values. Expressions support standard JavaScript syntax, including operators, function calls, ternary expressions, and object and array literals.

A variable `el` is available in every Datastar expression, representing the element that the attribute exists on.

```html
<div id="bar" data-text="$foo + el.id"></div>
```

Read more about [Datastar expressions](https://data-star.dev/guide/datastar_expressions) in the guide.

## Error Handling #

Datastar has built-in error handling and reporting for runtime errors. When a data attribute is used incorrectly, for example `data-text-foo`, the following error message is logged to the browser console.

```
Uncaught datastar runtime error: textKeyNotAllowed
More info: https://data-star.dev/errors/key_not_allowed?metadata=%7B%22plugin%22%3A%7B%22name%22%3A%22text%22%2C%22type%22%3A%22attribute%22%7D%2C%22element%22%3A%7B%22id%22%3A%22%22%2C%22tag%22%3A%22DIV%22%7D%2C%22expression%22%3A%7B%22rawKey%22%3A%22textFoo%22%2C%22key%22%3A%22foo%22%2C%22value%22%3A%22%22%2C%22fnContent%22%3A%22%22%7D%7D
Context: {
    "plugin": {
        "name": "text",
        "type": "attribute"
    },
    "element": {
        "id": "",
        "tag": "DIV"
    },
    "expression": {
        "rawKey": "textFoo",
        "key": "foo",
        "value": "",
        "fnContent": ""
    }
}
```

The “More info” link takes you directly to a context-aware error page that explains the error and provides correct sample usage. See [the error page for the example above](https://data-star.dev/errors/key_not_allowed?metadata=%7B%22plugin%22%3A%7B%22name%22%3A%22text%22%2C%22type%22%3A%22attribute%22%7D%2C%22element%22%3A%7B%22id%22%3A%22%22%2C%22tag%22%3A%22DIV%22%7D%2C%22expression%22%3A%7B%22rawKey%22%3A%22textFoo%22%2C%22key%22%3A%22foo%22%2C%22value%22%3A%22%22%2C%22fnContent%22%3A%22%22%7D%7D), and all available error messages in the sidebar menu.

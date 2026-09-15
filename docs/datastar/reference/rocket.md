<!-- Source: https://data-star.dev/reference/rocket -->
<!-- Fetched: 2026-09-14 -->

# Rocket

Rocket is currently in beta.

## Overview #

Rocket is Datastar Pro’s web-component API. You define a custom element with `rocket(tagName, { ... })`, describe public props with codecs, put non-DOM instance behavior in `setup`, use `onFirstRender` only when work depends on rendered refs or mounted DOM, and return DOM from `render`.

Rocket is a JavaScript API built around the browser’s custom-element model, with Datastar handling reactivity, local signal scoping, action dispatch, and DOM application.

```
rocket('demo-counter', {
  mode: 'light',
  props: ({ number, string }) => ({
    count: number.step(1).min(0),
    label: string.trim.default('Counter'),
  }),
  setup: ({ $$, observeProps, props }) => {
    $$.count = props.count
    observeProps(() => {
      $$.count = props.count
    }, 'count')
  },
  render: ({ html, props: { count, label } }) => {
    return html`
      <div class="stack gap-2">
        <button
          type="button"
          data-on:click="$$count += 1"
          data-text="${label} + ': ' + $$count"
        ></button>
        <template data-if="$$count !== ${count}">
          <button type="button" data-on:click="$$count = ${count}">Reset</button>
        </template>
      </div>
    `
  },
})
```

The result is a real web component. Once defined, you use it like any other custom element.

```html
<script type="module">
  import { rocket } from '/bundles/datastar-pro.js'
  // define demo-counter here
</script>

<demo-counter count="5" label="Inventory"></demo-counter>
```

The examples on this page assume the same module pattern. Import `rocket` explicitly from the bundle module rather than relying on a global.

### `tag` #

`tag` is the first argument to `rocket(...)`. It must contain a hyphen, must be unique, and becomes the actual HTML tag users place in the page.

```
rocket(tag: string, options?: RocketDefinition<Defs>): void
```

```
rocket('demo-user-card', {
  props: ({ string }) => ({
    name: string.default('Anonymous'),
  }),
  render: ({ html, props: { name } }) => {
    return html`
        <p>${name}</p>
    `
  },
})
```

Rocket registers the element with `customElements.define`. Re-registering the same tag is ignored, which makes repeated module evaluation safe during development.

## Definition #

A Rocket definition describes one custom-element type. Each field controls a specific part of the browser component lifecycle.

Field| What it does  
---|---  
`mode`| Chooses light DOM, open shadow DOM, or closed shadow DOM.  
`props`| Defines public props, decoding, defaults, and attribute reflection.  
`setup`| Runs once per connected instance to create local state, prop observers, timers, host APIs, and cleanup.  
`onFirstRender`| Runs after the initial render and Datastar apply pass, when refs and rendered DOM exist.  
`render`| Returns the component DOM as Rocket `html` or `svg`.  
`renderOnPropChange`| Controls whether prop updates trigger rerendering.  
`manifest`| Adds slot and event metadata to Rocket’s generated component manifest.  
  
### `mode` #

`mode` chooses where the component renders.

```
mode?: 'open' | 'closed' | 'light'
```

Value| Mount target| When to use it  
---|---|---  
`'light'`| The host element itself.| Use when the component should participate directly in the page’s DOM and CSS.  
`'open'`| An open shadow root.| Use when you want style encapsulation but still want `element.shadowRoot`.  
`'closed'`| A closed shadow root.| Use when the internal DOM should stay fully encapsulated.  
  
If you omit `mode`, Rocket uses shadow DOM and defaults to `'open'`.

Rocket defaults to `'open'` because it gives you native slots and a normal shadow-root debugging surface without giving up the main styling mechanism components should rely on: CSS custom properties. CSS variables pierce the shadow boundary, so the usual pattern is to keep component structure encapsulated while letting parents theme it through `--tokens`.

### `props` #

`props` defines the component’s public API. Rocket calls it once at definition time and passes in the codec registry. The object you return becomes:

```
props?: (codecs: CodecRegistry) => Defs
```

  * The list of observed HTML attributes.
  * The normalization pipeline for incoming attribute values.
  * The property descriptors on the custom-element instance.
  * The typed `props` object passed to `setup` and `render`.

```
rocket('demo-progress', {
  props: ({ number, bool, string }) => ({
    value: number.clamp(0, 100),
    striped: bool,
    label: string.trim.default('Progress'),
  }),
  render: ({ html, props: { value, striped, label } }) => {
    return html`
      <div>
        <strong>${label}</strong>
        <div class="bar" data-style="{ width: ${value} + '%' }"></div>
        <span data-show="${striped}">Striped</span>
      </div>
    `
  },
})
```

`props` is optional. If you omit it, Rocket defines no observed attributes and `setup` / `render` receive an empty decoded props object.

Each declared prop already gets a normal element property accessor on the custom-element prototype. In other words, `host.value`, `host.checked`, and similar prop access work by default without per-instance `Object.defineProperty(...)` calls.

The intent of `props` is to give Rocket components a decoded props object that is always usable. Rocket codecs normalize incoming attribute values into valid decoded values instead of throwing on bad input.

On the DOM side, web-component attributes arrive as strings. Rocket converts those raw attribute strings into usable prop types for you before `setup` or `render` reads them.

This follows the same broad design instinct as Go and Odin zero values and the general behavior of native HTML elements: invalid or missing input should degrade to a sensible value, not crash component setup or rendering. A malformed number becomes a number, a missing string becomes a string, and defaults stay inside the codec layer instead of being rechecked throughout the component.

Prop names are written in JavaScript style and reflected to attributes in kebab case. A prop named `startDate` maps to an HTML attribute named `start-date`.

### Props vs Signals #

Default to `props` when data is part of the component’s public API. Props already match the native browser model: they map cleanly to attributes and element properties, reflect through the host element, and give outside code a normal way to configure the component.

Use local Rocket signals for internal reactive state and for imperative integration points where the component has to talk to something outside Rocket’s normal prop/render flow. Good examples are calling charting libraries, talking to third-party widgets, starting timers, or reacting to fetch results and other external async work.

A useful rule is: if a parent or page author should be able to set it directly on the element, make it a prop. If the value mainly exists so `setup` can coordinate external calls or internal UI state over time, make it a signal.

### `manifest` #

`manifest` lets you add documentation metadata that Rocket cannot infer from the DOM alone. Rocket already generates prop metadata from your codecs. Use `manifest` to document slots and events so tooling can describe the full public surface of the component.

```
manifest?: {
  slots?: Array<{
    name: string
    description?: string
  }>
  events?: Array<{
    name: string
    kind?: 'event' | 'custom-event'
    bubbles?: boolean
    composed?: boolean
    description?: string
  }>
}
```

```javascript
import { publishRocketManifests, rocket } from '/bundles/datastar-pro.js'

rocket('demo-dialog', {
  props: ({ string, bool }) => ({
    title: string.default('Dialog'),
    open: bool,
  }),
  manifest: {
    slots: [
      { name: 'default', description: 'Dialog body content.' },
      { name: 'footer', description: 'Action row content.' },
    ],
    events: [
      {
        name: 'close',
        kind: 'custom-event',
        bubbles: true,
        composed: true,
        description: 'Fired when the dialog requests dismissal.',
      },
    ],
  },
  render: ({ html, props: { title, open } }) => {
    return html`
      <section data-show="${open}">
        <header>${title}</header>
        <slot></slot>
        <footer><slot name="footer"></slot></footer>
      </section>
    `
  },
})

const manifest = customElements.get('demo-dialog')?.manifest?.()

await publishRocketManifests({
  endpoint: '/api/rocket/manifests',
})
```

The generated manifest includes one component entry per Rocket tag. Each entry contains the tag name, inferred prop metadata, and your manual slot and event metadata.

Each registered Rocket class also gets a static `manifest()` method that returns that component’s manifest entry. This is useful when you want to inspect or test one component locally without publishing the full document.

`publishRocketManifests(...)` posts the full manifest document as JSON. Rocket sorts components by tag and includes a top-level `version` and `generatedAt` timestamp so a docs build or registry service can store snapshots.

### `setup` #

`setup` runs once per connected element instance after Rocket creates the component scope and before the initial Datastar apply pass. This is the default place for local signals, computed state, prop observers, timers, cleanup handlers, and host APIs that do not depend on rendered refs.

If the code needs rendered DOM, measurements, focus targets, or `data-ref:*` handles, move that part to `onFirstRender()` instead of delaying it inside `setup()`.

```
setup?: (context: SetupContext<InferProps<Defs>>) => void
```

Datastar is mostly about setting up relationships, not manually pushing DOM updates. Most component behavior should come from local signals changing over time and the rest of the component reacting to those changes through effects, bindings, and render output.

```javascript
rocket('demo-timer', {
  props: ({ number, bool }) => ({
    intervalMs: number.min(50).default(1000),
    autoplay: bool,
  }),
  setup: ({ $$, cleanup, props, observeProps }) => {
    $$.seconds = 0
    let timerId = 0

    let syncTimer = () => {
      clearInterval(timerId)
      if (!props.autoplay) {
        return
      }
      timerId = window.setInterval(() => {
        $$.seconds += 1
      }, props.intervalMs)
    }

    syncTimer()
    observeProps(syncTimer)

    cleanup(() => clearInterval(timerId))
  },
  render: ({ html }) => {
    return html`
        <p data-text="$$seconds"></p>
    `
  },
})
```

Use `setup` to handle non-ref behavior. Keep markup creation in `render`, and keep ref-backed DOM work in `onFirstRender()`. That split matters because Rocket may rerun `render` many times, while `setup` and `onFirstRender` only run once per connected instance.

When behavior needs to react to prop changes, use `observeProps(...)`. The `props` object is normalized and always usable, but it is not itself a local signal source.

### `onFirstRender` #

`onFirstRender` runs once per connected instance after Rocket has finished the initial `render()`, Datastar `apply(...)`, and ref population pass. Use it for work that depends on rendered DOM or `data-ref:*` refs.

```
onFirstRender?: (context: SetupContext<InferProps<Defs>> & { refs: Record<string, any> }) => void
```

This is the right place for ref-backed host accessors, DOM measurements, focus management, or third-party widget setup that needs the actual rendered nodes. If a piece of logic would work without refs, keep it in `setup` instead.

`onFirstRender` receives the normal setup context plus `refs`, so it can use `$$`, `refs`, `overrideProp`, `defineHostProp`, `cleanup`, and the rest without nesting a second callback inside setup.

```javascript
rocket('demo-input-bridge', {
  props: ({ string }) => ({
    value: string.default(''),
  }),
  render: ({ html, props: { value } }) => html`
    <input data-ref:input value="${value}">
  `,
  onFirstRender: ({ overrideProp, refs }) => {
    overrideProp(
      'value',
      (getDefault) => refs.input?.value ?? getDefault(),
      (value, setDefault) => {
        const next = String(value ?? '')
        if (refs.input && refs.input.value !== next) refs.input.value = next
        setDefault(next)
      },
    )
  },
})
```

If setup code needs to force a render later, call `ctx.render` with an empty overrides object and any trailing args. That reruns the component render function with the current host, props, and template helpers, plus any extra arguments you pass.

This is not a replacement for local signals. Reach for `ctx.render` when async or imperative work needs a coarse structural patch, similar to switching a `data-if` branch. For high-frequency state like counters, form values, loading flags, or selection state, keep using signals and normal Datastar bindings.

```javascript
rocket('demo-user-card', {
  props: ({ string, number }) => ({
    userId: number.min(1),
    fallbackName: string.default('Unknown user'),
  }),
  setup: ({ cleanup, render, props }) => {
    let cancelled = false

    ;(async () => {
      try {
        const response = await fetch('/users/' + props.userId + '.json')
        const user = await response.json()
        if (!cancelled) {
          render({}, user, null)
        }
      } catch (error) {
        if (!cancelled) {
          render({}, null, error)
        }
      }
    })()

    cleanup(() => {
      cancelled = true
    })
  },
  render: ({ html, props: { fallbackName } }, user = null, error = null) => {
    if (error) {
      return html`<p>Failed to load user.</p>`
    }
    if (!user) {
      return html`<p>Loading user...</p>`
    }
    return html`
      <article>
        <h3>${user.name ?? fallbackName}</h3>
        <p>${user.email ?? 'No email provided'}</p>
      </article>
    `
  },
})
```

### `render` #

`render` is optional. It receives the normalized props, the host element, and two tagged-template helpers: `html` and `svg`. Return a Rocket tagged-template fragment, primitive text, an iterable of composed values, or `null`/`undefined`.

```
type RocketPrimitiveRenderValue =
  | string
  | number
  | boolean
  | bigint
  | Date
  | null
  | undefined

type RocketComposedRenderValue =
  | RocketPrimitiveRenderValue
  | Node
  | Iterable<RocketComposedRenderValue>

type RocketRenderValue =
  | DocumentFragment
  | RocketPrimitiveRenderValue
  | Iterable<RocketComposedRenderValue>

type RocketRender<Props extends Record<string, any>> = {
  (context: RenderContext<Props>): RocketRenderValue
  <A1>(context: RenderContext<Props>, a1: A1): RocketRenderValue
  <A1, A2, A3, A4, A5, A6, A7, A8>(
    context: RenderContext<Props>,
    a1: A1,
    a2: A2,
    a3: A3,
    a4: A4,
    a5: A5,
    a6: A6,
    a7: A7,
    a8: A8,
  ): RocketRenderValue
}

render?: RocketRender<InferProps<Defs>>
```

This is the method that turns Rocket from a state container into an actual web component. The host element stays stable, while the rendered subtree inside it or inside its shadow root is morphed from the output of `render`.

Rocket supports up to 8 typed trailing render arguments. Automatic renders call `render(context)`. Setup-driven renders can call `ctx.render` with an empty overrides object to pass those extra values explicitly.

Treat those manual render calls like coarse DOM branch updates, not like a second reactive state system. If the UI should keep updating as values change over time, model that state with signals and let Datastar update the existing DOM in place.

Inside Rocket `html` templates, attribute interpolation omits the attribute for `false`, `null`, and `undefined`, while `true` creates the empty-string form of a boolean attribute.

In normal data positions, `false`, `null`, and `undefined` render nothing. If you want the literal text `"false"` or `"true"` in the DOM, pass a string, not a boolean value.

If you omit `render`, Rocket still registers the custom element, runs `setup`, scopes host-owned children, and wires action dispatch, but it does not morph a rendered subtree.

### `renderOnPropChange` #

```
renderOnPropChange?:
  | boolean
  | ((context: {
      host: HTMLElement
      props: Props
      changes: Partial<Props>
    }) => boolean)
```

By default Rocket behaves as if this were `true`.

Rocket coalesces multiple prop updates in the same turn into a single queued `render()` call per component. Prop changes still update `props` synchronously and notify `observeProps()` listeners immediately; the queue only deduplicates the component DOM rerender step.

```
rocket('demo-chart', {
  props: ({ json, string }) => ({
    series: json.default(() => []),
    theme: string.default('light'),
  }),
  mode: 'light',
  renderOnPropChange: ({ changes }) => {
    return 'theme' in changes
  },
  setup: ({ host, observeProps, props }) => {
    observeProps(() => {
      drawChart(host, props.series, props.theme)
    }, 'series', 'theme')
  },
  render: ({ html }) => {
    return html`
        <canvas width="640" height="320"></canvas>
    `
  },
})
```

In that pattern, updating `series` still updates `props.series` and notifies `observeProps` listeners, but it skips DOM rerendering because the canvas is updated imperatively. Use `observeProps` for prop changes; use `effect` for local Rocket signals or other reactive Datastar state.

## Modes #

Rocket supports both light DOM and shadow DOM because real component systems need both.

  * Use `light` when the component should inherit page styles, participate in layout naturally, and expose its internals to outside CSS.
  * Use `open` when you want encapsulated styles but still need debugging access through `shadowRoot`.
  * Use `closed` when the component is a sealed implementation detail.

In shadow DOM, `<slot>` is the platform slot API. In light DOM, it is only a Rocket placeholder for host-child projection. Rocket supports default and named `<slot>` markers in light DOM, and if a slot receives no matching host children, its fallback content is rendered instead. This is still a Rocket runtime feature, not browser slotting.

```
rocket('demo-chip', {
  mode: 'light',
  props: ({ string }) => ({
    label: string.default('Chip'),
  }),
  render: ({ html, props: { label } }) => {
    return html`
        <span class="chip">${label}</span>
    `
  },
})

rocket('demo-modal-frame', {
  mode: 'open',
  props: ({ string }) => ({
    title: string.default('Dialog'),
  }),
  render: ({ html, props: { title } }) => {
    return html`
      <style>
        :host { display: block; }
        .frame { border: 1px solid #d4d4d8; padding: 1rem; }
      </style>
      <div class="frame">${title}</div>
    `
  },
})
```

## Props and Codecs #

Custom-element attributes arrive as strings. Rocket codecs turn those strings into useful values, apply normalization, supply defaults, and encode property writes back to attributes. This is what makes a Rocket component feel like a real typed component API instead of a stringly-typed DOM wrapper.

### How Props Flow #

Each prop has one codec. Rocket uses it in three places:

  * At construction time, to decode initial attributes into `props`.
  * When an observed attribute changes, to decode the new string value.
  * When code assigns to `element.someProp`, to encode that value back into an attribute.

```javascript
rocket('demo-badge', {
  props: ({ string, bool, number }) => ({
    label: string.trim.default('New'),
    tone: string.lower.default('neutral'),
    visible: bool.default(true),
    priority: number.clamp(0, 5),
  }),
  render: ({ html, props: { label, tone, visible, priority } }) => {
    return html`
      <span
        data-show="${visible}"
        data-attr:data-tone="${tone}"
        data-text="${label + ' #' + priority}">
      </span>
    `
  },
})

const badge = document.querySelector('demo-badge')

// Property write:
badge.priority = 7

// Reflected attribute after encoding:
// <demo-badge priority="5"></demo-badge>

// Attribute write:
badge.setAttribute('label', '  shipped  ')

// Decoded prop value in setup/render:
// props.label === 'shipped'
```

Defaults matter for component design because custom elements are often dropped into a page with incomplete markup. A default lets the component boot into a valid state without requiring every consumer to pass every attribute.

### Fluent Codec Pattern #

Codecs are immutable builders. Every method returns a new codec with an extra transform or constraint layered on top of the previous one. That is why the API is fluent.

```
props: ({ string, number, object, array, oneOf }) => ({
  slug: string.trim.lower.kebab.maxLength(48),
  progress: number.clamp(0, 100).step(5),
  theme: oneOf('light', 'dark', 'system').default('system'),
  tags: array(string.trim.lower),
  profile: object({
    name: string.trim.default('Anonymous'),
    age: number.min(0),
  }),
})
```

Read each chain left to right.

  * `string.trim.lower.kebab` means “decode as string, trim it, lowercase it, then convert it to kebab case.”
  * `number.clamp(0, 100).step(5)` means “decode as number, constrain it to 0-100, then snap it to increments of 5.”
  * `array(string.trim.lower)` means “decode a JSON array, then decode each item with the nested string codec.”

`default(...)` can appear anywhere in the chain, but putting it at the end reads best because it describes the final fallback value after the normalization pipeline has been fully defined.

### Custom Codecs #

You can provide your own codecs. In practice, `props` does not require that every value come from Rocket’s built-in registry. Any value that implements the codec contract can be returned from the `props` object.

```javascript
export type Codec<T> = {
  decode(value: unknown): T
  encode(value: T): string
}
```

Use `createCodec(...)` when you want Rocket to turn a plain decode/encode pair into a prop codec that behaves like the built-in ones.

`decode(value: unknown)` uses `unknown` on purpose. Raw custom-element attributes do arrive as strings, but Rocket also reuses codec decode paths for missing values, nested object and array members, and already-materialized JavaScript values. Using `unknown` keeps the contract honest: a codec should be able to normalize whatever input Rocket hands it, not just a string from HTML.

If a codec `decode(...)` throws, Rocket calls `console.warn(...)` and falls back to that codec’s default value instead. That applies to built-in codecs and to custom codecs returned from `createCodec(...)` or provided directly in `props`.

Most custom codecs should wrap an existing codec rather than starting from scratch. That lets you keep Rocket’s normal defaulting and attribute-reflection behavior while only changing the part that is specific to your domain.

```javascript
import { createCodec } from '/bundles/datastar-pro.js'

let myCodec = createCodec({
  decode(value) {
    // normalize input
  },
  encode(value) {
    // reflect back to attribute text
  },
})
```

```javascript
import { createCodec, rocket } from '/bundles/datastar-pro.js'

let percent = createCodec({
    decode(value) {
      let text = String(value ?? '').trim().replace(/%$/, '')
      let number = Number.parseFloat(text)
      return Number.isFinite(number)
        ? Math.max(0, Math.min(100, number))
        : 0
    },
    encode(value) {
      return String(Math.max(0, Math.min(100, value)))
    },
})

rocket('demo-meter', {
  props: ({ string }) => ({
    value: percent.default(50),
    label: string.trim.default('Progress'),
  }),
})
```

If you want the exported types, Rocket exposes `Codec` and `CodecRegistry` from the public module entrypoint.

### Codec Tables #

Rocket ships these codecs in the `props` registry.

Codec| Decoded type| Typical input| Typical uses  
---|---|---|---  
`string`| `string`| `" hello "`| Text props, labels, ids, classes, case-normalized names.  
`number`| `number`| `"42"`| Ranges, dimensions, timing, scores, percentages.  
`bool`| `boolean`| `""`, `"true"`, `"1"`| Feature flags and toggles.  
`date`| `Date`| `"2026-03-18T12:00:00.000Z"`| Timestamps and schedule props.  
`json`| `any`| `'{"items":[1,2,3]}'`| Structured JSON payloads.  
`js`| `any`| `"{ foo: 1, bar: [2, 3] }"`| JS-like object literals when strict JSON is inconvenient.  
`bin`| `Uint8Array`| text-like binary payload| Binary or byte-oriented props.  
`array(codec)`| `T[]`| `'["a","b"]'`| Lists of values with per-item normalization.  
`array(codecA, codecB, ...)`| Tuple| `'["en",10,true]'`| Fixed-length ordered values.  
`object(shape)`| Typed object| `'{"x":10,"y":20}'`| Named structured props.  
`oneOf(...)`| Union| `"primary"`| Enums and constrained variants.  
  
### `string` #

`string` is the most composable codec. It is useful on its own, but it also acts as a normalization pipeline any time a value eventually needs to become text.

Without an explicit `.default(...)`, the zero value is `""`.

Member| Effect| Example  
---|---|---  
`.trim`| Removes surrounding whitespace.| `" Ada "` becomes `"Ada"`.  
`.upper`| Uppercases the string.| `"ion"` becomes `"ION"`.  
`.lower`| Lowercases the string.| `"Rocket"` becomes `"rocket"`.  
`.kebab`| Converts to kebab case.| `"Demo Button"` becomes `"demo-button"`.  
`.camel`| Converts to camel case.| `"rocket button"` becomes `"rocketButton"`.  
`.snake`| Converts to snake case.| `"Rocket Button"` becomes `"rocket_button"`.  
`.pascal`| Converts to Pascal case.| `"rocket button"` becomes `"RocketButton"`.  
`.title`| Title-cases each word.| `"hello world"` becomes `"Hello World"`.  
`.prefix(value)`| Adds a prefix if missing.| `"42"` with `prefix('#')` becomes `"#42"`.  
`.suffix(value)`| Adds a suffix if missing.| `"24"` with `suffix('px')` becomes `"24px"`.  
`.maxLength(n)`| Truncates to `n` characters.| `"abcdef"` with `maxLength(4)` becomes `"abcd"`.  
`.default(value)`| Supplies a fallback string.| Missing values can become `"Anonymous"`.  
  
```
props: ({ string }) => ({
  slug: string.trim.lower.kebab.maxLength(48),
  cssSize: string.trim.suffix('px').default('16px'),
  title: string.trim.title.default('Untitled'),
})
```

### `number` #

`number` turns a prop into a numeric API and lets you enforce range, rounding, snapping, and remapping rules right in the prop definition.

Without an explicit `.default(...)`, the zero value is `0`.

Member| Effect| Example  
---|---|---  
`.min(value)`| Enforces a lower bound.| `-4` with `min(0)` becomes `0`.  
`.max(value)`| Enforces an upper bound.| `120` with `max(100)` becomes `100`.  
`.clamp(min, max)`| Applies both bounds.| `120` with `clamp(0, 100)` becomes `100`.  
`.step(step, base?)`| Snaps to the nearest increment.| `13` with `step(5)` becomes `15`.  
`.round`| Rounds to the nearest integer.| `3.6` becomes `4`.  
`.ceil(decimals?)`| Rounds up with optional decimal precision.| `1.231` with `ceil(2)` becomes `1.24`.  
`.floor(decimals?)`| Rounds down with optional decimal precision.| `1.239` with `floor(2)` becomes `1.23`.  
`.fit(inMin, inMax, outMin, outMax, clamped?, rounded?)`| Maps one numeric range into another.| `50` from `0-100` into `0-1` becomes `0.5`.  
`.default(value)`| Supplies a fallback number.| Missing values can become `0` or `1`.  
  
```
props: ({ number }) => ({
  width: number.min(0).default(320),
  opacity: number.clamp(0, 1).ceil(2).default(1),
  progress: number.clamp(0, 100).step(5),
  normalizedX: number.fit(0, 1920, 0, 1, true, false),
})
```

### `bool` #

`bool` decodes common truthy attribute forms into a boolean. Empty-string attributes such as `<demo-dialog open>` decode to `true`.

Without an explicit `.default(...)`, the zero value is `false`.

Member| Effect| Notes  
---|---|---  
`.default(value)`| Supplies the fallback boolean.| `true`, `false`, or a factory function.  
  
```
props: ({ bool }) => ({
  open: bool,
  disabled: bool,
  elevated: bool.default(true),
})
```

### `date` #

`date` decodes a prop into a `Date`. Invalid input falls back to a valid date object rather than leaving the component with an unusable value.

Without an explicit `.default(...)`, the zero value is a fresh valid `Date` created at decode time.

Member| Effect| Notes  
---|---|---  
`.default(value)`| Supplies the fallback date.| Prefer a factory like `() => new Date()` to create a fresh timestamp per instance.  
  
```
props: ({ date }) => ({
  startAt: date.default(() => new Date()),
  endAt: date.default(() => new Date(Date.now() + 60_000)),
})
```

### `json` #

`json` parses JSON text and clones structured values so instances do not share mutable default objects by accident.

Without an explicit `.default(...)`, the zero value is an empty object ``.

Member| Effect| Typical use  
---|---|---  
`.default(value)`| Supplies a fallback object or array.| Payloads, chart series, settings blobs, filter state.  
  
```
props: ({ json }) => ({
  series: json.default(() => []),
  options: json.default(() => ({ stacked: false, legend: true })),
})
```

### `js` #

`js` is similar to `json` but accepts JavaScript-like object syntax, not just strict JSON. Use it when consumers will hand-author complex literals in HTML and you want a more forgiving parser.

Without an explicit `.default(...)`, the zero value is an empty object ``.

Member| Effect| Typical use  
---|---|---  
`.default(value)`| Supplies a fallback object or array.| Config literals that are easier to write without quoted keys.  
  
```
props: ({ js }) => ({
  config: js.default(() => ({
    scale: 1,
    axis: { x: true, y: true },
  })),
})
```

### `bin` #

`bin` decodes base64 string input into `Uint8Array` and encodes bytes back into base64. Use it when the component’s natural public API is binary rather than textual.

Without an explicit `.default(...)`, the zero value is an empty `Uint8Array`.

Member| Effect| Typical use  
---|---|---  
`.default(value)`| Supplies fallback bytes.| Byte buffers, encoded data, binary previews.  
  
```
props: ({ bin }) => ({
  payload: bin,
})
```

### `array` #

`array` has two forms. With one nested codec it creates a homogeneous array. With multiple codecs it creates a tuple.

Without an explicit `.default(...)`, `array(codec)` defaults to `[]`. Tuple forms default each missing slot from that slot codec’s own default or zero value.

Form| Decoded type| What it means  
---|---|---  
`array(codec)`| `T[]`| Every item is decoded with the same codec.  
`array(codecA, codecB, codecC)`| Tuple| Each position has its own codec and default behavior.  
  
```
props: ({ array, string, number, bool }) => ({
  tags: array(string.trim.lower),
  point: array(number, number),
  localeSpec: array(
    string.lower.default('en'),
    number.min(1).default(1),
    bool,
  ).default(() => ['en', 1, false]),
})
```

Homogeneous arrays are ideal for tags, ids, and numeric series. Tuples are better when position matters, such as coordinates, breakpoints, or fixed parser options.

### `object` #

`object(shape)` builds a typed nested object. Each field has its own codec, so you can mix strings, numbers, booleans, arrays, and even nested objects inside a single prop.

Without an explicit `.default(...)`, each field falls back to that field codec’s own default or zero value.

Member| Effect| Notes  
---|---|---  
`object(shape)`| Creates a fixed-key decoded object.| Missing nested fields use their nested codec defaults when present.  
`.default(value)`| Supplies a fallback object.| Prefer a factory to create per-instance objects.  
  
```
props: ({ object, string, number, bool, array }) => ({
  profile: object({
    id: string.trim,
    name: string.trim.default('Anonymous'),
    age: number.min(0),
    admin: bool,
    tags: array(string.trim.lower),
  }).default(() => ({
    id: '',
    name: 'Anonymous',
    age: 0,
    admin: false,
    tags: [],
  })),
})
```

### `oneOf` #

`oneOf` constrains a prop to a known set of allowed values. You can pass literal values, codecs, or both.

Without an explicit `.default(...)`, the zero value is the first allowed entry.

Form| Typical uses| Behavior  
---|---|---  
`oneOf('a', 'b', 'c')`| Enums and variant names.| Returns the matching literal or the first/default entry.  
`oneOf(codecA, codecB)`| Union-like decoding.| Tries each codec in order until one succeeds.  
`.default(value)`| Explicit fallback.| Overrides the implicit “first option wins” fallback.  
  
```
props: ({ oneOf, string, number }) => ({
  tone: oneOf('neutral', 'info', 'success', 'warning', 'danger').default('neutral'),
  alignment: oneOf('start', 'center', 'end').default('start'),
  flexibleValue: oneOf(string.trim, number.round),
})
```

## Setup and Actions #

`setup` is where Rocket components become stateful when that state does not depend on rendered refs. The context object gives you a focused set of hooks to create local Datastar-backed state and wire browser behavior.

### Setup Context #

Helper| What it does| Why it helps web components  
---|---|---  
`props`| The normalized prop values for the current instance.| Keeps setup code on the same decoded inputs that render uses, without a second function argument.  
`$$`| Creates mutable instance-local state and exposes it on `$$.name`.| Gives each component instance its own Datastar-backed state bucket with a property-style API that mirrors template `$$name` access.  
`effect(fn)`| Runs a reactive side effect and tracks cleanup.| Ideal for timers, subscriptions, and imperative DOM/library sync.  
`apply(root, merge?)`| Runs Datastar apply on a root.| Useful when third-party code injects DOM that needs Datastar activation.  
`cleanup(fn)`| Registers disconnect cleanup.| Prevents leaked timers, observers, and library instances.  
`$`| Reads and writes the global Datastar signal store.| Useful when setup needs shared app state instead of component-local Rocket state.  
`actions`| Calls Datastar global actions from setup code.| Useful when component setup needs the same global helpers available to `@action(...)` expressions.  
`action(name, fn)`| Registers a local action callable from rendered markup.| Lets event handlers target the current component instance instead of global actions.  
`observeProps(fn, ...propNames)`| Responds to prop changes after decoding.| Separates prop-driven imperative work from full rerenders.  
`overrideProp(name, getter?, setter?)`| Wraps a declared prop’s default host accessor for this instance.| Useful when a public prop must read from or write through a live inner control.  
`defineHostProp(name, descriptor)`| Defines a host-only property or method on this instance.| Useful for native-like host APIs that are not Rocket props, such as `files` or imperative methods.  
`render(overrides, ...args)`| Reruns the component render function from setup code.| Lets async or imperative work trigger a render with explicit trailing args.  
`host`| The current custom element instance.| Gives access to attributes, classes, observers, focus, and shadow APIs.  
  
```javascript
rocket('demo-copy-button', {
  props: ({ string, number }) => ({
    text: string.default('Copy me'),
    resetMs: number.min(100).default(1200),
  }),
  setup: ({ $$, $, action, actions, cleanup, props }) => {
    $$.copied = false
    $$.label = () => ($$.copied ? 'Copied' : 'Copy')
    $$.resetMsLabel = actions.intl(
      'number',
      props.resetMs,
      { maximumFractionDigits: 0 },
      'en-US',
    )
    let timerId = 0

    action('copy', async () => {
      await navigator.clipboard.writeText(props.text)
      $$.copied = true
      if ($.analyticsEnabled !== false) {
        $.lastCopiedText = props.text
      }
      clearTimeout(timerId)
      timerId = window.setTimeout(() => {
        $$.copied = false
      }, props.resetMs)
    })

    cleanup(() => clearTimeout(timerId))
  },
  render: ({ html, props: { text } }) => {
    return html`
      <button data-on:click="@copy()">
        <span data-text="$$label"></span>
        <small>${text} ($$resetMsLabel ms)</small>
      </button>
    `
  },
})
```

Local actions are optional. Prefer plain Datastar expressions like `data-on:click="$$count += 1"` when local state changes are simple, and prefer page-owned state when the behavior belongs to the surrounding demo or app. Reach for `action(name, fn)` when the markup needs a named imperative entry point.

`$$` is the setup alias for Rocket-local signals. In practice that means `$$.count = 0` creates `$$count`, which templates can read, and also enables `$$.count` and `$$.count += 1` inside `setup`.

Assigning a function to `$$.name` creates a local computed signal, so `$$.label = () => $$.count + 1` is the shorthand form of a derived value. Use that form for derived local state.

`actions` exposes the global Datastar action registry inside `setup`. Use it when setup code needs the same helpers available to declarative expressions like `@intl(...)` or `@clipboard(...)`, without re-registering them as local Rocket actions.

`$` exposes the shared Datastar signal root inside `setup`. Use it when a component needs to coordinate with application-level state instead of only reading and writing Rocket’s instance-local signals.

Local refs created with `data-ref:name` are exposed on `onFirstRender({ refs })` as `refs.name`. They are populated during the Datastar apply pass, so they are intentionally not part of `setup(...)`. They are Rocket refs, not Rocket signals, so they do not appear on `$$`.

### Host Accessor Overrides #

Most components should stop at plain `props`. Rocket already gives each prop a host accessor plus decoded values, attribute reflection, upgrade replay, and `observeProps()` updates.

Use `overrideProp(name, getter?, setter?)` when that default accessor is not the right host API. Typical cases are native-like form wrappers where `host.value` or `host.checked` should mirror a live inner control instead of only returning the last decoded prop value.

Use `defineHostProp(name, descriptor)` when the member is not a Rocket prop at all. Good examples are read-only host properties like `files` or imperative host methods like `start()` and `stop()`.

Do not use accessor overrides for internal state. If outside code should not read or set it through the host element, keep it in `$$`. Also do not move every prop to an own-property just to be safe. Prototype accessors remain the default fast path.

The override helpers are available during `setup`:

```
overrideProp<Name extends keyof Props & string>(
  name: Name,
  getter?: (getDefault: () => Props[Name]) => any,
  setter?: (value: any, setDefault: (value: Props[Name]) => void) => void,
): void

defineHostProp(name: string, descriptor: PropertyDescriptor): void
```

If you omit `getter`, Rocket uses `getDefault()`. If you omit `setter`, Rocket uses `setDefault(value)`. That keeps the common case short when you only need one side customized.

```
rocket('demo-prop-normalize', {
  props: ({ string }) => ({
    value: string.default(''),
  }),
  setup: ({ overrideProp }) => {
    overrideProp('value', undefined, (value, setDefault) => {
      setDefault(String(value ?? '').trim())
    })
  },
})
```

Setup runs before Rocket’s initial render and Datastar apply pass. That means `data-ref:*` refs like `refs.input` or `refs.select` do not exist yet during the first synchronous line of `setup`.

If an override depends on rendered refs, put that wiring in `onFirstRender(...)`. It receives the setup context plus `refs`, so ref-backed code still has access to `$$`, props, cleanup, host helpers, and the rendered DOM refs.

```javascript
rocket('demo-input-bridge', {
  props: ({ string }) => ({
    value: string.default(''),
  }),
  render: ({ html, props: { value } }) => html`
    <input data-ref:input value="${value}">
  `,
  onFirstRender: ({ overrideProp, refs }) => {
    overrideProp(
      'value',
      (getDefault) => refs.input?.value ?? getDefault(),
      (value, setDefault) => {
        const next = String(value ?? '')
        if (refs.input && refs.input.value !== next) refs.input.value = next
        setDefault(next)
      },
    )
  },
})
```

```
rocket('demo-host-methods', {
  setup: ({ defineHostProp }) => {
    defineHostProp('version', {
      get() {
        return '1'
      },
    })
    defineHostProp('reset', {
      value() {
        console.log('reset')
      },
    })
  },
})
```

### Watching Attribute Changes #

`observeProps` is useful when prop changes should drive targeted imperative work. The callback receives the full normalized props object plus a `changes` object containing only the props that changed. If you omit `propNames`, `observeProps(fn)` watches all props.

```
rocket('demo-video-frame', {
  props: ({ string, number }) => ({
    src: string.trim,
    currentTime: number.min(0),
  }),
  mode: 'light',
  renderOnPropChange: false,
  onFirstRender: ({ refs, observeProps }) => {
    observeProps((props, changes) => {
      if (!(refs.video instanceof HTMLVideoElement)) {
        return
      }
      if ('src' in changes) {
        refs.video.src = props.src
      }
      if ('currentTime' in changes) {
        refs.video.currentTime = props.currentTime
      }
    })
  },
  render: ({ html, props: { src } }) => {
    return html`
        <video data-ref:video controls src="${src}"></video>
    `
  },
})
```

## Rendering and Scoping #

Rocket rendering is Datastar-aware. The output of `render` is not just a string template. Rocket parses it, converts it into DOM, rewrites local signal references, morphs the mounted subtree, and then applies Datastar behavior to the result.

### Render Contract #

The render context is:

Field| What it gives you| Why it exists  
---|---|---  
`html`| An HTML tagged template that returns a fragment.| Safely constructs HTML while supporting node composition and Datastar rewriting.  
`svg`| An SVG tagged template that returns SVG nodes.| Lets a component render SVG without manual namespace handling.  
`props`| The normalized prop values.| Keeps markup based on already-decoded component inputs.  
`host`| The custom element instance.| Allows render decisions based on host state, slots, or attributes.  
  
You should think of `render` as the component’s declarative DOM shape, not as an all-purpose setup hook. Create signals and effects in `setup`. Use `render` to express what the DOM should look like for the current props and local state.

If a light-DOM component needs to keep host-provided children, return `<slot>` markers where those children should go. That is not native slotting. In light mode Rocket uses `<slot>` as a projection marker because the browser only performs real slot distribution in shadow DOM. Rocket replaces those slot nodes with the original host children before morphing the host subtree.

### Render Example: Counter #

This is the smallest useful Rocket render pattern: typed props define the public API, `setup` creates local state because no refs are needed, and the returned HTML reads and writes local signals with standard Datastar attributes.

```
rocket('demo-stepper', {
  mode: 'light',
  props: ({ number, string }) => ({
    start: number.min(0),
    step: number.min(1).default(1),
    label: string.trim.default('Count'),
  }),
  setup: ({ $$, props }) => {
    $$.count = props.start
  },
  render: ({ html, props: { label, step } }) => {
    return html`
      <section class="stack gap-2">
        <h3>${label}</h3>
        <div class="row gap-2">
          <button data-on:click="$$count -= ${step}" data-attr:disabled="$$count <= 0">-</button>
          <output data-text="$$count"></output>
          <button data-on:click="$$count += ${step}">+</button>
        </div>
      </section>
    `
  },
})
```

The important detail is that the event handlers and bindings are just Datastar attributes. Rocket does not invent a second templating language for markup. It only scopes component state and packages the result as a reusable custom element.

### Render Example: List Rendering #

The `html` helper accepts composed nodes and iterables, so complex render output can stay declarative without string concatenation.

```
rocket('demo-nav-list', {
  props: ({ array, object, string }) => ({
    items: array(object({
      href: string.trim.default('#'),
      label: string.trim.default('Untitled'),
    })),
    title: string.trim.default('Navigation'),
  }),
  render: ({ html, props: { items, title } }) => {
    return html`
      <nav aria-label="${title}">
        <h3>${title}</h3>
        <ul>
          ${items.map((item) => html`
            <li>
              <a href="${item.href}">${item.label}</a>
            </li>
          `)}
        </ul>
      </nav>
    `
  },
})
```

That matters for web components because real components often render lists of nested child nodes. Rocket lets you return fragments from inner templates instead of dropping down to manual DOM creation.

### Render Example: SVG #

Use `svg` when the output is naturally vector-based. Rocket handles the SVG namespace and still rewrites Datastar expressions inside the returned nodes.

```javascript
rocket('demo-meter-ring', {
  props: ({ number, string }) => ({
    value: number.clamp(0, 100),
    stroke: string.default('#0f172a'),
  }),
  render: ({ html, svg, props: { value, stroke } }) => {
    const circumference = 2 * Math.PI * 28

    return html`
      <figure class="stack gap-2">
        ${svg`
          <svg viewBox="0 0 64 64" width="64" height="64" aria-hidden="true">
            <circle cx="32" cy="32" r="28" fill="none" stroke="#e5e7eb" stroke-width="8"></circle>
            <circle
              cx="32"
              cy="32"
              r="28"
              fill="none"
              stroke="${stroke}"
              stroke-width="8"
              stroke-dasharray="${circumference}"
              stroke-dashoffset="${circumference - (value / 100) * circumference}"
              transform="rotate(-90 32 32)"></circle>
          </svg>
        `}
        <figcaption>${value}%</figcaption>
      </figure>
    `
  },
})
```

### Conditional Rendering #

Rocket supports structural conditionals inside `render` with `<template data-if>`, `data-else-if`, and `data-else`.

```
rocket('demo-status', {
  mode: 'light',
  setup: ({ $$ }) => {
    $$.step = 0
  },
  render: ({ html }) => html`
    <div class="stack gap-2">
      <button type="button" data-on:click="$$step = ($$step + 1) % 3">Next</button>

      <template data-if="$$step === 0">
        <p>Idle</p>
      </template>
      <template data-else-if="$$step === 1">
        <p>Loading</p>
      </template>
      <template data-else>
        <p>Ready</p>
      </template>
    </div>
  `,
})
```

Only one branch in the chain is mounted at a time. Inactive branches are not present in the live DOM. Switching branches unmounts the old branch and mounts a fresh new one.

Conditionals are owned by the Rocket runtime rather than by normal Datastar attribute plugins. That lets Rocket defer `$$` rewriting until the selected branch is actually mounted. If you need an element to stay mounted and only change visibility, use `data-show` instead.

### Loop Rendering #

Rocket also supports structural list rendering with `<template data-for>`.

```
rocket('demo-letter-list', {
  mode: 'light',
  setup: ({ $$ }) => {
    $$.letters = ['A', 'B', 'C']
  },
  render: ({ html }) => html`
    <ul>
      <template data-for="letter, row in $$letters">
        <li>
          <strong data-text="row + 1"></strong>
          <span data-text="letter"></span>
        </li>
      </template>
    </ul>
  `,
})
```

`data-for` accepts any Datastar expression that evaluates to an iterable and supports exactly three shapes:

Form| Accepted Shape| Loop Locals  
---|---|---  
`data-for="$$letters"`| Bare iterable expression with default aliases.| `item`, `i`  
`data-for="letter in $$letters"`| Custom item alias with the default index alias.| `letter`, `i`  
`data-for="letter, row in $$letters"`| Custom item alias and custom index alias.| `letter`, `row`  
  
The source expression can be any iterable Datastar expression, so forms like `data-for="letter, row in $$letters.filter(Boolean)"` and `data-for="$page.items"` are both valid.

Rocket does not support an index-only form like `data-for=", row in $$letters"`. If you want a custom index alias, you must also provide an item alias.

Those loop locals are only available inside Datastar expressions in the repeated subtree. That means attributes like `data-text="item"`, `data-text="letter"`, and `data-class:active="i === 0"` work inside the loop body, while normal component locals like `$$selected` and global signals like `$page` keep their existing meaning outside those aliases.

Structural templates in Rocket, including conditionals and `data-for`, clone their selected `<template>` content into document fragments and then hand the resulting nodes back to Rocket’s normal patch/morph path. When the source list changes, Rocket keeps row slots by position and updates the current `item`/`i` bindings for each slot. If you reorder the source, Rocket does not preserve item identity across rows in this version.

If you author literal Datastar expressions that contain `${...}` inside Rocket example files, Biome can flag them with `lint/suspicious/noTemplateCurlyInString` even though they are intentional. This is the Biome config override this repo uses for Rocket example sources:

```
{
  "overrides": [
    {
      "includes": ["site/shared/static/rocket/**/*.js"],
      "linter": {
        "rules": {
          "suspicious": {
            "noTemplateCurlyInString": "off"
          }
        }
      }
    }
  ]
}
```

### Rocket Scope Rewriting #

Inside rendered Datastar expressions, Rocket rewrites `$$name` to an instance-specific signal path under `$._rocket`. That is what gives every component instance isolated local state while still using Datastar’s global signal store.

The instance segment comes from the host element’s `id` when one is present. Rocket normalizes that id into a path-safe identifier for Datastar expressions. If the element has no `id`, Rocket generates a sequential fallback instance id instead.

```
// You write this in render():
html`
  <button data-on:click="$$count += 1"></button>
  <span data-text="$$count"></span>
`

// For <demo-counter id="inventory-panel">, Rocket rewrites it as:
<button data-on:click="_rocket.demo_counter.inventory_panel.count += 1"></button>
<span data-text="_rocket.demo_counter.inventory_panel.count"></span>
```

That rewriting is what makes Rocket practical for reusable web components. You can drop ten instances of the same component into a page and each one gets separate local state without naming collisions. In normal component code you should still write `$$count`, not the rewritten `_rocket...` path directly.

### `__root` #

Use the `__root` modifier when a Rocket component needs to leave a signal-name attribute in the outer page scope instead of rewriting it into the component’s private `$._rocket...` path.

This is mainly for authored host children inside `open` or `closed` components. By default Rocket rescopes those children so `data-bind:name` inside a component instance becomes something like `data-bind:_rocket.my_component.id.name`. That is usually correct for component-local behavior, but it is wrong when the child should keep talking to a page-level signal like `$name`.

```html
<demo-fieldset>
  <demo-input data-bind:name__root></demo-input>
</demo-fieldset>
```

In that example, Rocket strips `__root` and leaves the binding as `data-bind:name` instead of rewriting it into the component scope.

Use `__root` sparingly. It is an escape hatch for wrapper-style components where host children should stay connected to outer-page Datastar signals. Do not use it for normal component internals, and do not use it when the signal should actually be instance-local.

For keyed signal-name attributes, put the modifier on the key segment: `data-bind:name__root`, `data-computed:total__root`, `data-indicator:loading__root`, `data-ref:input__root`. This keyed form is the right choice for authored host children because Datastar can still parse the base attribute shape before Rocket rewrites it.

Rocket currently applies `__root` to these signal-name attribute families:

  * `data-bind` and `data-bind:*`
  * `data-computed:*`
  * `data-indicator` and `data-indicator:*`
  * `data-ref` and `data-ref:*`

It does not currently affect every `data-*` attribute. In particular, attributes like `data-attr:*`, `data-text`, and Rocket’s own internal host/ref bookkeeping are separate concerns.

## Examples #

The best live references in the repo are the Rocket examples: [Copy Button](https://data-star.dev/examples/rocket_copy_button), [Counter](https://data-star.dev/examples/rocket_counter), [Flow](https://data-star.dev/examples/rocket_flow), [Letter Stream](https://data-star.dev/examples/rocket_letter_stream), [QR Code](https://data-star.dev/examples/rocket_qr_code), [Starfield](https://data-star.dev/examples/rocket_starfield), and [Virtual Scroll](https://data-star.dev/examples/rocket_virtual_scroll).

Use this page as the API reference and those pages as the behavioral reference. Between the two, you should be able to define a typed, reactive, reusable web component without falling back to ad hoc component wiring.

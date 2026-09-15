<!-- Source: https://github.com/starfederation/datastar/releases -->
<!-- Fetched: 2026-09-14 -->

# Datastar release notes

Newest first. Includes the RC series because the 1.0 syntax changes (`:` delimiter, `data-init`, etc.) landed in RC.6.

## v1.0.3 — 2026-08-27

- Added an opt-in [CSP mode](https://data-star.dev/reference/security#csp-mode) that lets Datastar run without `unsafe-eval` ([#1142](https://github.com/starfederation/datastar/issues/1142)).
- The current state of signals is now sent when retrying backend requests due to a network error ([#1174](https://github.com/starfederation/datastar/issues/1174)).
- Fixed a bug in which view transition support was being checked on DOM elements but not the document ([#1181](https://github.com/starfederation/datastar/issues/1181)).
- Fixed a bug in which dynamically adding a `multiple` property to a `<select>` element would not update the bound signal with the selected values.

## v1.0.2 — 2026-06-02

- Added a `viewTransitionSelector` data line to SSE responses that specifies the CSS selector for the element to use for view transitions when patching elements. ([#1154](https://github.com/starfederation/datastar/issues/1154))
- Changed the fetch abort controller to cancel in-flight requests with the same method and URL, regardless of the element that initiated the request. ([#1166](https://github.com/starfederation/datastar/issues/1166))
- Changed the default event for checkbox and radio inputs using `data-bind` to `input` for more immediate updates.
- Optimized patch element handling for improved performance when only a single target is involved. ([#1155](https://github.com/starfederation/datastar/issues/1155))
- Fixed a bug in which the `data-bind` attribute with a modifier was not working correctly for checkboxes with the signal set to an array. ([#1159](https://github.com/starfederation/datastar/issues/1159))
- Fixed retry options for fetch actions to ensure that retries are attempted correctly when a request fails with a 5xx response. ([#1161](https://github.com/starfederation/datastar/issues/1161))

## v1.0.1 — 2026-04-20

- Made it so that the `__prop` and `__event` modifiers of the `data-bind` attribute can be used independently of each other.
- The prop value following the `__prop` modifier of the `data-bind` attribute is now converted to camel case.
- Fixed a bug in `data-bind` in which the signal type for a `select` element was being set to a number instead of a string when no initial value was set.

## v1.0.0 — 2026-04-16

> Datastar 1.0 has finally shipped. We are done. Done like dinner. Watch the [launch podcast](https://youtu.be/T6uwri94ylk).

- The payload is now resent when reconnecting after a tab visibility change when using the `fetch` action. ([#1140](https://github.com/starfederation/datastar/issues/1140))
- The `Content-Type: application/json` header is now only set for requests that contain a body. ([#1144](https://github.com/starfederation/datastar/issues/1144))
- A `body` is now only sent for non-GET and non-DELETE requests. ([#1144](https://github.com/starfederation/datastar/issues/1144))
- The `data-bind` attribute now supports a `__prop` modifier that binds through a specific property instead of the inferred native/default binding.
- The `data-bind` attribute now supports an `__event` modifier that defines which events sync the element back to the signal.
- The `data-bind` attribute now respects the initial checked property of radio buttons.
- The `data-on` attribute now supports a `__document` modifier to attach event listeners to the `document` element. ([#1151](https://github.com/starfederation/datastar/issues/1151))
- Improved morphing of `input`, `select` and `textarea` elements. ([#1075](https://github.com/starfederation/datastar/issues/1075))
- A new `datastar-prop-change` event is now emitted instead of the native `change` event whenever a property is changed during morphing.
- Improved the `kebab` function to handle consecutive uppercase letters.
- Fixed a bug in which the value of an input element with `type="submit"` was not being included in form submissions. ([#1150](https://github.com/starfederation/datastar/issues/1150))
- Fixed a bug in which the `__viewtransition` modifier was interfering with other modifiers and event methods. ([#1153](https://github.com/starfederation/datastar/issues/1153))
- Renamed the `retryMaxWaitMs` backend action option to `retryMaxWait` for consistency.
- Rocket has been rewritten as a JavaScript API. See the new documentation at [data-star.dev/reference/rocket](https://data-star.dev/reference/rocket).

## v1.0.0-RC.8 — 2026-03-02

- Added the [`data-match-media`](https://data-star.dev/reference/attributes#data-match-media) Datastar Pro attribute that sets a signal to `true` or `false` depending on whether a media query matches, and keeps it in sync whenever the query changes.
- Added the [`@intl`](https://data-star.dev/reference/actions#intl) action to Datastar Pro, for internationalized formatting of dates, numbers, and other values using the [Intl namespace object](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl).
- Fetch requests with `requestCancellation` set to `auto` no longer cancel the request when the initiating element/attribute is removed from the DOM. Use the `cleanup` option to opt in to that behavior.
- Backend action requests now send the current state of the signals when retrying, rather than the initial state. ([#900](https://github.com/starfederation/datastar/issues/900))
- Datastar expressions containing dollar sign symbols in string/template literals (e.g. `'$foo'`) are no longer parsed as signal references. ([#1106](https://github.com/starfederation/datastar/issues/1106))
- The `datastar-patch-elements` event now accepts `Element` and `DocumentFragment` payloads and will consume/move them instead of re-parsing strings (non-string payloads are single-use and target only the first match). ([#1125](https://github.com/starfederation/datastar/issues/1125))
- The `data-attr` attribute now preserves function values.
- Fixed a bug in the `data-on` plugin in which event listeners were not cleared on element removal when using the `__capture` modifier.
- Fixed a bug in the `data-on-intersect` plugin in which the `threshold` modifier was not properly applied.

## v1.0.0-RC.7 — 2025-12-16

- Added the `namespace` data line to the `datastar-patch-elements` event, which makes it possible to patch elements using `svg` and `mathml` namespaces. ([#1108](https://github.com/starfederation/datastar/issues/1108))
- Added the `retry` option to backend actions, where `auto` is the default behavior and `error` is any `4xx` or `5xx` response. ([#1073](https://github.com/starfederation/datastar/issues/1073))
- Added the `payload` option to backend actions, which allows the fetch payload to be overridden.
- Added an `__exit` modifier to the `data-on-intersect` attribute to trigger the callback when the element exits the viewport. ([#1011](https://github.com/starfederation/datastar/issues/1011))
- Added a `__threshold` modifier to the `data-on-intersect` attribute to specify a custom intersection threshold as a percentage.
- Added the ability to have multiple cleanups per attribute.
- The `data-class` attribute no longer runs its expression on cleanup. ([#1060](https://github.com/starfederation/datastar/issues/1060))
- Backend actions now default to peeking signals so that no subscriptions are created. ([#1100](https://github.com/starfederation/datastar/issues/1100))
- Form fields are no longer validated if a `novalidate` attribute exists on the `form` tag (for real, this time). ([#1056](https://github.com/starfederation/datastar/issues/1056))
- Fetch requests are now cancelled when the any ancestor of the element that initiated them is removed from the DOM and the `requestCancellation` option is set to `auto`. ([#1078](https://github.com/starfederation/datastar/issues/1078), [#1080](https://github.com/starfederation/datastar/pull/1080))
- The `openWhenHidden` option for backend actions now defaults to `false` for `GET` requests and `true` for all other HTTP methods.
- Setting a signal to an object now merges the diff into the existing proxy rather than replacing it.
- Improved the handling of SVG elements when patching elements.
- Fixed a bug in which, when using the `throttle` modifier with a `trailing` argument, the expression was executed with the event from the leading edge of the throttle window rather than the last event. ([#1091](https://github.com/starfederation/datastar/issues/1091))
- Fixed a bug in which duplicate requests were being made due to visibility changes. ([#1096](https://github.com/starfederation/datastar/issues/1096))
- Fixed a bug in which the `data-on-signal-patch` attribute was not working when used with an alias.

## v1.0.0-RC.6 — 2025-10-21

This release contains a syntax change to attributes that _must_ be addressed manually before updating. 

The recommended CDN link for Datastar is now version-locked to ensure safer updates and to avoid serving outdated cached files:
```
<script type="module" src="https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.0-RC.6/bundles/datastar.js"></script>
```

### Breaking changes:

- The attribute key delimiter has been changed from `-` to `:` (`data-signals-foo` becomes `data-signals:foo`, `data-on-click` becomes `data-on:click`, etc.), as allowed by the [HTML Standard](https://html.spec.whatwg.org/multipage/syntax.html#attributes-2). The following regular expression can be used to update attributes.
    - Search: `data-(?!(on-(intersect|interval|load|raf|resize|signal-patch|signal-patch-filter)(_|=)))(attr|bind|class|computed|indicator|on|persist|ref|signals|style)-`  
    - Replace: `data-$4:`
- Renamed the `data-on-load` attribute to `data-init`.
- Renamed `.trail` and `.notrail` modifier arguments to `.trailing` and `.notrailing` respectively.
- Removed file upload progress monitoring from Datastar Pro, which had limited browser support. It is recommended to use a file upload library instead, or wait for Rocket which will include a file upload web component. ([#1049](https://github.com/starfederation/datastar/issues/1049))
- Fetch requests are now cancelled when the element that initiated them is removed from the DOM. ([#1045](https://github.com/starfederation/datastar/pull/1045))
- The `data-bind` attribute on an input with type `file` now only creates one signal instead of `{signalName}`, `{signalName}Names`, and `{signalName}Mimes`. The resultant type of the signal is `{name: string, contents: string, mime: string}[]`. ([#1041](https://github.com/starfederation/datastar/pull/1041))
- Camel casing no longer capitalizes after a number, and kebab casing now only preserves what was written in the attribute key (`data-signals:a2b--foo__case.kebab` makes `$a2b--foo` instead of `$a-2-b-foo`). ([#1030](https://github.com/starfederation/datastar/pull/1030))

### Non-breaking changes:

- Datastar now supports all browsers that have support for ES2021. ([#1037](https://github.com/starfederation/datastar/issues/1037))
- Added support for using object syntax with the `data-computed` attribute. ([#1053](https://github.com/starfederation/datastar/issues/1053))
- The `data-indicator` attribute now sets the signal to `false` even if the signal has already been defined.
- Fetch requests now respect URIs provided by a [`base`](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/base) tag. ([#1047](https://github.com/starfederation/datastar/issues/1047))
- Form fields are no longer validated if a `novalidate` attribute exists on the `form` tag. ([#1056](https://github.com/starfederation/datastar/issues/1056))
- Setting a signal to `null` or `undefined` now actually deletes the signal. ([#1058](https://github.com/starfederation/datastar/issues/1058))
- Attributes can now be used on the `html` element.
- Attribute keys and aliases can now start with a non-letter (`data-signals:1foo` or `data-on:-foo`). ([#1030](https://github.com/starfederation/datastar/pull/1030))
- Fixed a bug in `data-bind` when using namespaced signals as arrays.
- Fixed a bug resulting in `data-ignore__self` not being applied. ([#1064](https://github.com/starfederation/datastar/issues/1064))
- Fixed a bug preventing the `__debounce.leading` modifier from working.
- Fixed a bug preventing the combination of `__debounce` and `__viewtransition` modifiers from working.
- Fixed a bug when using `auto` request cancellation where the `FINISHED` fetch event would fire after the `STARTED` fetch event of the cancelling request.

## v1.0.0-RC.5 — 2025-08-15

- The `data-on-*` attribute no longer requires that the `isTrusted` property on the event is `true` for an expression to be executed.
- The `data-on-load` attribute no longer wraps the expression in `setTimeout` when no delay is provided.
- Changed the `filterSignals` option in the backend actions to maintain the default `include` and `exclude` values if only one or the other is provided.
- Fixed a bug in which setting the length of an array signal would cause errors when trying to access an element of the array.
- Fixed a bug in which `data-query-string` would merge in the path of the object rather than the object itself.
- Removed the `__trusted` modifier.

## v1.0.0-RC.4 — 2025-08-01

- Fixed a bug in which an input element being morphed would not have its value updated.

## v1.0.0-RC.3 — 2025-07-29

This release packs some new features, cleans up `data-bind` behavior, and fixes a rake of minor bugs.

- Added support for patching `html`, `head`, and `body` elements.
- Added support for patching arbitrary fragments into the DOM given a selector is provided.
- Added support for passing include and exclude filters in as strings.
- Added a `__filter` modifier to `data-query-string` that filters out empty values when syncing signals to query string params.
- When a signal is predefined, its `type` is now preserved during binding – whenever the element’s value changes, the signal value is automatically converted to match the original type. 
- Custom keys in `data-persist` are now added as `data-persist-mykey`, and the `__key` modifier has been removed.
- Numeric signal names are now correctly parsed.
- Fixed a regression in which `data-attr` values were not being JSON encoded.
- Fixed a bug in which `data-text` and `data-json-signals` would not reapply themselves after their text was changed.
- Fixed a bug in which attributes that had a prefix with the same length as the alias would not be ignored (e.g. `data-option-load` would be applied as `on-load`).
- Fixed a bug in which the `retries-failed` event was named `retrying`.
- Fixed a bug in which the `remove` mode would fail when patching elements if no elements were provided.
- Fixed a bug in which `data-class` would enter an infinite loop if there were multiple such attributes on the same element.
- Fixed a bug in which attribute plugins on child elements were not cleaned up when removed.
- Removed support for using the ID of a provided element as a selector for the `remove` mode when patching elements.
- Changed the behavior of how input values are diffed when morphed:

| Old input value | New input value  | Behavior                              |
| --------------- | ---------------- | -------------------------------------- |
| `null`          | `null`           | preserve old input value               |
| some value      | the same value   | preserve old input value               |
| some value      | `null`           | set old input value to `""`            |
| `null`          | some value       | set old input value to new input value |
| some value      | some other value | set old input value to new input value |

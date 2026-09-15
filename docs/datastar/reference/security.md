<!-- Source: https://data-star.dev/reference/security -->
<!-- Fetched: 2026-09-14 -->

# Security

[Datastar expressions](https://data-star.dev/guide/datastar_expressions) are strings that are evaluated in a sandboxed context. This means you can use JavaScript in Datastar expressions.

## Escape User Input #

The golden rule of security is to never trust user input. This is especially true when using Datastar expressions, which can execute arbitrary JavaScript. When using Datastar expressions, you should always escape user input. This helps prevent, among other issues, Cross-Site Scripting (XSS) attacks.

## Avoid Sensitive Data #

Keep in mind that signal values are visible in the source code in plain text, and can be modified by the user before being sent in requests. For this reason, you should avoid leaking sensitive data in signals and always implement backend validation.

## Ignore Unsafe Input #

If, for some reason, you cannot escape unsafe user input, you should ignore it using the [`data-ignore`](https://data-star.dev/reference/attributes#data-ignore) attribute. This tells Datastar to ignore an element and its descendants when processing DOM nodes.

## Content Security Policy #

By default, Datastar uses the [`Function()` constructor](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Function/Function) to evaluate expressions. The [Content Security Policy](https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP) (CSP) for this mode must include `unsafe-eval`.

```html
<meta http-equiv="Content-Security-Policy"
    content="script-src 'self' 'unsafe-eval'"
>
```

### CSP Mode #

To enable CSP mode, add a `data-nonce` attribute to the `html` element. Its value must match the nonce in your CSP’s `script-src` directive. Generate a new cryptographically secure random nonce on the server for every full-page response.

```html
<html data-nonce="{page-nonce}">
    <head>
        <meta http-equiv="Content-Security-Policy"
            content="script-src 'self' 'nonce-{page-nonce}';"
        >
        <script type="module" src="/datastar.js"></script>
    </head>
    <body>
        <button data-on:click="$count++">Increment</button>
    </body>
</html>
```

Datastar reads the nonce and removes the `data-nonce` attribute. It applies the nonce when compiling expressions and when executing scripts received in element patches or JavaScript responses. Element patch responses do not need to include the nonce.

CSP mode does not make Datastar expressions safe to use with untrusted content. Datastar does not examine or sanitize expressions in Datastar attributes. Untrusted content inserted into an attribute can therefore execute JavaScript.

Use the context-appropriate escaping and serialization tools provided by your server language and template system. If you intentionally allow user-provided HTML, sanitize it with a suitable HTML sanitizer. Pass user values through signals instead of interpolating them into Datastar expressions, and keep the expressions static.

In browsers that support Trusted Types, Datastar creates a policy named `datastar` for HTML and script content. This policy allows Datastar to work with `require-trusted-types-for 'script'`. It does not sanitize the content. 

```html
<meta http-equiv="Content-Security-Policy"
    content="script-src 'self' 'nonce-{page-nonce}'; trusted-types datastar; require-trusted-types-for 'script';"
>
```

Learn more about [Trusted Type policies](https://developer.mozilla.org/en-US/docs/Web/API/TrustedTypePolicy).

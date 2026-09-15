<!-- Source: https://data-star.dev/examples/form_data -->
<!-- Fetched: 2026-09-14 -->

# Form Data 

Demo

foo: bar: baz:

Submit GET request Submit POST request

Submit GET request from outside the form

## Explanation #

Setting the `contentType` option to `form` tells the `@get()` action to look for the closest form, perform validation on it, and send all form elements within it to the backend. A `selector` option can be provided to specify a form element. No signals are sent to the backend in this type of request.

```html
<form id="myform">
    foo:<input type="checkbox" name="checkboxes" value="foo" />
    bar:<input type="checkbox" name="checkboxes" value="bar" />
    baz:<input type="checkbox" name="checkboxes" value="baz" />
    <button data-on:click="@get('/endpoint', {contentType: 'form'})">
        Submit GET request
    </button>
    <button data-on:click="@post('/endpoint', {contentType: 'form'})">
        Submit POST request
    </button>
</form>

<button data-on:click="@get('/endpoint', {contentType: 'form', selector: '#myform'})">
    Submit GET request from outside the form
</button>
```

Demo

foo: 

Submit form

## Explanation #

In this example, the `@get()` action is placed inside a submit listener on the form element using `data-on:submit`.

```html
<form data-on:submit="@get('/endpoint', {contentType: 'form'})">
    foo: <input type="text" name="foo" required />
    <button>
        Submit form
    </button>
</form>
```

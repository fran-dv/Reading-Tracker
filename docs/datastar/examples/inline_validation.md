<!-- Source: https://data-star.dev/examples/inline_validation -->
<!-- Fetched: 2026-09-14 -->

# Inline Validation 

Demo

Email Address 

The only valid email address is "test@test.com".

First Name  Last Name  Sign Up

## HTML #

```html
<div id="demo">
    <label>
        Email Address
        <input
            type="email"
            required
            aria-live="polite"
            aria-describedby="email-info"
            data-bind:email
            data-on:keydown__debounce.500ms="@post('/examples/inline_validation/validate')"
        />
    </label>
    <p id="email-info" class="info">The only valid email address is "test@test.com".</p>
    <label>
        First Name
        <input
            type="text"
            required
            aria-live="polite"
            data-bind:first-name
            data-on:keydown__debounce.500ms="@post('/examples/inline_validation/validate')"
        />
    </label>
    <label>
        Last Name
        <input
            type="text"
            required
            aria-live="polite"
            data-bind:last-name
            data-on:keydown__debounce.500ms="@post('/examples/inline_validation/validate')"
        />
    </label>
    <button
        class="success"
        data-on:click="@post('/examples/inline_validation')"
    >
        <i class="material-symbols:person-add"></i>
        Sign Up
    </button>
</div>
```

## Explanation #

This example shows how to do inline field validation, in this case of an email address. To do this we need to create a form with an input that `POST`s back to the server with the value to be validated and updates the DOM with the validation results. Since it’s easy to replace the whole form, the logic for displaying the validation results is kept simple.

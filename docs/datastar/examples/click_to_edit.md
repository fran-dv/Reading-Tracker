<!-- Source: https://data-star.dev/examples/click_to_edit -->
<!-- Fetched: 2026-09-14 -->

# Click To Edit 

Demo

First Name: John

Last Name: Doe

Email: joe@blow.com

Edit Reset

## Explanation #

The click to edit pattern is a way to inline edit all or part of a record without a page refresh. This pattern starts with a UI that shows the details of a contact. The div has a button that will get the editing UI for the contact from `/edit`

```html
<div id="demo">
    <p>First Name: John</p>
    <p>Last Name: Doe</p>
    <p>Email: joe@blow.com</p>
    <div role="group">
        <button
            class="info"
            data-indicator:_fetching
            data-attr:disabled="$_fetching"
            data-on:click="@get('/examples/click_to_edit/edit')"
        >
            Edit
        </button>
        <button
            class="warning"
            data-indicator:_fetching
            data-attr:disabled="$_fetching"
            data-on:click="@patch('/examples/click_to_edit/reset')"
        >
            Reset
        </button>
    </div>
</div>
```

This returns a form that can be used to edit the contact

```html
<div id="demo">
    <label>
        First Name
        <input
            type="text"
            data-bind:first-name
            data-attr:disabled="$_fetching"
        >
    </label>
    <label>
        Last Name
        <input
            type="text"
            data-bind:last-name
            data-attr:disabled="$_fetching"
        >
    </label>
    <label>
        Email
        <input
            type="email"
            data-bind:email
            data-attr:disabled="$_fetching"
        >
    </label>
    <div role="group">
        <button
            class="success"
            data-indicator:_fetching
            data-attr:disabled="$_fetching"
            data-on:click="@put('/examples/click_to_edit')"
        >
            Save
        </button>
        <button
            class="error"
            data-indicator:_fetching
            data-attr:disabled="$_fetching"
            data-on:click="@get('/examples/click_to_edit/cancel')"
        >
            Cancel
        </button>
    </div>
</div>
```

### There Is No Form #

If you compare to htmx you’ll notice there is no form, you can use one, but it’s unnecessary. This is because you’re already using signals and when you `PUT` to `/edit`, the body is the entire contents of the signals, and it’s available to handle errors and validation holistically. There is also a profanity filter on the normal rendering of the contact that is not applied to the edit form. Controlling the rendering completely on the server allows you to have a single source of truth for the data and the rendering.

### There Is No Client Side Validation #

On the backend we’ve also added a quick sanitizer on the input to avoid bad actors (to some degree). You already have to deal with the data on the server so you might as well do the validation there. In this case, its just modifying how the text is rendered when not editing. This is a simple example, but you can see how to extend it to more complex forms.

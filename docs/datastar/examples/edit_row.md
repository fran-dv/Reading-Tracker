<!-- Source: https://data-star.dev/examples/edit_row -->
<!-- Fetched: 2026-09-14 -->

# Edit Row 

Demo

Name| Email| Actions  
---|---|---  
Joe Smith| joe@smith.org| Edit  
Angie MacDowell| angie@macdowell.org| Edit  
Fuqua Tarkenton| fuqua@tarkenton.org| Edit  
Kim Yee| kim@yee.org| Edit  
  
Reset

## Explanation #

This example shows how to implement editable rows. First let’s look at the row prior to editing:

```html
<tr>
    <td>Joe Smith</td>
    <td>joe@smith.org</td>
    <td>
        <button data-on:click="@get('/examples/edit_row/0')">
            Edit
        </button>
    </td>
</tr>
```

This will trigger a whole table replacement as we are going to remove the edit buttons from other rows as well as change out the inputs to allow editing.

Finally, here is what the row looks like when the data is being edited:

```html
<tr>
    <td>
        <input type="text" data-bind:name>
    </td>
    <td>
        <input type="text" data-bind:email>
    </td>
    <td>
        <button data-on:click="@get('/examples/edit_row/cancel')">
            Cancel
        </button>
        <button data-on:click="@patch('/examples/edit_row/0')">
            Save
        </button>
    </td>
</tr>
```

Here we have a few things going on, clicking the cancel button will bring back the read-only version of the row. Finally, there is a save button that issues a `PATCH` to update the contact.

<!-- Source: https://data-star.dev/examples/delete_row -->
<!-- Fetched: 2026-09-14 -->

# Delete Row 

Demo

Name| Email| Actions  
---|---|---  
Joe Smith| joe@smith.org| Delete  
Angie MacDowell| angie@macdowell.org| Delete  
Fuqua Tarkenton| fuqua@tarkenton.org| Delete  
Kim Yee| kim@yee.org| Delete  
Reset

## Explanation #

This example shows how to implement a delete button that removes a table row upon completion. First let’s look at the table body:

```html
<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Email</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>Joe Smith</td>
            <td>joe@smith.org</td>
            <td>
                <button
                    class="error"
                    data-on:click="confirm('Are you sure?') && @delete('/examples/delete_row/0')"
                    data-indicator:_fetching
                    data-attr:disabled="$_fetching"
                >
                    Delete
                </button>
            </td>
        </tr>
    </tbody>
</table>
```

The row has a normal confirm to `confirm()` the delete action.

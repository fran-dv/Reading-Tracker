<!-- Source: https://data-star.dev/examples/bulk_update -->
<!-- Fetched: 2026-09-14 -->

# Bulk Update 

Demo

| Name| Email| Status  
---|---|---|---  
| Joe Smith| joe@smith.org| Inactive  
| Angie MacDowell| angie@macdowell.org| Inactive  
| Fuqua Tarkenton| fuqua@tarkenton.org| Inactive  
| Kim Yee| kim@yee.org| Inactive  
  
Activate Deactivate

## HTML #

```html
<div
    id="demo"
    data-signals__ifmissing="{_fetching: false, selections: Array(4).fill(false)}"
>
    <table>
        <thead>
            <tr>
                <th>
                    <input
                        type="checkbox"
                        data-bind:_all
                        data-on:change="$selections = Array(4).fill($_all)"
                        data-effect="$selections; $_all = $selections.every(Boolean)"
                        data-attr:disabled="$_fetching"
                    />
                </th>
                <th>Name</th>
                <th>Email</th>
                <th>Status</th>
            </tr>
        </thead>
        <tbody>
            <tr>
                <td>
                    <input
                        type="checkbox"
                        data-bind:selections
                        data-attr:disabled="$_fetching"
                    />
                </td>
                <td>Joe Smith</td>
                <td>joe@smith.org</td>
                <td>Active</td>
            </tr>
            <!-- More rows... -->
        </tbody>
    </table>
    <div role="group">
        <button
            class="success"
            data-on:click="@put('/examples/bulk_update/activate')"
            data-indicator:_fetching
            data-attr:disabled="$_fetching"
        >
            <i class="pixelarticons:user-plus"></i>
            Activate
        </button>
        <button
            class="error"
            data-on:click="@put('/examples/bulk_update/deactivate')"
            data-indicator:_fetching
            data-attr:disabled="$_fetching"
        >
            <i class="pixelarticons:user-x"></i>
            Deactivate
        </button>
    </div>
</div>
```

## Explanation #

This example shows how to implement a common pattern where rows are selected and then bulk updated. This is accomplished by putting a form around a table, with checkboxes in the table, and then including the checked values in `PUT`s to two different endpoints: activate and deactivate.

The server will either activate or deactivate the checked users and then re-render the table with updated rows.

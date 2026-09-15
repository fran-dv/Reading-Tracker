<!-- Source: https://data-star.dev/examples/file_upload -->
<!-- Fetched: 2026-09-14 -->

# File Upload 

Demo

Pick anything less than 1 MiB

Submit

## Explanation #

In this example we show how to create a file upload form that will be submitted via fetch.

```html
<label>
    <p>Pick anything less than 1MB</p>
    <input type="file" data-bind:files multiple/>
</label>
<button
    class="warning"
    data-on:click="$files.length && @post('/examples/file_upload')"
    data-attr:disabled="!$files.length"
>
    Submit
</button>
```

We don’t need a form because everything is encoded as signals and automatically sent to the server. We `POST` the form to `/examples/file_upload`, since the `input` is using `data-bind` the file’s contents will be automatically encoded as base64. 

### Note #

If you try to upload a file that is too large you will get an error message in the console.

<!-- Source: https://data-star.dev/examples -->
<!-- Fetched: 2026-09-14 -->

# Active Search 

Demo

First Name| Last Name  
---|---  
Dock| Wiza  
Kylie| Hauck  
Mustafa| Cassin  
Yasmin| Huel  
Rosemarie| Anderson  
Mireya| Lakin  
Jerrell| Collier  
Myron| Stokes  
Mohammed| Beatty  
Justus| Nitzsche  
  
## Explanation #

This example actively searches a contacts database as the user enters text.

The interesting part is the input field:

```html
<input
    type="text"
    placeholder="Search..."
    data-bind:search
    data-on:input__debounce.200ms="@get('/examples/active_search/search')"
/>
```

The input issues a `GET` to `/active_search/search` with the input value bound to `$search`. The `__debounce.200ms` modifier ensures that the search is not issued on every keystroke, but only after the user has stopped typing.

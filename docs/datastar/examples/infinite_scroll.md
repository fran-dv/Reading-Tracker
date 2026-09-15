<!-- Source: https://data-star.dev/examples/infinite_scroll -->
<!-- Fetched: 2026-09-14 -->

# Infinite Scroll 

The infinite scroll pattern provides a way to load content dynamically on user scrolling action.

Let’s focus on the final row (or the last element of your content):

```html
<div data-on-intersect="@get('/examples/infinite_scroll/more')">
    Loading...
</div>
```

This last element contains a listener which, when scrolled into view, will trigger a request. The result is then appended after it. `data-on-intersect` is an attribute that triggers a request when the element is scrolled into view.

Demo

Agents Name| Email| ID  
---|---|---  
Agent Smith 0| void1@null.org| 1982e3a7bb241055  
Agent Smith 1| void2@null.org| 65cd25028f98f158  
Agent Smith 2| void3@null.org| 7b95a7322f5da314  
Agent Smith 3| void4@null.org| 7324dc1e7e9474f0  
Agent Smith 4| void5@null.org| 628911027fcf803f  
Agent Smith 5| void6@null.org| 5edb980100c87e72  
Agent Smith 6| void7@null.org| 3564a48862bc4a0d  
Agent Smith 7| void8@null.org| 6eed105b82285fa  
Agent Smith 8| void9@null.org| 664f427c6b2c4bea  
Agent Smith 9| void10@null.org| 28353a066812b268  
  
Loading...

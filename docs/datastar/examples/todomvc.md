<!-- Source: https://data-star.dev/examples/todomvc -->
<!-- Fetched: 2026-09-14 -->

# TodoMVC 

Demo

  * Learn any backend language
  * Learn Datastar
  * ???
  * Profit

**3** items pending AllPendingCompleted Delete Reset

## Explanation #

This is a full implementation of TodoMVC using Datastar. It demonstrates complex state management, including adding, editing, deleting, and filtering todos, all handled through server-sent events.

## HTML #

```html
<section
    id="todomvc"
    data-init="@get('/examples/todomvc/updates')"
>
    <header id="todo-header">
        <input
            type="checkbox"
            data-on:click__prevent="@post('/examples/todomvc/-1/toggle')"
            data-init="el.checked = false"
        />
        <input
            id="new-todo"
            type="text"
            placeholder="What needs to be done?"
            data-signals:input
            data-bind:input
            data-on:keydown="
                evt.key === 'Enter' && $input.trim() && @patch('/examples/todomvc/-1') && ($input = '');
            "
        />
    </header>
    <ul id="todo-list">
        <!-- Todo items are dynamically rendered here -->
    </ul>
    <div id="todo-actions">
        <span>
            <strong>0</strong> items pending
        </span>
        <button class="small info" data-on:click="@put('/examples/todomvc/mode/0')">
            All
        </button>
        <button class="small" data-on:click="@put('/examples/todomvc/mode/1')">
            Pending
        </button>
        <button class="small" data-on:click="@put('/examples/todomvc/mode/2')">
            Completed
        </button>
        <button class="error small" aria-disabled="true">
            Delete
        </button>
        <button class="warning small" data-on:click="@put('/examples/todomvc/reset')">
            Reset
        </button>
    </div>
</section>
```

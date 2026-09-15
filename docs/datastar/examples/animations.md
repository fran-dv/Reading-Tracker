<!-- Source: https://data-star.dev/examples/animations -->
<!-- Fetched: 2026-09-14 -->

# Animations 

Datastar is designed to allow you to use CSS transitions and the new View Transitions API to add smooth animations and transitions to your web page using only CSS and HTML. Below are a few examples of various animation techniques.

## Color Throb #

The simplest animation technique in Datastar is to keep the id of an element stable across a content swap. If the id of an element is kept stable, Datastar will swap it in such a way that CSS transitions can be written between the old version of the element and the new one.

Consider this div

Demo

brown on orange

With SSE, we just update the style every second

```html
<div
    id="color-throb"
    style="color: var(--blue-8); background-color: var(--orange-5);"
>
    blue on orange
</div>
```

## View Transitions #

The swapping of the button below is happening on the backend. Each click is causing a transition of state. The animated opacity animation is provided automatically by the View Transition API (not yet supported by Firefox). Doesn’t matter if the targeted elements are different types, it will still “do the right thing”.

Demo

Swap It!

## Fade Out On Swap #

If you want to fade out an element that is going to be removed when the request ends, just send an SSE event with the opacity set to 0 and set a transition duration. This will fade out the element before it is removed.

Demo

Fade out then delete on click

## Fade In On Addition #

Building on the previous example, we can fade in the new content the same way, starting from an opacity of 0 and transitioning to an opacity of 1.

Demo

Fade me in on click

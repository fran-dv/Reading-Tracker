// A combobox: a text input driving a listbox of results as its value
// narrows (Keyboard Navigation Through Results, P1). Datastar owns
// whether the list is open ($_showResults) and what picking an option
// does (each option's own data-on:click); this script tracks the
// keyboard cursor and mirrors it onto aria-activedescendant, the way
// picker.js does for its own listbox.
//
// Markup contract:
//   input[role=combobox][aria-controls=<listbox id>][aria-expanded]  (one or more per listbox)
//   ul[role=listbox][data-query]
//     li[role=option]  (any number; only these move the cursor)

const COMBOBOX = "[role=combobox]";
const OPTION = "[role=option]";
const LISTBOX = "[role=listbox]";

function listbox(input) {
  return document.getElementById(input.getAttribute("aria-controls"));
}

function options(input) {
  const list = listbox(input);
  return list ? [...list.querySelectorAll(OPTION)] : [];
}

function isOpen(input) {
  return input.getAttribute("aria-expanded") === "true";
}

// isFresh reports whether the listbox's current contents actually answer
// what the field holds right now — the same comparison that drives the
// results-pending class (Pending State Without Flicker, P4), read
// straight from the DOM. A request in flight, or one still debouncing,
// answers a query the field has already moved past; its cursor must
// never be armed.
function isFresh(input) {
  const list = listbox(input);
  return !!list && input.value.trim() === list.dataset.query;
}

function activeIndex(input) {
  return options(input).findIndex((o) => o.classList.contains("is-active"));
}

// clearActive drops the cursor entirely: no option is active and
// aria-activedescendant points at nothing. Used whenever the cursor's
// target has gone stale — the field changed, or the list closed.
function clearActive(input) {
  options(input).forEach((o) => o.classList.remove("is-active"));
  input.removeAttribute("aria-activedescendant");
}

function setActive(input, index) {
  const opts = options(input);
  if (opts.length === 0) {
    clearActive(input);
    return;
  }
  index = Math.max(0, Math.min(index, opts.length - 1));
  opts.forEach((o, i) => o.classList.toggle("is-active", i === index));
  input.setAttribute("aria-activedescendant", opts[index].id);
  opts[index].scrollIntoView({ block: "nearest" });
}

// refreshCursor puts the cursor on the first (most relevant) result —
// the same "starts on the current value" rule picker.js applies on open
// — whenever the list is open and its contents genuinely answer the
// field's current value. A stale or still-in-flight answer leaves the
// cursor alone instead, which in practice means clearActive's last
// keystroke wins: nothing is active until a fresh answer lands.
function refreshCursor(input) {
  if (isOpen(input) && isFresh(input)) setActive(input, 0);
}

document.addEventListener("keydown", (evt) => {
  const input = evt.target.closest(COMBOBOX);
  // A pending list (isFresh false) is shown, dimmed, per the Pending
  // State Without Flicker rule, but never armed: arrow keys and Enter
  // both stay off it until a fresh answer confirms what is on screen
  // still matches what is typed.
  if (!input || !isOpen(input) || !isFresh(input)) return;
  switch (evt.key) {
    case "ArrowDown":
    case "ArrowUp":
      evt.preventDefault();
      setActive(input, activeIndex(input) + (evt.key === "ArrowDown" ? 1 : -1));
      break;
    case "Enter": {
      const i = activeIndex(input);
      if (i < 0) return; // nothing highlighted: let the form submit as typed
      evt.preventDefault();
      options(input)[i].click();
      break;
    }
  }
});

// Every keystroke narrows the results, so a cursor position left over
// from the previous list means nothing once it is gone — drop it
// outright (not just the attribute), since Enter reads the .is-active
// class, not aria-activedescendant. Then try to re-arm at once: typing
// back to a value the list already answers (no round trip needed to
// notice) is fresh immediately, with no mutation to wait for.
document.addEventListener("input", (evt) => {
  const input = evt.target.closest(COMBOBOX);
  if (!input) return;
  clearActive(input);
  refreshCursor(input);
});

// A refined query re-renders the list while it stays open, so
// aria-expanded flipping (the list opening or closing) is not the only
// thing that can move the cursor: data-query changing, or the listbox's
// children changing (Datastar's morph skips a same-value setAttribute,
// so an unchanged query answered again — Try again — only ever shows up
// here), both call for the same refresh.
//
// More than one combobox can control the same listbox (capture's title
// field and its ISBN field both drive #search-results). When the listbox
// itself changes, the input actually being typed in — not just the first
// one in the document — is the one whose cursor a fresh answer should
// arm, so the currently focused matching combobox wins over document
// order.
function comboboxFor(listbox) {
  const matches = `${COMBOBOX}[aria-controls="${listbox.id}"]`;
  if (document.activeElement && document.activeElement.matches(matches)) return document.activeElement;
  return document.querySelector(matches);
}

new MutationObserver((mutations) => {
  const refresh = new Set();
  for (const { type, attributeName, target } of mutations) {
    if (type === "attributes" && attributeName === "aria-expanded" && target.matches(COMBOBOX)) {
      if (isOpen(target)) {
        refresh.add(target);
      } else {
        clearActive(target);
      }
      continue;
    }
    const isListboxChange =
      (type === "attributes" && attributeName === "data-query" && target.matches(LISTBOX)) ||
      (type === "childList" && target.matches(LISTBOX));
    if (!isListboxChange) continue;
    const input = comboboxFor(target);
    if (input) refresh.add(input);
  }
  refresh.forEach(refreshCursor);
}).observe(document.body, {
  attributes: true,
  attributeFilter: ["aria-expanded", "data-query"],
  childList: true,
  subtree: true,
});

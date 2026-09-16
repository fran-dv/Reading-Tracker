// A drawn listbox. Every .picker on a page shares this behaviour; what an
// option *does* stays in its own data-on:click, which sets the signals the
// form reads.
//
// Markup contract:
//   .picker
//     button.picker-button[aria-haspopup=listbox][aria-expanded]
//     ul.picker-list[role=listbox][hidden]
//       li.picker-option[role=option][aria-selected]  (any number)
//
// The button's aria-expanded is the open state. The keyboard cursor is the
// .is-active option, mirrored on the button as aria-activedescendant.
// Keys: arrows move, Enter or Space picks, Escape closes.

const OPTION = "[role=option]";

function button(picker) {
  return picker.querySelector(".picker-button");
}

function options(picker) {
  return [...picker.querySelectorAll(OPTION)];
}

function isOpen(picker) {
  return button(picker).getAttribute("aria-expanded") === "true";
}

function setActive(picker, index) {
  const opts = options(picker);
  if (opts.length === 0) return;
  index = Math.max(0, Math.min(index, opts.length - 1));
  opts.forEach((o, i) => o.classList.toggle("is-active", i === index));
  button(picker).setAttribute("aria-activedescendant", opts[index].id);
  opts[index].scrollIntoView({ block: "nearest" });
}

function activeIndex(picker) {
  return options(picker).findIndex((o) => o.classList.contains("is-active"));
}

function open(picker) {
  button(picker).setAttribute("aria-expanded", "true");
  picker.querySelector(".picker-list").hidden = false;
  // The cursor starts on the current value.
  const selected = options(picker).findIndex((o) => o.getAttribute("aria-selected") === "true");
  setActive(picker, Math.max(0, selected));
}

function close(picker) {
  button(picker).setAttribute("aria-expanded", "false");
  button(picker).removeAttribute("aria-activedescendant");
  picker.querySelector(".picker-list").hidden = true;
  options(picker).forEach((o) => o.classList.remove("is-active"));
}

document.addEventListener("click", (evt) => {
  const picker = evt.target.closest(".picker");
  if (!picker) return;
  if (evt.target.closest(".picker-button")) {
    isOpen(picker) ? close(picker) : open(picker);
  } else if (evt.target.closest(OPTION)) {
    close(picker); // the option's own handler has already run
  }
});

// Pressing inside the list must not move focus off the button: the button
// losing focus closes the list on mousedown, and the option never receives
// its click.
document.addEventListener("mousedown", (evt) => {
  if (evt.target.closest(".picker-list")) evt.preventDefault();
});

document.addEventListener("focusout", (evt) => {
  const picker = evt.target.closest(".picker");
  if (picker && !picker.contains(evt.relatedTarget)) close(picker);
});

document.addEventListener("keydown", (evt) => {
  const picker = evt.target.closest(".picker");
  if (!picker) return;
  switch (evt.key) {
    case "ArrowDown":
    case "ArrowUp":
      evt.preventDefault();
      if (!isOpen(picker)) {
        open(picker);
      } else {
        setActive(picker, activeIndex(picker) + (evt.key === "ArrowDown" ? 1 : -1));
      }
      break;
    case "Enter":
    case " ":
      if (!isOpen(picker)) return; // the button opens itself
      evt.preventDefault();
      options(picker)[activeIndex(picker)]?.click();
      break;
    case "Escape":
      if (!isOpen(picker)) return;
      evt.preventDefault();
      close(picker);
      break;
  }
});

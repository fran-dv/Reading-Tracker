// Drag-and-drop and paste for the cover plate (cover-management: Cover
// Upload Entry Points, P12). Both feed the same hidden #cover-file
// input item-form.html's own file chooser already wires up, through
// its existing "change" handler, so every entry point — chooser, drop,
// paste — shares one upload path and nothing here talks to the server
// directly. stage() does not pre-filter by file type: a dropped file
// this cannot use still reaches /covers/upload, which already answers
// errors.cover with the plain line (one source of truth for what
// counts as readable, same as the server-side ISBN checksum).

function stage(file) {
  const input = document.getElementById("cover-file");
  if (!input || !file) return;
  const files = new DataTransfer();
  files.items.add(file);
  input.files = files.files;
  input.dispatchEvent(new Event("change", { bubbles: true }));
}

document.addEventListener("dragover", (evt) => {
  if (evt.target.closest(".cover-plate")) evt.preventDefault();
});

document.addEventListener("drop", (evt) => {
  if (!evt.target.closest(".cover-plate") || !evt.dataTransfer?.files.length) return;
  evt.preventDefault();
  stage(evt.dataTransfer.files[0]);
});

// Paste only while the open item form itself holds focus (capture's
// #capture, or the shelf view's open #entry-form), so pasting an image
// somewhere else on the page — into a note, say — is never hijacked.
document.addEventListener("paste", (evt) => {
  if (!document.activeElement?.closest("#capture, #entry-form")) return;
  const file = [...(evt.clipboardData?.items ?? [])]
    .find((item) => item.kind === "file" && item.type.startsWith("image/"))
    ?.getAsFile();
  if (file) stage(file);
});

// Two small jobs that share this file for being too small to need one
// each: drag-and-drop and paste for the cover plate (cover-management:
// Cover Upload Entry Points, P12), and a helper the edition picker's own
// candidate button uses (item-form.html).

// resetEditionCandidates empties the picker's own candidates row in the
// DOM directly. #cover-candidates holds whatever the server last
// patched into it; clearing the signals that named which book it was
// fetched for (workKey, title) does not, by itself, remove what is
// already drawn, so every place those signals change for a different
// book calls this too — otherwise a stale candidate from the old book
// stays clickable and would fill the new one's fields from it.
window.resetEditionCandidates = function () {
  document.getElementById("cover-candidates")?.replaceChildren();
};

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

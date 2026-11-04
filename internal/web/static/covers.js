// Small jobs for the cover plate that share this file for being too small
// to need one each: drag-and-drop and paste (cover-management: Cover
// Upload Entry Points, P12), a helper the edition picker's own candidate
// button uses (item-form.html), and the generated cover's cloth for the
// capture preview.

// coverCloth is gencover.go's coverCloth, line for line, so the preview
// wears the cloth the filed book will: FNV-1a over the UTF-8 bytes of the
// trimmed title, A to Z lower-cased, modulo the eight cloths. The vectors
// in gencover_test.go hold here too:
//   "Thinking in Systems" 2, "  THINKING IN SYSTEMS  " 2, "Dune" 3,
//   "Pedro Páramo" 4, "Invisible Cities" 7, "Ulysses" 1, "Emma" 5,
//   "Stanisław Lem" 0, "こころ" 1, "" 5.
window.coverCloth = function (title) {
  let hash = 0x811c9dc5;
  for (let byte of new TextEncoder().encode(title.trim())) {
    if (byte >= 0x41 && byte <= 0x5a) byte += 0x20;
    hash = Math.imul(hash ^ byte, 0x01000193) >>> 0;
  }
  return hash % 8;
};

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

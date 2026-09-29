// Barcode scan (P10, design.md ADR-6): a Scan action beside the ISBN
// field, present only when the browser can actually do it. Everything else
// about that field — checksum validation, the lookup it triggers — is
// Datastar and server-side (design.md ADR-5); this script only turns a
// camera view into digits in the ISBN input.
//
// Markup contract (once, per page that shows the item form):
//   button#scan-button[hidden]
//   dialog#scan-dialog
//     video#scan-video
//
// Capture renders this markup on the page itself, but the shelf view's
// edit form (internal/web/templates/shelf.html "entry-form") only exists
// once Edit is clicked, patched in well after this module has already run
// once. So init() runs again whenever #main's contents change, the same
// way combobox.js's own observer notices a redrawn listbox; a per-button
// marker keeps a page that already has its Scan button wired from being
// wired twice.
//
// Task 4.1 technical check (2026-09-28): confirmed against MDN's Barcode
// Detection API page and caniuse (September 2026 data). BarcodeDetector
// needs a secure context (HTTPS, or `localhost`) even where a browser
// implements it at all — exactly what isSecureContext below already
// checks. Chrome and Chromium-based browsers (desktop and Android)
// support it; Firefox does not implement it; Safari (macOS and iOS) ships
// the constructor disabled by default on every released version, so
// "BarcodeDetector" in window reads false there unless the owner turns on
// an experimental flag by hand — exactly the outcome the feature test
// below already assumes. ean_13, the one format scanned for here, is on
// every implementation's supported list. No contradiction with ADR-6 to
// report.
const supported = window.isSecureContext && "BarcodeDetector" in window && !!navigator.mediaDevices;

function wire(button, dialog, video) {
  let stream = null;
  let timer = null;

  // Closing the dialog — the Close button, Escape, or a successful scan
  // calling dialog.close() itself — always fires the dialog's own "close"
  // event, so this is the one place every camera track gets stopped.
  function stop() {
    clearInterval(timer);
    timer = null;
    if (stream) {
      stream.getTracks().forEach((track) => track.stop());
      stream = null;
    }
    video.srcObject = null;
  }

  async function start() {
    dialog.showModal();
    let granted;
    try {
      granted = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "environment" } });
    } catch {
      dialog.close(); // the owner declined the camera, or none exists
      return;
    }
    if (!dialog.open) {
      // Closed already (Close, Escape) while the permission prompt was up.
      granted.getTracks().forEach((track) => track.stop());
      return;
    }
    stream = granted;
    video.srcObject = stream;
    await video.play();
    if (!dialog.open) {
      // Closed while play() was settling; stop() already ran once (the
      // dialog's own "close" listener), but the stream it stopped was
      // still null at that point, so this stream needs stopping itself.
      stream.getTracks().forEach((track) => track.stop());
      stream = null;
      return;
    }

    const detector = new BarcodeDetector({ formats: ["ean_13"] });
    let detecting = false; // a detect() call can outlast one 200ms tick; never overlap two
    timer = setInterval(async () => {
      if (detecting) return;
      detecting = true;
      try {
        const codes = await detector.detect(video);
        const match = codes.map((c) => c.rawValue).find((v) => v.startsWith("978") || v.startsWith("979"));
        // dialog.open: this detect() call may have outlasted a dialog the
        // owner already closed (Close, Escape) while it was in flight;
        // stop() already ran, so there is nothing left to write into or
        // close a second time.
        if (match && dialog.open) {
          const isbnInput = document.getElementById("isbn");
          isbnInput.value = match;
          isbnInput.dispatchEvent(new Event("input", { bubbles: true })); // Datastar's data-bind and lookup trigger take it from there
          dialog.close();
        }
      } catch {
        // a frame mid-transition; the next tick tries again
      } finally {
        detecting = false;
      }
    }, 200); // ~5Hz, per ADR-6
  }

  button.hidden = false;
  button.addEventListener("click", start);
  dialog.addEventListener("close", stop);
}

function init() {
  const button = document.getElementById("scan-button");
  const dialog = document.getElementById("scan-dialog");
  const video = document.getElementById("scan-video");
  if (!button || !dialog || !video || button.dataset.scanWired) return;
  button.dataset.scanWired = "true";
  wire(button, dialog, video);
}

if (supported) {
  init();
  new MutationObserver(init).observe(document.getElementById("main"), { childList: true, subtree: true });
}

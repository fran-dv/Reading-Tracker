// The app shell, and nothing else (spec §1.3): static assets are cached so
// the page can draw offline; data always comes from the server.
//
// Network first: assets are unversioned, so a fresh binary must win over
// the cache whenever the server can be reached. The cache only answers when
// it cannot.
const CACHE = "shell-v1";
const SHELL = [
  "/static/app.css",
  "/static/datastar.js",
  "/static/picker.js",
  "/static/fonts/AlegreyaSans-Regular.woff2",
  "/static/fonts/AlegreyaSans-Medium.woff2",
  "/static/fonts/Alegreya-Italic.woff2",
  "/static/icon.svg",
  "/static/manifest.json",
];

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== CACHE).map((key) => caches.delete(key))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (event.request.method !== "GET" || !url.pathname.startsWith("/static/")) {
    return; // pages and actions go straight to the server
  }
  event.respondWith(
    fetch(event.request)
      .then((response) => {
        const copy = response.clone();
        caches.open(CACHE).then((cache) => cache.put(event.request, copy));
        return response;
      })
      .catch(() => caches.match(event.request)),
  );
});

const CACHE_NAME = 'api-gateway-pwa-v1';
const APP_SHELL = [
  '/',
  '/intro.html',
  '/index.html',
  '/traffic.html',
  '/security.html',
  '/apikeys.html',
  '/routes.html',
  '/cloud.html',
  '/chaos.html',
  '/style.css',
  '/app.js',
  '/logo-dark.png',
  '/icon-192.png',
  '/icon-512.png',
  '/manifest.json'
];

// Install: pre-cache core app shell
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(APP_SHELL);
    }).then(() => self.skipWaiting())
  );
});

// Activate: clean up outdated caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames.map((name) => {
          if (name !== CACHE_NAME) {
            return caches.delete(name);
          }
        })
      );
    }).then(() => self.clients.claim())
  );
});

// Fetch strategy
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // 1. Bypass API endpoints, SSE streams, metrics, and non-GET
  if (
    event.request.method !== 'GET' ||
    url.pathname.startsWith('/api/') ||
    url.pathname.startsWith('/metrics') ||
    url.pathname.startsWith('/healthz')
  ) {
    return;
  }

  // 2. For HTML navigations: Network-first with offline cache fallback
  if (event.request.mode === 'navigate' || event.request.headers.get('accept')?.includes('text/html')) {
    event.respondWith(
      fetch(event.request)
        .then((response) => {
          if (response && response.status === 200) {
            const responseClone = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(event.request, responseClone));
          }
          return response;
        })
        .catch(() => {
          return caches.match(event.request).then((cached) => {
            return cached || caches.match('/intro.html');
          });
        })
    );
    return;
  }

  // 3. For static assets (CSS, JS, images, fonts): Stale-While-Revalidate
  event.respondWith(
    caches.match(event.request).then((cached) => {
      const fetchPromise = fetch(event.request).then((networkResponse) => {
        if (networkResponse && networkResponse.status === 200) {
          const responseClone = networkResponse.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, responseClone));
        }
        return networkResponse;
      }).catch(() => cached);

      return cached || fetchPromise;
    })
  );
});

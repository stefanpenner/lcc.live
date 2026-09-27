self.addEventListener('install', () => {
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener('push', (event) => {
  let data = { title: 'Canyon road', body: 'This is a test alert.', url: '/' };
  try {
    if (event.data) data = Object.assign(data, event.data.json());
  } catch (e) { /* keep the default */ }
  event.waitUntil(self.registration.showNotification(data.title || 'Canyon road', {
    body: data.body || '',
    data: { url: data.url || '/' },
  }));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const url = (event.notification.data && event.notification.data.url) || '/';
  event.waitUntil(clients.openWindow(url));
});

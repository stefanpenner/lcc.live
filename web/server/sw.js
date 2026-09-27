self.addEventListener('install', () => {
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener('push', (event) => {
  let data = { title: 'Canyon road', body: 'The road is closed.', url: '/', tag: 'lcc-road' };
  try {
    if (event.data) data = Object.assign(data, event.data.json());
  } catch (e) { /* keep the default */ }
  const title = data.title || 'Canyon road';
  event.waitUntil(self.registration.showNotification(title, {
    body: data.body || '',
    tag: data.tag || 'lcc-road',
    renotify: true,
    icon: '/s/apple-touch-icon.png',
    data: { url: data.url || '/' },
  }));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const url = (event.notification.data && event.notification.data.url) || '/';
  event.waitUntil(clients.openWindow(url));
});

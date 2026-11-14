"use strict";
/* Service worker for OG's Salon Web Push.

   The server sends a small JSON payload through the browser's push service.
   This worker turns it into a system notification even when no app tab is
   open, and opens the right page when the notification is tapped. */

self.addEventListener("push", function (event) {
    var payload = {};
    if (event.data) {
        try {
            payload = event.data.json();
        } catch (error) {
            payload = { body: event.data.text() };
        }
    }

    var title = payload.title || "OG's Salon";
    var options = {
        body: payload.body || "You have a new notification.",
        tag: payload.tag || undefined,
        renotify: Boolean(payload.tag),
        data: { url: payload.url || "/" }
    };

    event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", function (event) {
    event.notification.close();

    var url = "/";
    if (event.notification.data && event.notification.data.url) {
        url = event.notification.data.url;
    }

    event.waitUntil(
        clients.matchAll({ type: "window", includeUncontrolled: true }).then(function (windows) {
            for (var i = 0; i < windows.length; i++) {
                var client = windows[i];
                if (client.url === url || client.url.indexOf(url) === 0) {
                    return client.focus();
                }
            }
            if (clients.openWindow) {
                return clients.openWindow(url);
            }
            return null;
        })
    );
});

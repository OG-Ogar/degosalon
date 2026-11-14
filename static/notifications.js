"use strict";
/* Live booking and chat alerts for a signed-in barber or customer.

   Two layers work together:

   1. In-app polling. The page polls /<role>/notifications. The first call is a
      baseline (the newest booking and message ids); later calls ask only for
      rows newer than the last id seen. A popup, a short chime and a vibration
      announce a new booking or message while the app is open.

   2. Web Push. When the user taps "Enable alerts", the browser registers a
      service worker and a push subscription. The server then notifies the
      device through its push service even when the app is closed. Without
      VAPID keys on the server the in-app layer still works; only the
      closed-app part is unavailable.

   The chime is synthesised with the Web Audio API, so no audio file is shipped.
   Browsers block audio until the visitor has interacted with the page, so the
   context is unlocked on the first tap or key press. */
(function () {
    "use strict";

    var script = document.currentScript || document.querySelector("script[data-notification-role]");
    var role = script ? script.getAttribute("data-notification-role") : "";
    if (role !== "barber" && role !== "customer") {
        return;
    }

    var POLL_MS = 20000;
    var RETRY_MS = 45000;
    var PROMPT_KEY = "ogs-alert-prompt-" + role;
    var ALERT_ENDPOINT = "/" + role + "/notifications";
    var SUBSCRIBE_ENDPOINT = ALERT_ENDPOINT + "/subscribe";
    var SERVICE_WORKER = "/static/sw.js";

    var bookingCursor = null;
    var messageCursor = null;
    var initialized = false;
    var seenBookings = {};
    var seenMessages = {};
    var timer = null;
    var audioContext = null;
    var audioUnlocked = false;
    var stack = null;
    var pushPublicKey = "";
    var pushSynced = false;

    /* ------------------------------------------------------------------
     * Small helpers
     * ---------------------------------------------------------------- */

    function requestJSON(url, options) {
        if (typeof window.fetch !== "function") {
            return Promise.reject(new Error("fetch unavailable"));
        }
        var settings = options || {};
        settings.credentials = "same-origin";
        settings.headers = settings.headers || {};
        settings.headers.Accept = "application/json";
        return window.fetch(url, settings).then(function (response) {
            if (!response.ok) {
                throw new Error("HTTP " + response.status);
            }
            if (response.status === 204) {
                return null;
            }
            return response.json();
        });
    }

    function store(key, value) {
        try {
            window.localStorage.setItem(key, value);
        } catch (error) {
            /* Private browsing: the prompt simply appears again next time. */
        }
    }

    function stored(key) {
        try {
            return window.localStorage.getItem(key);
        } catch (error) {
            return null;
        }
    }

    function naira(amount) {
        if (typeof amount !== "number" || !isFinite(amount) || amount <= 0) {
            return "";
        }
        var text = amount.toFixed(2);
        var parts = text.split(".");
        var digits = parts[0];
        var grouped = digits.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
        return "\u20a6" + grouped + "." + parts[1];
    }

    function whenText(booking) {
        var date = String(booking.date || "").trim();
        var time = String(booking.time || "").trim();
        /* The database often returns HH:MM:SS; the barber only needs HH:MM. */
        time = time.replace(/^(\d{1,2}:\d{2}):\d{2}$/, "$1");
        if (!date) {
            return time || "Time not set";
        }
        var stamp = new Date(date + "T" + (time || "00:00"));
        if (isNaN(stamp.getTime())) {
            return (date + " " + time).trim();
        }
        try {
            return stamp.toLocaleDateString(undefined, {
                weekday: "short",
                day: "numeric",
                month: "short"
            }) + (time ? " \u00b7 " + time : "");
        } catch (error) {
            return (date + " " + time).trim();
        }
    }

    function serviceText(booking) {
        var service = String(booking.service || "").trim();
        return service || "Haircut";
    }

    function messagePreview(message) {
        var text = String(message.text || "").trim();
        if (text) {
            return text;
        }
        if (message.image) {
            return "Sent a photo";
        }
        if (message.video) {
            return "Sent a video";
        }
        if (message.audio) {
            return "Sent a voice note";
        }
        return "Sent a message";
    }

    function senderText(message) {
        var sender = String(message.sender || "").trim();
        return sender || "Someone";
    }

    /* ------------------------------------------------------------------
     * Chime (synthesised, no file)
     * ---------------------------------------------------------------- */

    function ensureAudio() {
        var Ctx = window.AudioContext || window.webkitAudioContext;
        if (!Ctx) {
            return null;
        }
        if (!audioContext) {
            try {
                audioContext = new Ctx();
            } catch (error) {
                return null;
            }
        }
        return audioContext;
    }

    function unlockAudio() {
        var context = ensureAudio();
        if (!context) {
            return;
        }
        var resume = context.resume ? context.resume() : null;
        if (resume && typeof resume.then === "function") {
            resume.then(function () {
                audioUnlocked = true;
            }).catch(function () {});
        } else {
            audioUnlocked = true;
        }
    }

    function chime() {
        var context = ensureAudio();
        if (!context) {
            return;
        }
        if (context.state === "suspended" && context.resume) {
            context.resume().catch(function () {});
        }
        try {
            var now = context.currentTime;
            var notes = [880, 1174.66];
            for (var i = 0; i < notes.length; i++) {
                var start = now + i * 0.19;
                var oscillator = context.createOscillator();
                var gain = context.createGain();
                oscillator.type = "sine";
                oscillator.frequency.value = notes[i];
                gain.gain.setValueAtTime(0.0001, start);
                gain.gain.exponentialRampToValueAtTime(0.22, start + 0.03);
                gain.gain.exponentialRampToValueAtTime(0.0001, start + 0.34);
                oscillator.connect(gain);
                gain.connect(context.destination);
                oscillator.start(start);
                oscillator.stop(start + 0.38);
            }
        } catch (error) {
            /* The popup still communicates the alert. */
        }
    }

    function buzz() {
        if (navigator.vibrate) {
            try {
                navigator.vibrate([220, 90, 220]);
            } catch (error) {
                /* Vibration is a bonus, never a requirement. */
            }
        }
    }

    /* ------------------------------------------------------------------
     * Visual popup
     * ---------------------------------------------------------------- */

    function ensureStyles() {
        if (document.getElementById("notification-alert-style")) {
            return;
        }
        var style = document.createElement("style");
        style.id = "notification-alert-style";
        style.textContent = [
            ".notification-alert-stack{position:fixed;top:12px;left:50%;transform:translateX(-50%);z-index:2147483000;display:flex;flex-direction:column;gap:10px;width:min(420px,calc(100vw - 24px));pointer-events:none}",
            ".notification-alert{pointer-events:auto;display:flex;align-items:flex-start;gap:12px;padding:14px 14px 14px 16px;border-radius:14px;border:1px solid #d8e0ea;background:#fff;box-shadow:0 18px 40px rgba(12,21,36,.22);font:inherit;color:#0c1524;animation:notification-alert-in .22s ease-out}",
            ".notification-alert-icon{font-size:1.35rem;line-height:1}",
            ".notification-alert-copy{flex:1;min-width:0}",
            ".notification-alert-copy strong{display:block;font-size:.95rem}",
            ".notification-alert-copy span{display:block;margin-top:3px;font-size:.82rem;color:#40516a;line-height:1.4;overflow-wrap:anywhere}",
            ".notification-alert-copy small{display:block;margin-top:3px;font-size:.75rem;color:#68778d}",
            ".notification-alert-actions{display:flex;flex-direction:column;gap:6px;align-items:flex-end}",
            ".notification-alert-view{padding:7px 11px;border-radius:8px;background:#142034;color:#fff;font-size:.76rem;font-weight:750;text-decoration:none;white-space:nowrap}",
            ".notification-alert-close{border:0;background:transparent;color:#68778d;font-size:1.1rem;line-height:1;cursor:pointer;padding:2px 4px}",
            ".notification-alert-prompt{position:fixed;left:50%;bottom:16px;transform:translateX(-50%);z-index:2147483000;display:flex;align-items:center;gap:10px;width:min(420px,calc(100vw - 24px));padding:12px 14px;border-radius:13px;border:1px solid #d8e0ea;background:#0c1524;color:#fff;box-shadow:0 18px 40px rgba(12,21,36,.28);font:inherit;font-size:.8rem}",
            ".notification-alert-prompt p{flex:1;margin:0;line-height:1.4}",
            ".notification-alert-prompt button{border:1px solid rgba(255,255,255,.35);background:transparent;color:#fff;border-radius:8px;padding:7px 10px;font:inherit;font-size:.76rem;font-weight:750;cursor:pointer;white-space:nowrap}",
            ".notification-alert-prompt button[data-alert-enable]{background:#fff;color:#0c1524;border-color:#fff}",
            "@keyframes notification-alert-in{from{opacity:0;transform:translateY(-10px)}to{opacity:1;transform:translateY(0)}}",
            "@media (prefers-reduced-motion:reduce){.notification-alert{animation:none}}"
        ].join("");
        document.head.appendChild(style);
    }

    function ensureStack() {
        if (stack && document.body.contains(stack)) {
            return stack;
        }
        ensureStyles();
        stack = document.createElement("div");
        stack.className = "notification-alert-stack";
        stack.setAttribute("role", "status");
        stack.setAttribute("aria-live", "assertive");
        document.body.appendChild(stack);
        return stack;
    }

    function dismiss(card) {
        if (card && card.parentNode) {
            card.parentNode.removeChild(card);
        }
    }

    function buildCard(iconText, titleText, detailText, extraText, href, dismissLabel) {
        var host = ensureStack();
        var card = document.createElement("div");
        card.className = "notification-alert";

        var icon = document.createElement("span");
        icon.className = "notification-alert-icon";
        icon.setAttribute("aria-hidden", "true");
        icon.textContent = iconText;

        var copy = document.createElement("div");
        copy.className = "notification-alert-copy";
        var title = document.createElement("strong");
        title.textContent = titleText;
        var detail = document.createElement("span");
        detail.textContent = detailText;
        copy.appendChild(title);
        copy.appendChild(detail);
        if (extraText) {
            var extra = document.createElement("small");
            extra.textContent = extraText;
            copy.appendChild(extra);
        }

        var actions = document.createElement("div");
        actions.className = "notification-alert-actions";

        var view = document.createElement("a");
        view.className = "notification-alert-view";
        view.href = href;
        view.textContent = "Open";

        var close = document.createElement("button");
        close.type = "button";
        close.className = "notification-alert-close";
        close.setAttribute("aria-label", dismissLabel);
        close.textContent = "\u00d7";
        close.addEventListener("click", function () {
            dismiss(card);
        });

        actions.appendChild(view);
        actions.appendChild(close);
        card.appendChild(icon);
        card.appendChild(copy);
        card.appendChild(actions);
        host.appendChild(card);

        while (host.children.length > 3) {
            host.removeChild(host.firstChild);
        }

        window.setTimeout(function () {
            dismiss(card);
        }, 25000);
    }

    function showBookingBanner(bookings) {
        if (bookings.length === 1) {
            var one = bookings[0];
            buildCard(
                "\ud83d\udd14",
                "New booking from " + (one.customer || "a customer"),
                serviceText(one) + " \u00b7 " + whenText(one),
                naira(one.price),
                "/barber/bookings",
                "Dismiss booking alert"
            );
            return;
        }

        var first = bookings[0];
        buildCard(
            "\ud83d\udd14",
            bookings.length + " new bookings",
            "Latest: " + (first.customer || "a customer") + " \u00b7 " + serviceText(first) + " \u00b7 " + whenText(first),
            "Open Bookings to accept or decline them.",
            "/barber/bookings",
            "Dismiss booking alerts"
        );
    }

    function showMessageBanner(message) {
        var sender = senderText(message);
        var href = String(message.url || "").trim();
        if (!href) {
            href = role === "barber"
                ? "/barber/chats/" + message.customerId
                : "/customer/chats/" + message.barberId;
        }
        buildCard(
            "\ud83d\udcac",
            "New message from " + sender,
            sender + ": " + messagePreview(message),
            "",
            href,
            "Dismiss message alert"
        );
    }

    function systemNotify(title, body, url, tag) {
        if (!("Notification" in window) || window.Notification.permission !== "granted") {
            return;
        }
        /* The popup already says it while the user is looking at the page.
           The system banner is for a hidden tab; Web Push covers a closed app. */
        if (!document.hidden) {
            return;
        }
        try {
            var notice = new Notification(title, {
                body: body,
                tag: tag
            });
            notice.onclick = function () {
                window.focus();
                if (url) {
                    window.location.href = url;
                }
                notice.close();
            };
        } catch (error) {
            /* Android Chrome cannot construct Notification directly from a
               page; the in-app popup and Web Push have already run. */
        }
    }

    function announceBookings(bookings) {
        chime();
        buzz();
        showBookingBanner(bookings);
        var first = bookings[0];
        var title = bookings.length === 1
            ? "New booking from " + (first.customer || "a customer")
            : bookings.length + " new bookings";
        systemNotify(title, serviceText(first) + " \u00b7 " + whenText(first), "/barber/bookings", "booking-" + first.id);
    }

    function announceMessages(messages) {
        chime();
        buzz();
        for (var i = 0; i < messages.length; i++) {
            var message = messages[i];
            showMessageBanner(message);
            var sender = senderText(message);
            systemNotify(
                "New message from " + sender,
                sender + ": " + messagePreview(message),
                message.url,
                "chat-message-" + message.id
            );
        }
    }

    /* ------------------------------------------------------------------
     * Polling
     * ---------------------------------------------------------------- */

    function schedule(ms) {
        if (timer) {
            window.clearTimeout(timer);
        }
        timer = window.setTimeout(poll, ms);
    }

    function updateCursor(body) {
        if (typeof body.latest === "number" && body.latest > bookingCursor) {
            bookingCursor = body.latest;
        }
        if (typeof body.latestMessage === "number" && body.latestMessage > messageCursor) {
            messageCursor = body.latestMessage;
        }
    }

    function freshRows(rows, seen) {
        var fresh = [];
        if (!Array.isArray(rows)) {
            return fresh;
        }
        for (var i = 0; i < rows.length; i++) {
            var row = rows[i];
            if (!row || typeof row.id !== "number" || seen[row.id]) {
                continue;
            }
            seen[row.id] = true;
            fresh.push(row);
        }
        return fresh;
    }

    function poll() {
        if (navigator.onLine === false) {
            schedule(RETRY_MS);
            return;
        }

        var firstCall = !initialized;
        var url = ALERT_ENDPOINT;
        var query = [];
        if (!firstCall) {
            query.push("after=" + encodeURIComponent(bookingCursor));
            query.push("afterMessage=" + encodeURIComponent(messageCursor));
        }
        if (query.length) {
            url += "?" + query.join("&");
        }

        requestJSON(url).then(function (body) {
            if (firstCall) {
                /* Baseline: remember where this device starts and do not
                   replay old bookings or messages. A zero is a real cursor:
                   the next poll must be able to announce the first message or
                   booking this account ever receives. */
                bookingCursor = Number(body.latest) || 0;
                messageCursor = Number(body.latestMessage) || 0;
                initialized = true;
                if (typeof body.pushPublicKey === "string" && body.pushPublicKey) {
                    pushPublicKey = body.pushPublicKey;
                }
                maybePrompt();
                maybeSyncPush();
                schedule(POLL_MS);
                return;
            }

            updateCursor(body);
            if (typeof body.pushPublicKey === "string" && body.pushPublicKey) {
                pushPublicKey = body.pushPublicKey;
            }

            var bookings = freshRows(body.bookings, seenBookings);
            if (bookings.length) {
                announceBookings(bookings);
            }

            var messages = freshRows(body.messages, seenMessages);
            if (messages.length) {
                announceMessages(messages);
            }

            maybePrompt();
            maybeSyncPush();
            schedule(POLL_MS);
        }).catch(function () {
            /* A signed-out or offline phone simply tries again later; a
               failure never disturbs the page. */
            schedule(RETRY_MS);
        });
    }

    /* ------------------------------------------------------------------
     * Web Push opt-in
     * ---------------------------------------------------------------- */

    function preferenceTarget() {
        var button = document.querySelector("[data-theme-switch]");
        if (!button) {
            return null;
        }
        var endpoint = button.getAttribute("data-theme-endpoint");
        var token = button.getAttribute("data-csrf-token");
        if (!endpoint || !token) {
            return null;
        }
        return { endpoint: endpoint, token: token };
    }

    function saveOptIn() {
        var target = preferenceTarget();
        if (!target) {
            return;
        }
        try {
            window.fetch(target.endpoint, {
                method: "PUT",
                credentials: "same-origin",
                headers: {
                    "Content-Type": "application/json",
                    "X-CSRF-Token": target.token
                },
                body: JSON.stringify({ notifyAlerts: true })
            }).catch(function () {});
        } catch (error) {
            /* The browser permission still stands for this device. */
        }
    }

    function urlBase64ToUint8Array(value) {
        var padding = "=".repeat((4 - (value.length % 4)) % 4);
        var base64 = (value + padding).replace(/-/g, "+").replace(/_/g, "/");
        var raw = window.atob(base64);
        var bytes = new Uint8Array(raw.length);
        for (var i = 0; i < raw.length; i++) {
            bytes[i] = raw.charCodeAt(i);
        }
        return bytes;
    }

    function pushSupported() {
        return "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
    }

    function sendSubscription(subscription) {
        var target = preferenceTarget();
        if (!target) {
            return Promise.reject(new Error("no csrf token"));
        }
        var json = typeof subscription.toJSON === "function" ? subscription.toJSON() : subscription;
        return requestJSON(SUBSCRIBE_ENDPOINT, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "X-CSRF-Token": target.token
            },
            body: JSON.stringify({
                endpoint: json.endpoint,
                keys: {
                    p256dh: json.keys && json.keys.p256dh,
                    auth: json.keys && json.keys.auth
                }
            })
        });
    }

    function syncPush() {
        if (!pushPublicKey || !pushSupported() || window.Notification.permission !== "granted") {
            return;
        }
        navigator.serviceWorker.register(SERVICE_WORKER).then(function (registration) {
            return registration.pushManager.getSubscription().then(function (existing) {
                if (existing) {
                    return existing;
                }
                return registration.pushManager.subscribe({
                    userVisibleOnly: true,
                    applicationServerKey: urlBase64ToUint8Array(pushPublicKey)
                });
            });
        }).then(function (subscription) {
            pushSynced = true;
            return sendSubscription(subscription);
        }).catch(function () {
            pushSynced = false;
        });
    }

    function maybeSyncPush() {
        if (!pushSynced) {
            syncPush();
        }
    }

    function removePrompt() {
        var prompt = document.getElementById("notification-alert-prompt");
        if (prompt && prompt.parentNode) {
            prompt.parentNode.removeChild(prompt);
        }
    }

    function showPrompt() {
        if (document.getElementById("notification-alert-prompt")) {
            return;
        }
        ensureStyles();
        var prompt = document.createElement("div");
        prompt.className = "notification-alert-prompt";
        prompt.id = "notification-alert-prompt";

        var message = document.createElement("p");
        var closedApp = Boolean(pushPublicKey) && pushSupported();
        if (role === "barber") {
            message.textContent = closedApp
                ? "Get an alert when a customer books or messages you, even when the app is closed."
                : "Get a sound and notification alert when a customer books or messages you while this page is open.";
        } else {
            message.textContent = closedApp
                ? "Get an alert when a barber replies, even when the app is closed."
                : "Get a sound and notification alert when a barber replies while this page is open.";
        }

        var enable = document.createElement("button");
        enable.type = "button";
        enable.setAttribute("data-alert-enable", "");
        enable.textContent = "Enable";
        enable.addEventListener("click", function () {
            unlockAudio();
            var finish = function (permission) {
                if (permission === "granted") {
                    saveOptIn();
                    pushSynced = false;
                    maybeSyncPush();
                } else {
                    /* A prompt dismissed without a decision should not pop
                       straight back on the next poll. */
                    store(PROMPT_KEY, "dismissed");
                }
                removePrompt();
            };
            if ("Notification" in window && window.Notification.requestPermission) {
                try {
                    window.Notification.requestPermission().then(finish).catch(function () {
                        finish(window.Notification.permission);
                    });
                } catch (error) {
                    finish(window.Notification.permission);
                }
            } else {
                finish("denied");
            }
        });

        var later = document.createElement("button");
        later.type = "button";
        later.textContent = "Not now";
        later.addEventListener("click", function () {
            store(PROMPT_KEY, "dismissed");
            removePrompt();
        });

        prompt.appendChild(message);
        prompt.appendChild(enable);
        prompt.appendChild(later);
        document.body.appendChild(prompt);
    }

    function maybePrompt() {
        if (!("Notification" in window) || window.Notification.permission !== "default") {
            return;
        }
        if (stored(PROMPT_KEY) === "dismissed") {
            return;
        }
        showPrompt();
    }

    /* ------------------------------------------------------------------
     * Start-up
     * ---------------------------------------------------------------- */

    function start() {
        if (!document.body) {
            document.addEventListener("DOMContentLoaded", start);
            return;
        }
        ensureStack();
        poll();

        document.addEventListener("visibilitychange", function () {
            if (!document.hidden) {
                schedule(0);
            }
        });

        window.addEventListener("online", function () {
            schedule(0);
        });
    }

    /* The first gesture unlocks audio for the rest of the visit. */
    ["pointerdown", "keydown", "touchstart"].forEach(function (eventName) {
        document.addEventListener(eventName, unlockAudio, { once: true, passive: true });
    });

    start();
})();

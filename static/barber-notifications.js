"use strict";
/* Live booking alert for the barber's phone.

   A customer's request reaches the barber by email already. This script adds
   the part email cannot: an in-app popup at the top of the screen, a short
   chime and a vibration the moment the booking is created.

   How it works
   ------------
   The page polls /barber/notifications. The first call establishes a
   baseline (the newest booking id); every call after that asks only for
   bookings newer than the last id it saw. Using the id means no request can
   be skipped or delivered twice, even if a poll is slow.

   Sound
   -----
   The chime is synthesised with the Web Audio API, so the app does not ship
   an audio file and nothing copyrighted is involved. Browsers block audio
   until the visitor has interacted with the page, so the context is unlocked
   on the first tap or key press. If it is still locked, the popup and
   vibration still fire.

   Phone notifications
   -------------------
   When the barber taps "Enable phone alerts", the script asks for the
   browser's notification permission and stores the choice in the existing
   preferences record. With permission granted the alert also appears in the
   phone's notification shade while the page is open. A fully closed browser
   needs server-side Web Push, which is out of scope here. */
(function () {
    "use strict";

    var POLL_MS = 20000;
    var RETRY_MS = 45000;
    var PROMPT_KEY = "dgs-booking-alert-prompt";
    var ALERT_ENDPOINT = "/barber/notifications";

    var latest = null;
    var seen = {};
    var timer = null;
    var audioContext = null;
    var audioUnlocked = false;
    var stack = null;

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
            /* The popup still communicates the booking. */
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
        if (document.getElementById("barber-alert-style")) {
            return;
        }
        var style = document.createElement("style");
        style.id = "barber-alert-style";
        style.textContent = [
            ".barber-alert-stack{position:fixed;top:12px;left:50%;transform:translateX(-50%);z-index:2147483000;display:flex;flex-direction:column;gap:10px;width:min(420px,calc(100vw - 24px));pointer-events:none}",
            ".barber-alert{pointer-events:auto;display:flex;align-items:flex-start;gap:12px;padding:14px 14px 14px 16px;border-radius:14px;border:1px solid #d8e0ea;background:#fff;box-shadow:0 18px 40px rgba(12,21,36,.22);font:inherit;color:#0c1524;animation:barber-alert-in .22s ease-out}",
            ".barber-alert-icon{font-size:1.35rem;line-height:1}",
            ".barber-alert-copy{flex:1;min-width:0}",
            ".barber-alert-copy strong{display:block;font-size:.95rem}",
            ".barber-alert-copy span{display:block;margin-top:3px;font-size:.82rem;color:#40516a;line-height:1.4}",
            ".barber-alert-copy small{display:block;margin-top:3px;font-size:.75rem;color:#68778d}",
            ".barber-alert-actions{display:flex;flex-direction:column;gap:6px;align-items:flex-end}",
            ".barber-alert-view{padding:7px 11px;border-radius:8px;background:#142034;color:#fff;font-size:.76rem;font-weight:750;text-decoration:none;white-space:nowrap}",
            ".barber-alert-close{border:0;background:transparent;color:#68778d;font-size:1.1rem;line-height:1;cursor:pointer;padding:2px 4px}",
            ".barber-alert-prompt{position:fixed;left:50%;bottom:16px;transform:translateX(-50%);z-index:2147483000;display:flex;align-items:center;gap:10px;width:min(420px,calc(100vw - 24px));padding:12px 14px;border-radius:13px;border:1px solid #d8e0ea;background:#0c1524;color:#fff;box-shadow:0 18px 40px rgba(12,21,36,.28);font:inherit;font-size:.8rem}",
            ".barber-alert-prompt p{flex:1;margin:0;line-height:1.4}",
            ".barber-alert-prompt button{border:1px solid rgba(255,255,255,.35);background:transparent;color:#fff;border-radius:8px;padding:7px 10px;font:inherit;font-size:.76rem;font-weight:750;cursor:pointer;white-space:nowrap}",
            ".barber-alert-prompt button[data-alert-enable]{background:#fff;color:#0c1524;border-color:#fff}",
            "@keyframes barber-alert-in{from{opacity:0;transform:translateY(-10px)}to{opacity:1;transform:translateY(0)}}",
            "@media (prefers-reduced-motion:reduce){.barber-alert{animation:none}}"
        ].join("");
        document.head.appendChild(style);
    }

    function ensureStack() {
        if (stack && document.body.contains(stack)) {
            return stack;
        }
        ensureStyles();
        stack = document.createElement("div");
        stack.className = "barber-alert-stack";
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

    function showBanner(bookings) {
        var host = ensureStack();
        var card = document.createElement("div");
        card.className = "barber-alert";

        var icon = document.createElement("span");
        icon.className = "barber-alert-icon";
        icon.setAttribute("aria-hidden", "true");
        icon.textContent = "\ud83d\udd14";

        var copy = document.createElement("div");
        copy.className = "barber-alert-copy";
        var title = document.createElement("strong");
        var detail = document.createElement("span");
        var extra = document.createElement("small");

        if (bookings.length === 1) {
            var one = bookings[0];
            title.textContent = "New booking from " + (one.customer || "a customer");
            detail.textContent = serviceText(one) + " \u00b7 " + whenText(one);
            var price = naira(one.price);
            if (price) {
                extra.textContent = price;
            }
        } else {
            title.textContent = bookings.length + " new bookings";
            var first = bookings[0];
            detail.textContent = "Latest: " + (first.customer || "a customer") + " \u00b7 " + serviceText(first) + " \u00b7 " + whenText(first);
            extra.textContent = "Open Bookings to accept or decline them.";
        }

        copy.appendChild(title);
        copy.appendChild(detail);
        if (extra.textContent) {
            copy.appendChild(extra);
        }

        var actions = document.createElement("div");
        actions.className = "barber-alert-actions";

        var view = document.createElement("a");
        view.className = "barber-alert-view";
        view.href = "/barber/bookings";
        view.textContent = "View";

        var close = document.createElement("button");
        close.type = "button";
        close.className = "barber-alert-close";
        close.setAttribute("aria-label", "Dismiss booking alert");
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

    function systemNotify(bookings) {
        if (!("Notification" in window) || window.Notification.permission !== "granted") {
            return;
        }
        var first = bookings[0];
        var title = bookings.length === 1
            ? "New booking from " + (first.customer || "a customer")
            : bookings.length + " new bookings";
        var body = serviceText(first) + " \u00b7 " + whenText(first);
        try {
            var notice = new Notification(title, {
                body: body,
                tag: "barber-booking-" + first.id
            });
            notice.onclick = function () {
                window.focus();
                window.location.href = "/barber/bookings";
                notice.close();
            };
        } catch (error) {
            /* Android Chrome cannot construct Notification directly; the
               in-app popup and sound have already run. */
        }
    }

    function announce(bookings) {
        chime();
        buzz();
        showBanner(bookings);
        systemNotify(bookings);
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

    function poll() {
        if (navigator.onLine === false) {
            schedule(RETRY_MS);
            return;
        }
        var url = ALERT_ENDPOINT;
        if (latest !== null) {
            url += "?after=" + encodeURIComponent(latest);
        }
        requestJSON(url).then(function (body) {
            if (latest === null) {
                /* Baseline: remember where the barber's phone starts and do
                   not replay old requests. */
                latest = Number(body.latest) || 0;
                schedule(POLL_MS);
                return;
            }

            var bookings = Array.isArray(body.bookings) ? body.bookings : [];
            var fresh = [];
            for (var i = 0; i < bookings.length; i++) {
                var booking = bookings[i];
                if (!booking || typeof booking.id !== "number" || seen[booking.id]) {
                    continue;
                }
                seen[booking.id] = true;
                fresh.push(booking);
            }

            if (typeof body.latest === "number" && body.latest > latest) {
                latest = body.latest;
            }

            if (fresh.length) {
                announce(fresh);
            }
            schedule(POLL_MS);
        }).catch(function () {
            /* A signed-out or offline phone simply tries again later; a
               failure never disturbs the page. */
            schedule(RETRY_MS);
        });
    }

    /* ------------------------------------------------------------------
     * Phone alert opt-in
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

    function removePrompt() {
        var prompt = document.getElementById("barber-alert-prompt");
        if (prompt && prompt.parentNode) {
            prompt.parentNode.removeChild(prompt);
        }
    }

    function showPrompt() {
        if (document.getElementById("barber-alert-prompt")) {
            return;
        }
        ensureStyles();
        var prompt = document.createElement("div");
        prompt.className = "barber-alert-prompt";
        prompt.id = "barber-alert-prompt";

        var message = document.createElement("p");
        message.textContent = "Get a sound alert the moment a customer books a haircut.";

        var enable = document.createElement("button");
        enable.type = "button";
        enable.setAttribute("data-alert-enable", "");
        enable.textContent = "Enable";
        enable.addEventListener("click", function () {
            unlockAudio();
            var finish = function () {
                saveOptIn();
                removePrompt();
            };
            if ("Notification" in window && window.Notification.requestPermission) {
                try {
                    window.Notification.requestPermission().then(finish).catch(finish);
                } catch (error) {
                    finish();
                }
            } else {
                finish();
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
        maybePrompt();
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
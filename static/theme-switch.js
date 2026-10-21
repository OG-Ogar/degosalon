"use strict";
/* The colour switch.

   One click moves the app between the navy green of the Forest palette
   and its navy blue twin. The choice is kept in `localStorage` so it
   survives a reload, and it is written to the server record so it
   follows the user to another device.

   The script is deliberately small and defensive:
   - every value it reads is run through the allowlist below, so a
     hand-edited storage entry can never inject an arbitrary token set;
   - a page without the switch button still gets the stored palette, so
     the choice is the same everywhere the user lands;
   - a failed server write never loses the local choice.

   It is loaded in the head of the page, before the first paint, so a
   returning visitor never sees a flash of the default palette. */
(function () {
    "use strict";

    var STORAGE_KEY = "dgs-display-v1";
    /* The themes the server allowlists. Anything else falls back to the
       default palette, exactly as the Go normaliser does. */
    var THEMES = ["forest", "navy", "midnight", "daylight", "ocean", "stone"];
    /* The two palettes the switch moves between. */
    var PAIRS = { forest: "navy", navy: "forest" };
    var NAMES = {
        forest: "navy green",
        navy: "navy blue",
        midnight: "midnight",
        daylight: "daylight",
        ocean: "ocean",
        stone: "stone"
    };

    var root = document.documentElement;
    /* Set when the visitor changes the palette, so a slow server answer
       cannot undo the choice they just made. */
    var changed = false;

    function knownTheme(value) {
        return THEMES.indexOf(value) > -1 ? value : "forest";
    }

    function readStored() {
        try {
            var raw = window.localStorage.getItem(STORAGE_KEY);
            if (!raw) {
                return {};
            }
            var stored = JSON.parse(raw);
            return stored && typeof stored === "object" ? stored : {};
        } catch (error) {
            return {};
        }
    }

    function writeStored(preferences) {
        try {
            window.localStorage.setItem(STORAGE_KEY, JSON.stringify(preferences));
        } catch (error) {
            /* Storage is unavailable (private mode, quota). The choice
               still holds for this page and the server keeps the record. */
        }
    }

    /* applyTheme puts the palette on the document element, keeps the
       browser chrome in step with it, and re-labels the switch so it
       always says which palette comes next. */
    function applyTheme(theme) {
        var next = knownTheme(theme);
        /* The default palette is the one in `:root`, so the page keeps
           its markup untouched until a visitor asks for another one. */
        if (next === "forest") {
            root.removeAttribute("data-theme");
        } else {
            root.setAttribute("data-theme", next);
        }

        var voidColour = window.getComputedStyle(root).getPropertyValue("--void").trim();
        if (voidColour) {
            var metas = document.querySelectorAll('meta[name="theme-color"]');
            for (var i = 0; i < metas.length; i++) {
                metas[i].setAttribute("content", voidColour);
            }
        }

        var target = PAIRS[next] || "navy";
        var label = "Switch colour to " + (NAMES[target] || target);
        var switches = document.querySelectorAll("[data-theme-switch]");
        for (var j = 0; j < switches.length; j++) {
            switches[j].setAttribute("data-current", next);
            switches[j].setAttribute("title", label);
            switches[j].setAttribute("aria-label", label);
        }
        return next;
    }

    /* The switch carries the address of the preference record and the
       token that authorises the write, so the script needs no secrets
       of its own. */
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

    function saveToServer(theme) {
        var target = preferenceTarget();
        if (!target || typeof window.fetch !== "function") {
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
                body: JSON.stringify({ theme: theme })
            }).catch(function () {
                /* The local choice stands; the next change tries again. */
            });
        } catch (error) {
            /* The request never left the page; the local choice stands. */
        }
    }

    /* The server record is the source of truth once a visitor is signed
       in, so the stored palette is checked against it on arrival. */
    function loadFromServer() {
        var target = preferenceTarget();
        if (!target || typeof window.fetch !== "function") {
            return;
        }
        window.fetch(target.endpoint, {
            credentials: "same-origin",
            headers: { "X-CSRF-Token": target.token }
        }).then(function (response) {
            return response.ok ? response.json() : null;
        }).then(function (body) {
            if (!body || !body.theme || changed) {
                return;
            }
            var theme = knownTheme(body.theme);
            if (theme !== root.getAttribute("data-theme")) {
                applyTheme(theme);
            }
            var preferences = readStored();
            preferences.theme = theme;
            writeStored(preferences);
        }).catch(function () {
            /* The local choice stands when the server cannot be reached. */
        });
    }

    /* The click is caught on the document, so the switch works wherever
       it sits in the page — in the drawer panel or in the header bar. */
    document.addEventListener("click", function (event) {
        var target = event.target;
        while (target && target !== document) {
            if (target.getAttribute && target.getAttribute("data-theme-switch") !== null) {
                break;
            }
            target = target.parentNode;
        }
        if (!target || target === document) {
            return;
        }
        event.preventDefault();
        changed = true;

        var current = knownTheme(root.getAttribute("data-theme") || readStored().theme);
        var next = PAIRS[current] || "navy";
        applyTheme(next);

        var preferences = readStored();
        preferences.theme = next;
        writeStored(preferences);
        saveToServer(next);
    });

    function boot() {
        var stored = readStored();
        var theme = knownTheme(stored.theme || root.getAttribute("data-theme") || "forest");
        applyTheme(theme);

        if (document.readyState === "loading") {
            document.addEventListener("DOMContentLoaded", function () {
                /* The switch is in the markup by now, so it can be
                   labelled with the palette that comes next. */
                applyTheme(root.getAttribute("data-theme") || theme);
                loadFromServer();
            });
            return;
        }
        loadFromServer();
    }

    boot();
})();

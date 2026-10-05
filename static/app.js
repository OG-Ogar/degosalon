"use strict";
/* Typed interactive layer for De Go Salon.

   Every page still works with this file blocked or broken: the server
   validates and renders everything on its own, and nothing below adds a
   feature that only exists in the browser. It makes the same pages
   quicker to use, nothing more. */
(function () {
    "use strict";
    /* ---- password fields ---- */
    /* Puts a Show/Hide button beside a password box. Typing a password
       nobody can read is where most sign-in mistakes come from. */
    function addRevealButton(input) {
        var _a;
        var wrap = document.createElement("div");
        wrap.className = "pw";
        var button = document.createElement("button");
        button.type = "button";
        button.className = "small reveal";
        button.textContent = "Show";
        button.setAttribute("aria-label", "Show password");
        button.addEventListener("click", function () {
            var hidden = input.type === "password";
            var next = hidden ? "Hide" : "Show";
            input.type = hidden ? "text" : "password";
            button.textContent = next;
            button.setAttribute("aria-label", next + " password");
            input.focus();
        });
        (_a = input.parentNode) === null || _a === void 0 ? void 0 : _a.insertBefore(wrap, input);
        wrap.appendChild(input);
        wrap.appendChild(button);
    }
    /* The line of feedback under one field, made on first use and reused
       afterwards so repeated typing does not stack up notes. */
    function noteFor(input) {
        const cached = input.dataset.fieldNoteId;
        if (cached) {
            const existing = document.getElementById(cached);
            if (existing instanceof HTMLParagraphElement)
                return existing;
        }
        {
            var note = document.createElement("p");
            note.className = "field-note";
            note.setAttribute("aria-live", "polite");
            (input.closest(".pw") || input).insertAdjacentElement("afterend", note);
            note.id = "field-note-" + Math.random().toString(36).slice(2);
            input.dataset.fieldNoteId = note.id;
        }
        return document.getElementById(input.dataset.fieldNoteId || "");
    }
    /* Says whether a new password is long enough and whether the
       confirmation matches it, while it is being typed. The server checks
       both again; this only saves a round trip. */
    function watchNewPassword(form) {
        var password = form.querySelector('input[autocomplete="new-password"][name="password"]');
        var confirm = form.querySelector('input[name="confirm_password"]');
        if (!password) {
            return;
        }
        var minLength = parseInt(password.getAttribute("minlength") || "", 10) || 8;
        var passwordInput = password;
        function say(input, text, state) {
            var note = noteFor(input);
            note.textContent = text;
            note.className = state ? "field-note " + state : "field-note";
        }
        function check() {
            var short = minLength - passwordInput.value.length;
            if (!passwordInput.value) {
                say(passwordInput, "");
            }
            else if (short > 0) {
                say(passwordInput, short + (short === 1 ? " more character" : " more characters") +
                    " needed.", "warn");
            }
            else {
                say(passwordInput, "Long enough.", "ok");
            }
            if (!confirm) {
                return;
            }
            if (!confirm.value) {
                say(confirm, "");
            }
            else if (confirm.value === passwordInput.value) {
                say(confirm, "Both passwords match.", "ok");
            }
            else {
                say(confirm, "The two passwords do not match.", "warn");
            }
        }
        passwordInput.addEventListener("input", check);
        if (confirm) {
            confirm.addEventListener("input", check);
        }
    }
    /* ---- lists ---- */
    /* Hides the rows or cards that do not contain what is typed. The box
       carries data-filter with a selector for the things to filter, so
       one function serves tables and card grids alike. */
    function wireFilter(box) {
        var selector = box.getAttribute("data-filter");
        if (!selector)
            return;
        var targets = document.querySelectorAll(selector);
        if (!targets.length) {
            return;
        }
        var status = document.createElement("p");
        status.className = "hint";
        status.setAttribute("aria-live", "polite");
        box.insertAdjacentElement("afterend", status);
        box.addEventListener("input", function () {
            var needle = box.value.trim().toLowerCase();
            var shown = 0;
            targets.forEach(function (target) {
                var match = !needle ||
                    target.textContent.toLowerCase().indexOf(needle) !== -1;
                target.hidden = !match;
                if (match) {
                    shown += 1;
                }
            });
            status.textContent = needle
                ? shown + " of " + targets.length + " shown"
                : "";
        });
    }
    /* ---- chat ---- */
    /* Enter sends a message and Shift+Enter starts a new line, which is
       what a message box is expected to do. Submitting through the form
       keeps the guards below in play. */
    function sendOnEnter(form, box) {
        box.addEventListener("keydown", function (event) {
            if (event.key !== "Enter" || event.shiftKey || !box.value.trim()) {
                return;
            }
            event.preventDefault();
            if (form.requestSubmit) {
                form.requestSubmit();
            }
            else {
                form.submit();
            }
        });
    }
    /* ---- forms ---- */
    /* How much room is left in a capped text box. */
    function addCounter(box) {
        var limit = parseInt(box.getAttribute("maxlength") || "", 10);
        if (!limit) {
            return;
        }
        var counter = document.createElement("p");
        counter.className = "counter";
        counter.setAttribute("aria-live", "polite");
        box.insertAdjacentElement("afterend", counter);
        function update() {
            counter.textContent = (limit - box.value.length) + " characters left";
        }
        box.addEventListener("input", update);
        update();
    }
    /* Shows the picture a file input is holding, at the size the finished
       page will show it, and says what the server is going to do with an
       oversized one. An input carrying data-preview names the class to
       show it under, so a round picture is previewed round. The upload
       works the same without any of this. */
    function previewChosenPhoto(input) {
        var preview = document.createElement("img");
        preview.className = input.getAttribute("data-preview") || "photo";
        preview.alt = "The photo you just chose";
        preview.hidden = true;
        input.insertAdjacentElement("afterend", preview);
        input.addEventListener("change", function () {
            var file = input.files && input.files[0];
            var note = noteFor(input);
            if (preview.src) {
                URL.revokeObjectURL(preview.src);
            }
            if (!file) {
                preview.hidden = true;
                preview.removeAttribute("src");
                note.textContent = "";
                return;
            }
            // A file the browser does not call an image gets no preview:
            // the server refuses it anyway, and an <img> pointed at a
            // non-image would only show a broken icon.
            if (file.type.indexOf("image/") !== 0) {
                preview.hidden = true;
                preview.removeAttribute("src");
                note.textContent = file.name + " is not an image.";
                note.className = "field-note warn";
                return;
            }
            preview.src = URL.createObjectURL(file);
            preview.hidden = false;
            var imageFile = file;
            preview.onload = function () {
                var size = Math.round(imageFile.size / 1024) + " KB";
                var shape = preview.naturalWidth + " by " + preview.naturalHeight;
                var big = Math.max(preview.naturalWidth, preview.naturalHeight) > 1600;
                note.textContent = shape + " pixels, " + size +
                    (big ? " - this will be resized when you save." : ".");
                note.className = "field-note" + (big ? " warn" : " ok");
            };
        });
    }
    /* Asks before something that cannot be undone. The wording lives in
       the form's data-confirm attribute, so each page words its own. */
    function confirmBeforeSubmit(form) {
        form.addEventListener("submit", function (event) {
            if (!window.confirm(form.getAttribute("data-confirm") || "")) {
                event.preventDefault();
            }
        });
    }
    /* One submit per form. An impatient second click on a slow connection
       could otherwise send the same booking or message twice.

       The buttons are disabled after the browser has already collected the
       form, and released again if the page comes back from history, so
       going Back does not leave a form that cannot be sent. */
    function guardDoubleSubmit(form) {
        form.addEventListener("submit", function (event) {
            if (event.defaultPrevented) {
                return;
            }
            window.setTimeout(function () {
                var buttons = form.querySelectorAll("button:not([type=button])");
                buttons.forEach(function (button) {
                    button.disabled = true;
                    button.classList.add("busy");
                });
            }, 0);
        });
    }
    /* Names what the chosen style costs and how long it takes, so the
       price is visible at the moment of choosing rather than only in the
       list above. The figures come from the radio's data attributes, which
       the server rendered from the same hairstyle record. */
    function wireBookingSummary(form) {
        var styles = form.querySelectorAll('input[name="hairstyle_id"][data-price]');
        var actions = form.querySelector(".actions");
        if (!styles.length || !actions) {
            return;
        }
        var summary = document.createElement("p");
        summary.className = "notice";
        summary.setAttribute("aria-live", "polite");
        summary.hidden = true;
        actions.insertAdjacentElement("beforebegin", summary);
        function show(style) {
            summary.hidden = false;
            summary.textContent = "Booking " + style.getAttribute("data-name") +
                " at " + style.getAttribute("data-price") +
                ", about " + style.getAttribute("data-duration") + " minutes.";
        }
        styles.forEach(function (style) {
            style.addEventListener("change", function () {
                show(style);
            });
            if (style.checked) {
                show(style);
            }
        });
    }
    /* ---- wiring ---- */
    function init() {
        document.querySelectorAll('input[type="password"]').forEach(addRevealButton);
        document.querySelectorAll("form").forEach(function (form) {
            watchNewPassword(form);
            wireBookingSummary(form);
            // Before the submit guard, so a cancelled confirmation leaves
            // the buttons alone.
            if (form.hasAttribute("data-confirm")) {
                confirmBeforeSubmit(form);
            }
            guardDoubleSubmit(form);
        });
        document.querySelectorAll("[data-filter]").forEach(wireFilter);
        document.querySelectorAll("textarea[maxlength]").forEach(addCounter);
        document.querySelectorAll('input[type="file"]').forEach(previewChosenPhoto);
        // A conversation opens on its newest message, the way every other
        // chat does, rather than at the oldest one.
        var thread = document.querySelector(".thread");
        if (thread) {
            thread.scrollTop = thread.scrollHeight;
        }
        var send = document.querySelector("form.send");
        var box = send && send.querySelector("textarea");
        if (send && box) {
            sendOnEnter(send, box);
            box.focus();
        }
    }
    // Buttons disabled by the submit guard are freed when the browser
    // restores this page from its history cache.
    window.addEventListener("pageshow", function () {
        document.querySelectorAll("button.busy").forEach(function (button) {
            button.disabled = false;
            button.classList.remove("busy");
        });
    });
    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", init);
    }
    else {
        init();
    }
})();

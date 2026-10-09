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
            input.setAttribute("aria-describedby", [input.getAttribute("aria-describedby"), note.id].filter(Boolean).join(" "));
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
            input.setAttribute("aria-invalid", state === "warn" ? "true" : "false");
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
            if (event.key !== "Enter" || event.shiftKey || event.isComposing || event.keyCode === 229 || !box.value.trim()) {
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
    /* Auto-expanding textarea composer (Messenger & X DM standard). */
    function wireAutoExpandTextarea(box) {
        function adjust() {
            box.style.height = "auto";
            var nextHeight = Math.min(Math.max(box.scrollHeight, 48), 160);
            box.style.height = nextHeight + "px";
        }
        box.addEventListener("input", adjust);
        adjust();
    }
    /* Floating scroll-to-bottom indicator with smooth scroll jump. */
    function wireScrollToBottom(thread) {
        var button = document.createElement("button");
        button.type = "button";
        button.className = "chat-scroll-bottom";
        button.setAttribute("aria-label", "Scroll to latest messages");
        button.innerHTML = '<span aria-hidden="true">↓</span> Newest';
        thread.insertAdjacentElement("afterend", button);
        function checkScroll() {
            var distanceFromBottom = thread.scrollHeight - thread.scrollTop - thread.clientHeight;
            if (distanceFromBottom > 120) {
                button.classList.add("visible");
            }
            else {
                button.classList.remove("visible");
            }
        }
        thread.addEventListener("scroll", checkScroll, { passive: true });
        button.addEventListener("click", function () {
            thread.scrollTo({ top: thread.scrollHeight, behavior: "smooth" });
            button.classList.remove("visible");
        });
    }
    function enhanceChat() {
        var thread = document.querySelector(".thread");
        if (thread) {
            thread.scrollTop = thread.scrollHeight;
            wireScrollToBottom(thread);
        }
        var send = document.querySelector("form.send");
        var box = send && send.querySelector("textarea");
        if (send && box) {
            sendOnEnter(send, box);
            wireAutoExpandTextarea(box);
            if (window.matchMedia("(hover: hover) and (pointer: fine)").matches)
                box.focus();
        }
    }
    /* ---- social share ---- */
    /* Opens native Web Share API or accessible modal fallback with 1-click
       links for X (Twitter), Facebook, WhatsApp, and clipboard copy. */
    function wireSocialShare() {
        var shareButtons = document.querySelectorAll("[data-share]");
        if (!shareButtons.length)
            return;
        var shareDialog = null;
        var lastShareOpener = null;
        function getOrCreateShareDialog() {
            if (shareDialog)
                return shareDialog;
            shareDialog = document.createElement("dialog");
            shareDialog.className = "share-dialog";
            shareDialog.setAttribute("aria-label", "Share options");
            shareDialog.innerHTML =
                '<div class="share-dialog-content">' +
                    '<div class="share-dialog-header">' +
                    '<h2 class="share-dialog-title">Share this page</h2>' +
                    '<button type="button" class="share-dialog-close" aria-label="Close share options">&times;</button>' +
                    '</div>' +
                    '<div class="share-dialog-body">' +
                    '<p class="share-dialog-desc"></p>' +
                    '<div class="share-social-grid">' +
                    '<a class="share-social-link share-x" target="_blank" rel="noopener noreferrer">' +
                    '<span class="share-social-icon" aria-hidden="true">𝕏</span>' +
                    '<span>Post on X</span>' +
                    '</a>' +
                    '<a class="share-social-link share-fb" target="_blank" rel="noopener noreferrer">' +
                    '<span class="share-social-icon" aria-hidden="true">f</span>' +
                    '<span>Facebook</span>' +
                    '</a>' +
                    '<a class="share-social-link share-wa" target="_blank" rel="noopener noreferrer">' +
                    '<span class="share-social-icon" aria-hidden="true">💬</span>' +
                    '<span>WhatsApp</span>' +
                    '</a>' +
                    '</div>' +
                    '<div class="share-copy-row">' +
                    '<label class="visually-hidden" for="share-copy-url">Link URL</label>' +
                    '<input type="text" id="share-copy-url" class="share-copy-input" readonly>' +
                    '<button type="button" class="primary share-copy-btn">Copy Link</button>' +
                    '</div>' +
                    '<p class="share-feedback" role="status" aria-live="polite"></p>' +
                    '</div>' +
                    '</div>';
            document.body.appendChild(shareDialog);
            var closeBtn = shareDialog.querySelector(".share-dialog-close");
            closeBtn === null || closeBtn === void 0 ? void 0 : closeBtn.addEventListener("click", function () {
                shareDialog === null || shareDialog === void 0 ? void 0 : shareDialog.close();
            });
            shareDialog.addEventListener("click", function (event) {
                if (event.target === shareDialog) {
                    shareDialog === null || shareDialog === void 0 ? void 0 : shareDialog.close();
                }
            });
            shareDialog.addEventListener("close", function () {
                var feedback = shareDialog === null || shareDialog === void 0 ? void 0 : shareDialog.querySelector(".share-feedback");
                if (feedback)
                    feedback.textContent = "";
                if (lastShareOpener && lastShareOpener.isConnected) {
                    lastShareOpener.focus();
                }
            });
            var copyBtn = shareDialog.querySelector(".share-copy-btn");
            var copyInput = shareDialog.querySelector(".share-copy-input");
            var feedback = shareDialog.querySelector(".share-feedback");
            copyBtn === null || copyBtn === void 0 ? void 0 : copyBtn.addEventListener("click", function () {
                if (!copyInput)
                    return;
                var inputEl = copyInput;
                inputEl.select();
                if (navigator.clipboard && navigator.clipboard.writeText) {
                    navigator.clipboard.writeText(inputEl.value).then(function () {
                        if (copyBtn)
                            copyBtn.textContent = "Copied! ✓";
                        if (feedback)
                            feedback.textContent = "Link copied to clipboard.";
                        window.setTimeout(function () {
                            if (copyBtn)
                                copyBtn.textContent = "Copy Link";
                        }, 2500);
                    }).catch(function () {
                        fallbackCopy(inputEl);
                    });
                }
                else {
                    fallbackCopy(inputEl);
                }
            });
            function fallbackCopy(input) {
                try {
                    document.execCommand("copy");
                    if (copyBtn)
                        copyBtn.textContent = "Copied! ✓";
                    if (feedback)
                        feedback.textContent = "Link copied to clipboard.";
                    window.setTimeout(function () {
                        if (copyBtn)
                            copyBtn.textContent = "Copy Link";
                    }, 2500);
                }
                catch (_a) {
                    if (feedback)
                        feedback.textContent = "Please copy link manually.";
                }
            }
            return shareDialog;
        }
        function openFallbackModal(title, text, url, trigger) {
            lastShareOpener = trigger;
            var dialog = getOrCreateShareDialog();
            var desc = dialog.querySelector(".share-dialog-desc");
            var xLink = dialog.querySelector(".share-x");
            var fbLink = dialog.querySelector(".share-fb");
            var waLink = dialog.querySelector(".share-wa");
            var copyInput = dialog.querySelector(".share-copy-input");
            if (desc)
                desc.textContent = title;
            if (copyInput)
                copyInput.value = url;
            var shareText = (text ? text + " - " : "") + title;
            if (xLink)
                xLink.href = "https://twitter.com/intent/tweet?text=" + encodeURIComponent(shareText) + "&url=" + encodeURIComponent(url);
            if (fbLink)
                fbLink.href = "https://www.facebook.com/sharer/sharer.php?u=" + encodeURIComponent(url);
            if (waLink)
                waLink.href = "https://api.whatsapp.com/send?text=" + encodeURIComponent(shareText + " " + url);
            if (typeof dialog.showModal === "function") {
                dialog.showModal();
            }
            else {
                dialog.setAttribute("open", "");
            }
            var closeBtn = dialog.querySelector(".share-dialog-close");
            closeBtn === null || closeBtn === void 0 ? void 0 : closeBtn.focus();
        }
        shareButtons.forEach(function (btn) {
            btn.addEventListener("click", function () {
                var _a;
                var title = btn.getAttribute("data-share-title") || document.title;
                var text = btn.getAttribute("data-share-text") ||
                    ((_a = document.querySelector('meta[name="description"]')) === null || _a === void 0 ? void 0 : _a.content) || "";
                var url = btn.getAttribute("data-share-url") || window.location.href;
                if (url.startsWith("/")) {
                    url = window.location.origin + url;
                }
                if (navigator.share) {
                    navigator.share({ title: title, text: text, url: url }).catch(function (err) {
                        if (err && err.name !== "AbortError") {
                            openFallbackModal(title, text, url, btn);
                        }
                    });
                }
                else {
                    openFallbackModal(title, text, url, btn);
                }
            });
        });
    }
    /* ---- sticky mobile action bar ---- */
    /* On mobile viewports, when the hero CTA or trigger scrolls above the
       viewport, slides in a sticky bottom action bar. */
    function wireStickyMobileBar() {
        var bar = document.querySelector(".mobile-sticky-action-bar, [data-sticky-action-bar]");
        if (!bar)
            return;
        var trigger = document.querySelector("[data-sticky-trigger], .shop-hero, .barber-card");
        if (!trigger)
            return;
        const stickyBar = bar;
        const stickyTrigger = trigger;
        if (typeof IntersectionObserver !== "undefined") {
            var observer = new IntersectionObserver(function (entries) {
                entries.forEach(function (entry) {
                    var scrolledPast = !entry.isIntersecting && entry.boundingClientRect.top < 0;
                    if (scrolledPast) {
                        stickyBar.classList.add("visible");
                        stickyBar.removeAttribute("aria-hidden");
                    }
                    else {
                        stickyBar.classList.remove("visible");
                        stickyBar.setAttribute("aria-hidden", "true");
                    }
                });
            }, { threshold: 0.1 });
            observer.observe(stickyTrigger);
        }
        else {
            const onScroll = function () {
                var rect = stickyTrigger.getBoundingClientRect();
                if (rect.bottom < 0) {
                    stickyBar.classList.add("visible");
                }
                else {
                    stickyBar.classList.remove("visible");
                }
            };
            window.addEventListener("scroll", onScroll, { passive: true });
            onScroll();
        }
    }
    /* ---- gallery keyboard navigation ---- */
    /* Enhances any active image-viewer overlay with keyboard arrows and
       touch gestures if not already wired by standalone script. */
    function wireGalleryNavigation() {
        var dialog = document.querySelector(".image-viewer-overlay");
        if (!dialog)
            return;
        const overlay = dialog;
        if (overlay.dataset.galleryEnhanced === "true")
            return;
        overlay.dataset.galleryEnhanced = "true";
        overlay.addEventListener("keydown", function (event) {
            if (event.key === "ArrowRight") {
                var nextBtn = overlay.querySelector(".image-viewer-next");
                if (nextBtn && !nextBtn.hidden) {
                    event.preventDefault();
                    nextBtn.click();
                }
            }
            else if (event.key === "ArrowLeft") {
                var prevBtn = overlay.querySelector(".image-viewer-prev");
                if (prevBtn && !prevBtn.hidden) {
                    event.preventDefault();
                    prevBtn.click();
                }
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
        counter.id = "counter-" + Math.random().toString(36).slice(2);
        box.setAttribute("aria-describedby", [box.getAttribute("aria-describedby"), counter.id].filter(Boolean).join(" "));
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
            if (form.dataset.submitting === "true") {
                event.preventDefault();
                return;
            }
            if (event.defaultPrevented) {
                return;
            }
            form.dataset.submitting = "true";
            form.setAttribute("aria-busy", "true");
            var status = document.createElement("p");
            status.className = "submit-status";
            status.setAttribute("role", "status");
            status.textContent = "Submitting. Please wait…";
            form.appendChild(status);
            // Native submission collects submitter values before this visual disable.
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
        var summary = form.querySelector("[data-booking-summary]");
        if (!styles.length || !summary) {
            return;
        }
        var bookingSummary = summary;
        function show(style) {
            bookingSummary.textContent = "Selected service: " + style.getAttribute("data-name") +
                " · " + new Intl.NumberFormat("en-NG", { style: "currency", currency: "NGN" }).format(Number(style.getAttribute("data-price"))) +
                " · " + style.getAttribute("data-duration") + " minutes.";
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
    function wireFieldFeedback(form) {
        form.querySelectorAll("input:not([type=hidden]),select,textarea").forEach(function (control) {
            control.addEventListener("invalid", function () {
                noteFor(control).textContent = control.validationMessage;
                control.setAttribute("aria-invalid", "true");
            });
            function clearNativeError() {
                if (control.getAttribute("aria-invalid") === "true" && control.validity.valid) {
                    control.removeAttribute("aria-invalid");
                    noteFor(control).textContent = "";
                }
            }
            control.addEventListener("input", clearNativeError);
            control.addEventListener("change", clearNativeError);
        });
    }
    /* ---- wiring ---- */
    // Keep native optgroups as the no-script fallback; the server validates the pair.
    function enhanceGeography() {
        document.querySelectorAll("select[data-geography-state]").forEach(function (state) {
            var _a;
            var area = (_a = state.form) === null || _a === void 0 ? void 0 : _a.querySelector("select[data-geography-area]");
            if (!area)
                return;
            var groups = Array.from(area.querySelectorAll("optgroup")).map(group => group.cloneNode(true));
            function refresh() {
                if (!area)
                    return;
                var selected = area.value;
                area.querySelectorAll("optgroup").forEach(group => group.remove());
                groups.filter(group => group.dataset.stateId === state.value).forEach(group => area === null || area === void 0 ? void 0 : area.appendChild(group.cloneNode(true)));
                area.value = Array.from(area.options).some(option => option.value === selected) ? selected : "";
            }
            refresh();
            state.addEventListener("change", refresh);
        });
    }
    function init() {
        enhanceGeography();
        document.querySelectorAll('input[type="password"]').forEach(addRevealButton);
        document.querySelectorAll("form").forEach(function (form) {
            wireFieldFeedback(form);
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
        enhanceChat();
        wireSocialShare();
        wireStickyMobileBar();
        wireGalleryNavigation();
    }
    // Buttons disabled by the submit guard are freed when the browser
    // restores this page from its history cache.
    window.addEventListener("pageshow", function () {
        document.querySelectorAll("form[data-submitting]").forEach(function (form) {
            var _a;
            delete form.dataset.submitting;
            form.removeAttribute("aria-busy");
            (_a = form.querySelector(".submit-status")) === null || _a === void 0 ? void 0 : _a.remove();
        });
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

"use strict";
/* Continue with Google on the sign-in pages.

   When the visitor has already typed or been shown an email address, the
   address travels to Google as a login hint. Google then opens the account
   chooser on the account that matches it, while still letting the visitor
   pick another Google account already on the device or add a new one. */
(function () {
    "use strict";

    function emailFromForm() {
        var input = document.querySelector('input[name="email"]');
        if (!input) {
            return "";
        }
        return String(input.value || "").trim();
    }

    function lookLikeEmail(value) {
        return value.indexOf("@") > 0 && value.indexOf(" ") === -1;
    }

    function withLoginHint(link, email) {
        var target = new URL(link.href, window.location.origin);
        if (email && lookLikeEmail(email)) {
            target.searchParams.set("login_hint", email);
        } else {
            target.searchParams.delete("login_hint");
        }
        link.href = target.pathname + target.search + target.hash;
    }

    document.querySelectorAll("[data-google-auth]").forEach(function (link) {
        link.addEventListener("click", function () {
            withLoginHint(link, emailFromForm());
        });
    });
})();

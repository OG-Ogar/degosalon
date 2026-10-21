"use strict";
/* Role navigation drawer.

   The links of the shared role header wait behind the "=" button at the
   top left of every customer and barber page. Clicking it slides a panel
   in from the left edge with the links lined up from top to bottom.

   The links are ordinary links in the markup, so a page that never runs
   this file keeps the navigation it has always had: the class this file
   adds to the document element is what turns the header into a drawer.
   Every control here is an enhancement of markup that already works. */
(function () {
    "use strict";
    document.documentElement.classList.add("js-drawer");

    function start() {
        var toggle = document.querySelector(".site-nav-toggle");
        if (!toggle) {
            return;
        }
        var panel = document.getElementById(toggle.getAttribute("aria-controls") || "");
        if (!panel) {
            return;
        }
        var scrim = document.querySelector(".site-nav-scrim");
        var header = document.querySelector(".site-header");
        var role = toggle.getAttribute("data-nav-role") || "menu";
        var links = Array.prototype.slice.call(panel.querySelectorAll("a"));
        var open = false;

        /* The panel sits under the sticky header, so it starts where the
           header ends rather than over the brand and the button. */
        function syncPanelOffset() {
            if (!header) {
                return;
            }
            var height = Math.round(header.getBoundingClientRect().height);
            if (height > 0) {
                panel.style.setProperty("--nav-drawer-top", height + "px");
            }
        }

        function setOpen(next) {
            open = next;
            panel.classList.toggle("is-open", open);
            if (scrim) {
                scrim.hidden = !open;
            }
            toggle.setAttribute("aria-expanded", open ? "true" : "false");
            toggle.setAttribute("aria-label", (open ? "Close the " : "Open the ") + role + " menu");
            document.body.classList.toggle("site-nav-open", open);
            /* A closed panel sits off the left edge, so its links leave the
               tab order until the panel is opened again. */
            links.forEach(function (link) {
                if (open) {
                    link.removeAttribute("tabindex");
                }
                else {
                    link.setAttribute("tabindex", "-1");
                }
            });
            if (open) {
                syncPanelOffset();
            }
        }

        function close(returnFocus) {
            if (!open) {
                return;
            }
            setOpen(false);
            if (returnFocus) {
                toggle.focus();
            }
        }

        toggle.addEventListener("click", function () {
            if (open) {
                close(false);
                return;
            }
            setOpen(true);
            if (links.length) {
                links[0].focus();
            }
        });

        if (scrim) {
            scrim.addEventListener("click", function () {
                close(true);
            });
        }

        document.addEventListener("keydown", function (event) {
            if (!open) {
                return;
            }
            if (event.key === "Escape") {
                event.preventDefault();
                close(true);
                return;
            }
            if (event.key !== "Tab") {
                return;
            }
            /* The panel and the button that opened it are the whole of the
               drawer, so the focus stays between them. */
            var ring = [toggle].concat(links);
            var first = ring[0];
            var last = ring[ring.length - 1];
            if (event.shiftKey && document.activeElement === first) {
                event.preventDefault();
                last.focus();
            }
            else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault();
                first.focus();
            }
        });

        window.addEventListener("resize", syncPanelOffset);
        setOpen(false);
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", start);
    }
    else {
        start();
    }
})();

document.addEventListener("DOMContentLoaded", () => {
    const dialog = document.createElement("dialog");
    dialog.className = "image-viewer-overlay";
    dialog.setAttribute("aria-label", "Full-size image");
    dialog.innerHTML = '<button class="image-viewer-close" type="button" aria-label="Close image">&times;</button>' +
        '<button class="image-viewer-prev" type="button" tabindex="-1" aria-label="Previous image" hidden>&#8249;</button>' +
        '<div class="image-viewer-stage">' +
            '<img class="image-viewer-image" alt="">' +
            '<div class="image-viewer-meta" hidden>' +
                '<span class="image-viewer-counter"></span>' +
                '<span class="image-viewer-caption"></span>' +
            '</div>' +
        '</div>' +
        '<button class="image-viewer-next" type="button" tabindex="-1" aria-label="Next image" hidden>&#8250;</button>';
    document.body.appendChild(dialog);
    const viewerImage = dialog.querySelector("img");
    const closeButton = dialog.querySelector("button");
    const prevButton = dialog.querySelector(".image-viewer-prev");
    const nextButton = dialog.querySelector(".image-viewer-next");
    const metaBox = dialog.querySelector(".image-viewer-meta");
    const counterSpan = dialog.querySelector(".image-viewer-counter");
    const captionSpan = dialog.querySelector(".image-viewer-caption");
    let opener = null;
    let currentIndex = -1;
    const gallery = [];

    function updateView() {
        if (currentIndex < 0 || currentIndex >= gallery.length) return;
        const item = gallery[currentIndex];
        viewerImage.src = item.image.src;
        viewerImage.alt = item.image.alt || "Full-size image";
        if (gallery.length > 1) {
            if (prevButton) prevButton.hidden = false;
            if (nextButton) nextButton.hidden = false;
            if (metaBox) metaBox.hidden = false;
            if (counterSpan) counterSpan.textContent = (currentIndex + 1) + " of " + gallery.length;
            if (captionSpan) captionSpan.textContent = item.image.alt ? " · " + item.image.alt : "";
        } else {
            if (prevButton) prevButton.hidden = true;
            if (nextButton) nextButton.hidden = true;
            if (metaBox) metaBox.hidden = true;
        }
    }

    function showNext() {
        if (gallery.length <= 1) return;
        currentIndex = (currentIndex + 1) % gallery.length;
        updateView();
    }

    function showPrev() {
        if (gallery.length <= 1) return;
        currentIndex = (currentIndex - 1 + gallery.length) % gallery.length;
        updateView();
    }

    function openViewer(index) {
        currentIndex = index;
        updateView();
        dialog.showModal();
        document.body.classList.add("image-viewer-open");
        closeButton.focus();
    }

    // Keep profile links as navigation; enlargement is a separate action there.
    // Any other small photo can also be tapped directly to zoom, and every
    // zoomable photo keeps a labelled button so the action stays reachable by
    // keyboard and screen readers.
    document.querySelectorAll("img[data-fullscreen-image]").forEach((image) => {
        if (image.closest("a")) return;
        image.removeAttribute("data-fullscreen-image");

        const button = document.createElement("button");
        button.type = "button";
        button.className = "image-viewer-trigger";
        button.innerHTML = '<span class="image-viewer-trigger-icon" aria-hidden="true">&#128269;</span>' +
            '<span class="image-viewer-trigger-text">Enlarge image</span>';
        button.setAttribute("aria-label", "Enlarge " + (image.alt || "image"));
        button.setAttribute("aria-haspopup", "dialog");
        const label = image.closest("label");
        if (label) label.after(button);
        else image.after(button);

        const index = gallery.length;
        gallery.push({ image, button });

        button.addEventListener("click", () => {
            opener = button;
            openViewer(index);
        });

        // Inside a selection label a tap chooses that service, so only the
        // labelled button zooms there. Everywhere else the photo itself zooms.
        if (!label) {
            image.classList.add("image-zoomable");
            image.addEventListener("click", () => {
                opener = button;
                openViewer(index);
            });
        }
    });

    closeButton.addEventListener("click", () => dialog.close());
    if (prevButton) {
        prevButton.addEventListener("click", (event) => {
            event.stopPropagation();
            showPrev();
        });
    }
    if (nextButton) {
        nextButton.addEventListener("click", (event) => {
            event.stopPropagation();
            showNext();
        });
    }

    dialog.addEventListener("click", (event) => {
        if (event.target === dialog) {
            const bounds = dialog.getBoundingClientRect();
            if (event.clientX < bounds.left || event.clientX > bounds.right ||
                event.clientY < bounds.top || event.clientY > bounds.bottom) dialog.close();
        }
    });
    dialog.addEventListener("close", () => {
        viewerImage.removeAttribute("src");
        document.body.classList.remove("image-viewer-open");
        const returnTarget = (currentIndex >= 0 && gallery[currentIndex] && gallery[currentIndex].button.isConnected)
            ? gallery[currentIndex].button
            : opener;
        if (returnTarget && returnTarget.isConnected) returnTarget.focus();
    });
    dialog.addEventListener("keydown", (event) => {
        if (event.key === "Tab") {
            event.preventDefault();
            closeButton.focus();
        } else if (event.key === "ArrowRight") {
            event.preventDefault();
            showNext();
        } else if (event.key === "ArrowLeft") {
            event.preventDefault();
            showPrev();
        }
    });

    let touchStartX = 0;
    let touchStartY = 0;
    dialog.addEventListener("touchstart", (event) => {
        if (event.touches.length === 1) {
            touchStartX = event.touches[0].clientX;
            touchStartY = event.touches[0].clientY;
        }
    }, { passive: true });
    dialog.addEventListener("touchend", (event) => {
        if (event.changedTouches.length === 1) {
            const dx = event.changedTouches[0].clientX - touchStartX;
            const dy = event.changedTouches[0].clientY - touchStartY;
            if (Math.abs(dx) > 40 && Math.abs(dy) < 50) {
                if (dx < 0) showNext();
                else showPrev();
            } else if (dy > 80 && Math.abs(dx) < 60) {
                dialog.close();
            }
        }
    }, { passive: true });
});

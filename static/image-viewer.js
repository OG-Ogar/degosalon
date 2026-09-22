document.addEventListener("DOMContentLoaded", () => {
    const overlay = document.createElement("div");

    overlay.className = "image-viewer-overlay";

    overlay.innerHTML = `
        <button
            class="image-viewer-close"
            type="button"
            aria-label="Close image"
        >
            &times;
        </button>

        <img
            class="image-viewer-image"
            alt=""
        >
    `;

    document.body.appendChild(overlay);

    const viewerImage =
        overlay.querySelector(".image-viewer-image");

    const closeButton =
        overlay.querySelector(".image-viewer-close");

    function closeViewer() {
        overlay.classList.remove("open");
        viewerImage.removeAttribute("src");
        document.body.classList.remove("image-viewer-open");
    }

    function openViewer(image) {
        viewerImage.src = image.currentSrc || image.src;
        viewerImage.alt = image.alt || "Full-size image";

        overlay.classList.add("open");
        document.body.classList.add("image-viewer-open");
    }

    document.addEventListener("click", (event) => {
        const image =
            event.target.closest("[data-fullscreen-image]");

        if (!image) {
            return;
        }

        openViewer(image);
    });

    closeButton.addEventListener("click", closeViewer);

    overlay.addEventListener("click", (event) => {
        if (event.target === overlay) {
            closeViewer();
        }
    });

    document.addEventListener("keydown", (event) => {
        if (event.key === "Escape") {
            closeViewer();
        }
    });
});
document.addEventListener("DOMContentLoaded", () => {
    const dialog = document.createElement("dialog");
    dialog.className = "image-viewer-overlay";
    dialog.setAttribute("aria-label", "Full-size image");
    dialog.innerHTML = '<button class="image-viewer-close" type="button" aria-label="Close image">&times;</button><img class="image-viewer-image" alt="">';
    document.body.appendChild(dialog);
    const viewerImage = dialog.querySelector("img");
    const closeButton = dialog.querySelector("button");
    let opener = null;

    // Keep links and selection labels intact; enlargement is a separate action.
    document.querySelectorAll("img[data-fullscreen-image]").forEach((image) => {
        if (image.closest("a")) return;
        image.removeAttribute("data-fullscreen-image");
        const button = document.createElement("button");
        button.type = "button";
        button.className = "image-viewer-trigger";
        button.textContent = "Enlarge image";
        button.setAttribute("aria-label", "Enlarge " + (image.alt || "image"));
        button.setAttribute("aria-haspopup", "dialog");
        const label = image.closest("label");
        if (label) label.after(button);
        else image.after(button);
        button.addEventListener("click", () => {
            opener = button;
            viewerImage.src = image.src;
            viewerImage.alt = image.alt || "Full-size image";
            dialog.showModal();
            document.body.classList.add("image-viewer-open");
            closeButton.focus();
        });
    });

    closeButton.addEventListener("click", () => dialog.close());
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
        if (opener && opener.isConnected) opener.focus();
    });
    dialog.addEventListener("keydown", (event) => {
        if (event.key === "Tab") {
            event.preventDefault();
            closeButton.focus();
        }
    });
});

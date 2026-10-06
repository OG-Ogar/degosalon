document.addEventListener("DOMContentLoaded", () => {
    const dialog = document.createElement("dialog");
    if (typeof dialog.showModal !== "function") return;

    dialog.className = "image-viewer-overlay";
    dialog.setAttribute("aria-label", "Full-size photo");
    dialog.innerHTML = `
        <button class="image-viewer-close" type="button" aria-label="Close image" autofocus>&times;</button>
        <img class="image-viewer-image" alt="">
        <p class="image-viewer-status" role="status" hidden>This image could not be loaded.</p>
    `;
    document.body.appendChild(dialog);
    const image = dialog.querySelector(".image-viewer-image");
    const close = dialog.querySelector(".image-viewer-close");
    const status = dialog.querySelector(".image-viewer-status");
    let opener = null;

    image.addEventListener("error", () => { if (dialog.open) status.hidden = false; });
    image.addEventListener("load", () => { status.hidden = true; });
    close.addEventListener("click", () => dialog.close());
    dialog.addEventListener("click", (event) => {
        if (event.target === dialog) dialog.close();
    });
    // Native modal behavior supplies Escape, focus containment and inert background.
    // Clean up through close so every dismissal path restores the same state.
    dialog.addEventListener("close", () => {
        image.removeAttribute("src");
        image.alt = "";
        status.hidden = true;
        document.body.classList.remove("image-viewer-open");
        if (opener && opener.isConnected) opener.focus();
        opener = null;
    });
    document.querySelectorAll("button[data-image-viewer-src]").forEach((button) => {
        button.hidden = false;
        button.addEventListener("click", () => {
            const source = button.getAttribute("data-image-viewer-src");
            if (!source || dialog.open) return;
            opener = button;
            status.hidden = true;
            image.alt = button.getAttribute("data-image-viewer-alt") || "Full-size photo";
            image.src = source;
            dialog.showModal();
            document.body.classList.add("image-viewer-open");
        });
    });
});

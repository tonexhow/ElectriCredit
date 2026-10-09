/** ElectriCredit - Global Modal Manager */
const Modal = (() => {
    let activeModal = null;
    let activeOptions = null;

    function open(options = {}) {
        forceClose();

        const {
            title = "",
            content = "",
            confirmText = "",
            cancelText = "Close",
            onConfirm = null,
            closeOnBackdrop = true,
            hideCancel = false,
            beforeClose = null
        } = options;

        const overlay = document.createElement("div");
        overlay.className = "ec-modal-overlay";
        overlay.innerHTML = `
            <div class="ec-modal" role="dialog" aria-modal="true" aria-labelledby="ec-modal-title">
                <div class="ec-modal-header">
                    <h2 id="ec-modal-title">${escapeHTML(title)}</h2>
                    <button class="ec-modal-close" type="button" aria-label="Close">&times;</button>
                </div>
                <div class="ec-modal-body">${content}</div>
                <div class="ec-modal-footer" ${hideCancel && !confirmText ? "hidden" : ""}>
                    ${hideCancel ? "" : `<button class="ec-modal-cancel" type="button">${escapeHTML(cancelText)}</button>`}
                    ${confirmText ? `<button class="ec-modal-confirm" type="button">${escapeHTML(confirmText)}</button>` : ""}
                </div>
            </div>`;

        document.body.appendChild(overlay);
        activeModal = overlay;
        activeOptions = { beforeClose };

        overlay.querySelector(".ec-modal-close")?.addEventListener("click", () => requestClose("close"));
        overlay.querySelector(".ec-modal-cancel")?.addEventListener("click", () => requestClose("cancel"));

        const confirmButton = overlay.querySelector(".ec-modal-confirm");
        if (confirmButton && typeof onConfirm === "function") {
            confirmButton.addEventListener("click", async () => {
                if (confirmButton.disabled) return;
                confirmButton.disabled = true;
                try {
                    const result = await onConfirm();
                    if (result !== false && activeModal === overlay) {
                        await requestClose("confirm");
                    }
                } finally {
                    if (activeModal === overlay) confirmButton.disabled = false;
                }
            });
        }

        if (closeOnBackdrop) {
            overlay.addEventListener("click", event => {
                if (event.target === overlay) requestClose("backdrop");
            });
        }

        document.addEventListener("keydown", handleEscape);
        requestAnimationFrame(() => overlay.classList.add("is-active"));
        return overlay;
    }

    async function requestClose(reason = "close") {
        if (!activeModal) return;
        const beforeClose = activeOptions?.beforeClose;
        if (typeof beforeClose === "function") {
            try {
                const allowed = await beforeClose(reason, activeModal);
                if (allowed === false) return;
            } catch (_) {
                return;
            }
        }
        forceClose();
    }

    function close(options = {}) {
        if (options?.force) {
            forceClose();
            return;
        }
        return requestClose(options?.reason || "close");
    }

    function forceClose() {
        if (!activeModal) return;
        document.removeEventListener("keydown", handleEscape);
        activeModal.remove();
        activeModal = null;
        activeOptions = null;
    }

    function confirm(options = {}) {
        return new Promise(resolve => {
            const overlay = open({
                title: options.title || "Confirm",
                content: `<p>${escapeHTML(options.message || "Are you sure you want to continue?")}</p>`,
                confirmText: options.confirmText || "Confirm",
                cancelText: options.cancelText || "Cancel",
                onConfirm() {
                    forceClose();
                    resolve(true);
                }
            });

            const cancel = () => {
                forceClose();
                resolve(false);
            };
            overlay.querySelector(".ec-modal-cancel")?.addEventListener("click", cancel, { once: true });
            overlay.querySelector(".ec-modal-close")?.addEventListener("click", cancel, { once: true });
        });
    }

    function handleEscape(event) {
        if (event.key === "Escape") requestClose("escape");
    }

    function escapeHTML(value) {
        return String(value)
            .replaceAll("&", "&amp;")
            .replaceAll("<", "&lt;")
            .replaceAll(">", "&gt;")
            .replaceAll('"', "&quot;")
            .replaceAll("'", "&#039;");
    }

    return { open, close, confirm };
})();
window.Modal = Modal;

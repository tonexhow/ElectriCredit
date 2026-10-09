const IndexChatLogic = (() => {
    let mounted = false;
    async function mount(options = {}) {
        mounted = true;
        window.IndexChatDesign?.mount?.();
        const target = typeof options.target === "string" ? document.querySelector(options.target) : options.target;
        if (!target) return;
        target.innerHTML = window.IndexChatStructure.render();
        target.querySelector("[data-index-chat-back]")?.addEventListener("click", () => window.ElectriCredit?.setRoute?.("home"));
        target.querySelectorAll("[data-index-chat-call]").forEach(button => button.addEventListener("click", () => window.ChatUI?.startPublicCall?.(button.dataset.indexChatCall)));
        const root = target.querySelector("#index-chat-page-root");
        if (root && mounted) await window.ChatUI?.renderPublicPage?.(root);
    }
    function unmount() { mounted = false; window.IndexChatDesign?.unmount?.(); }
    return { mount, unmount };
})();
window.IndexChatLogic = IndexChatLogic;

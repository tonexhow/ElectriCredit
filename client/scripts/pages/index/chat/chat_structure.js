const IndexChatStructure = (() => {
    function icon(name) {
        const paths = {
            back: '<path d="m15 18-6-6 6-6"/><path d="M9 12h10"/>',
            phone: '<path d="M7 4 4 6c0 7.7 6.3 14 14 14l2-3-4-3-2 2c-3.1-1.2-5.8-3.9-7-7l2-2-2-3Z"/>',
            video: '<rect x="3" y="6" width="13" height="12" rx="2"/><path d="m16 10 5-3v10l-5-3"/>'
        };
        return `<svg class="ec-icon" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${paths[name] || paths.back}</svg>`;
    }

    function render() {
        return `
            <section class="ec-section ec-index-chat-page">
                <div class="ec-index-chat-pagebar">
                    <button class="ec-chat-page-action" type="button" data-index-chat-back aria-label="Back">${icon("back")}</button>
                    <strong>Chat</strong>
                    <div class="ec-index-chat-page-actions">
                        <button class="ec-chat-page-action" type="button" data-index-chat-call="audio" aria-label="Audio call" title="Audio call">${icon("phone")}</button>
                        <button class="ec-chat-page-action" type="button" data-index-chat-call="video" aria-label="Video call" title="Video call">${icon("video")}</button>
                    </div>
                </div>
                <div id="index-chat-page-root" class="ec-index-chat-page-root"></div>
            </section>`;
    }
    return { render };
})();
window.IndexChatStructure = IndexChatStructure;

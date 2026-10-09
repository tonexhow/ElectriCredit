/**
 * ElectriCredit - Public Users Design
 */

const TenantDesign = (() => {
    const STYLE_ID = "ec-index-users-style";

    function mount() {
        if (document.getElementById(STYLE_ID)) {
            return;
        }

        const style = document.createElement("style");
        style.id = STYLE_ID;

        style.textContent = `
            .ec-index-users {
                padding-top: clamp(1.25rem, 3vw, 2.5rem);
            }

            .ec-index-users-eyebrow {
                margin-bottom: var(--ec-space-3);
                background: var(--ec-primary-soft);
            }

            .ec-index-users-count {
                min-width: 120px;
                padding: var(--ec-space-3) var(--ec-space-4);
                border: 1px solid var(--ec-border);
                border-radius: var(--ec-radius-md);
                background: var(--ec-surface);
                box-shadow: var(--ec-shadow-sm);
            }

            .ec-index-users-count .ec-stat-value {
                display: block;
                margin-top: var(--ec-space-1);
                font-size: 1.5rem;
            }

            .ec-index-users-table-card {
                overflow: hidden;
            }

            .ec-index-users-table-wrap {
                width: 100%;
                overflow-x: auto;
            }

            .ec-index-users-table {
                width: 100%;
                min-width: 680px;
            }

            .ec-index-user-cell {
                display: flex;
                align-items: center;
                gap: var(--ec-space-3);
                min-width: 0;
            }

            .ec-index-user-avatar {
                width: 40px;
                height: 40px;
                flex: 0 0 auto;
                object-fit: cover;
                border: 1px solid var(--ec-border);
                border-radius: 50%;
                background: var(--ec-surface-soft);
            }

            .ec-index-user-avatar-lg {
                width: 54px;
                height: 54px;
            }

            .ec-index-user-primary {
                min-width: 0;
                display: flex;
                flex-direction: column;
                gap: 0.2rem;
            }

            .ec-index-user-primary span {
                color: var(--ec-text-muted);
                font-size: 0.8rem;
            }

            .ec-index-users-action-column {
                width: 130px;
                text-align: right;
            }

            .ec-index-users-empty {
                min-height: 220px;
                display: flex;
                align-items: center;
                justify-content: center;
                gap: var(--ec-space-4);
            }

            .ec-index-users-empty-mark {
                width: 54px;
                height: 54px;
                flex: 0 0 auto;
                display: flex;
                align-items: center;
                justify-content: center;
                border-radius: var(--ec-radius-md);
                background: var(--ec-primary-soft);
                font-size: 1.2rem;
                font-weight: 850;
            }

            .ec-index-modal-portal {
                position: fixed;
                inset: 0;
                z-index: 3000;
            }

            .ec-index-user-modal-backdrop {
                position: fixed;
                inset: 0;
                width: 100vw;
                height: 100dvh;
                z-index: 3000;
                display: grid;
                place-items: center;
                overflow: auto;
                padding: max(var(--ec-space-4), env(safe-area-inset-top)) max(var(--ec-space-4), env(safe-area-inset-right)) max(var(--ec-space-4), env(safe-area-inset-bottom)) max(var(--ec-space-4), env(safe-area-inset-left));
                background: rgba(0, 0, 0, 0.52);
                backdrop-filter: blur(4px);
                overscroll-behavior: contain;
            }

            .ec-index-user-modal {
                width: min(100%, 720px);
                max-height: calc(100dvh - max(2rem, env(safe-area-inset-top) + env(safe-area-inset-bottom)));
                min-height: 0;
                display: flex;
                flex-direction: column;
                overflow: hidden;
                margin: auto;
                box-shadow: var(--ec-shadow-lg);
            }

            .ec-index-user-modal-header {
                flex: 0 0 auto;
            }

            .ec-index-user-modal-identity {
                min-width: 0;
                display: flex;
                align-items: center;
                gap: var(--ec-space-3);
            }

            .ec-index-user-modal-label {
                display: block;
                margin-bottom: var(--ec-space-1);
                color: var(--ec-text-muted);
                font-size: 0.7rem;
                font-weight: 800;
                letter-spacing: 0.08em;
            }

            .ec-index-user-modal-close {
                width: 38px;
                height: 38px;
                flex: 0 0 auto;
                display: flex;
                align-items: center;
                justify-content: center;
                border: 1px solid var(--ec-border);
                border-radius: var(--ec-radius-sm);
                background: var(--ec-surface);
                color: var(--ec-text);
                cursor: pointer;
                font-size: 1.4rem;
                line-height: 1;
            }

            .ec-index-user-modal-close:hover {
                background: var(--ec-surface-soft);
            }

            .ec-index-user-modal-scroll {
                min-height: 0;
                flex: 1 1 auto;
                overflow-y: auto;
                overscroll-behavior: contain;
                scrollbar-gutter: stable;
            }

            .ec-index-user-modal-section + .ec-index-user-modal-section {
                margin-top: var(--ec-space-6);
                padding-top: var(--ec-space-5);
                border-top: 1px solid var(--ec-border);
            }

            .ec-index-user-modal-section h3 {
                margin: 0 0 var(--ec-space-4);
                font-size: 1rem;
            }

            .ec-index-user-details,
            .ec-index-rfid-details {
                display: grid;
                gap: var(--ec-space-3);
                margin: 0;
            }

            .ec-index-user-details > div,
            .ec-index-rfid-details > div {
                display: grid;
                grid-template-columns: minmax(100px, 0.35fr) minmax(0, 1fr);
                gap: var(--ec-space-3);
            }

            .ec-index-user-details dt,
            .ec-index-rfid-details dt {
                color: var(--ec-text-muted);
                font-size: 0.8rem;
                font-weight: 650;
            }

            .ec-index-user-details dd,
            .ec-index-rfid-details dd {
                min-width: 0;
                margin: 0;
                overflow-wrap: anywhere;
                color: var(--ec-text);
                font-size: 0.88rem;
                font-weight: 650;
            }

            .ec-index-user-card-grid {
                display: grid;
                grid-template-columns:
                    repeat(auto-fit, minmax(230px, 1fr));
                gap: var(--ec-space-3);
            }

            .ec-index-rfid-card {
                min-width: 0;
            }

            .ec-index-rfid-details {
                margin-top: var(--ec-space-4);
            }

            .ec-index-user-no-cards {
                color: var(--ec-text-muted);
                text-align: center;
            }

            body.ec-modal-open {
                overflow: hidden;
            }

            @media (max-width: 620px) {
                .ec-index-users-count {
                    width: 100%;
                }

                .ec-index-users-table-wrap {
                    overflow: visible;
                }

                .ec-index-users-table {
                    min-width: 0;
                    border-collapse: separate;
                    border-spacing: 0;
                }

                .ec-index-users-table thead {
                    display: none;
                }

                .ec-index-users-table,
                .ec-index-users-table tbody,
                .ec-index-users-table tr,
                .ec-index-users-table td {
                    display: block;
                    width: 100%;
                }

                .ec-index-users-table tbody {
                    display: grid;
                    gap: var(--ec-space-3);
                    padding: var(--ec-space-3);
                }

                .ec-index-users-table tr {
                    padding: var(--ec-space-3);
                    border: 1px solid var(--ec-border);
                    border-radius: var(--ec-radius-md);
                    background: var(--ec-surface);
                }

                .ec-index-users-table td {
                    display: grid;
                    grid-template-columns: minmax(84px, 0.32fr) minmax(0, 1fr);
                    align-items: center;
                    gap: var(--ec-space-3);
                    padding: var(--ec-space-2) 0;
                    border: 0;
                    text-align: left;
                }

                .ec-index-users-table td::before {
                    content: attr(data-label);
                    color: var(--ec-text-muted);
                    font-size: 0.72rem;
                    font-weight: 800;
                    letter-spacing: 0.04em;
                    text-transform: uppercase;
                }

                .ec-index-users-table td:first-child {
                    grid-template-columns: 1fr;
                    padding-top: 0;
                    padding-bottom: var(--ec-space-3);
                    border-bottom: 1px solid var(--ec-border);
                }

                .ec-index-users-table td:first-child::before {
                    display: none;
                }

                .ec-index-users-action-column {
                    width: 100%;
                }

                .ec-index-users-action-column .ec-button {
                    width: 100%;
                }

                .ec-index-users-empty {
                    flex-direction: column;
                    text-align: center;
                }

                .ec-index-user-modal-backdrop {
                    align-items: center;
                    padding: max(.55rem, env(safe-area-inset-top)) max(.55rem, env(safe-area-inset-right)) max(.55rem, env(safe-area-inset-bottom)) max(.55rem, env(safe-area-inset-left));
                }

                .ec-index-user-modal {
                    width: 100%;
                    max-height: calc(100dvh - 1.1rem - env(safe-area-inset-top) - env(safe-area-inset-bottom));
                }

                .ec-index-user-details > div,
                .ec-index-rfid-details > div {
                    grid-template-columns: 1fr;
                    gap: var(--ec-space-1);
                }
            }

            .ec-index-id-code {
                display: inline-flex;
                align-items: center;
                width: fit-content;
                padding: 0.28rem 0.48rem;
                border: 1px solid var(--ec-border);
                border-radius: 8px;
                background: var(--ec-surface-soft);
                color: var(--ec-text);
                font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
                font-size: 0.76rem;
                font-weight: 800;
                letter-spacing: 0.02em;
            }

            .ec-index-card-code-list {
                display: flex;
                flex-wrap: wrap;
                gap: 0.3rem;
                align-items: center;
            }

            .ec-index-card-code-list span {
                padding: 0.24rem 0.42rem;
                border-radius: 7px;
                background: var(--ec-primary-soft);
                font-size: 0.7rem;
                font-weight: 750;
            }

            .ec-index-card-code-list small {
                color: var(--ec-text-muted);
            }

            .ec-index-user-modal {
                position: relative;
                width: min(100%, 760px);
                border-radius: clamp(18px, 3vw, 26px);
            }

            .ec-index-user-modal-close {
                position: absolute;
                top: 12px;
                right: 12px;
                z-index: 10;
                border-radius: 50%;
                box-shadow: var(--ec-shadow-sm);
            }

            .ec-index-user-modal-hero {
                display: flex;
                align-items: center;
                gap: var(--ec-space-4);
                padding: clamp(1.2rem, 4vw, 2rem);
                padding-right: 4.5rem;
                border-bottom: 1px solid var(--ec-border);
                background: linear-gradient(135deg, var(--ec-primary-soft), var(--ec-surface));
            }

            .ec-index-user-modal-photo {
                width: clamp(72px, 15vw, 96px);
                height: clamp(72px, 15vw, 96px);
                flex: 0 0 auto;
                object-fit: cover;
                border: 3px solid var(--ec-surface);
                border-radius: 24px;
                box-shadow: var(--ec-shadow-md);
                background: var(--ec-surface-soft);
            }

            .ec-index-user-modal-title-wrap {
                min-width: 0;
            }

            .ec-index-user-modal-title-wrap h2 {
                margin: 0.25rem 0 0.55rem;
                font-size: clamp(1.2rem, 4vw, 1.65rem);
            }

            .ec-index-user-modal-meta,
            .ec-index-user-section-head {
                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: var(--ec-space-2);
                flex-wrap: wrap;
            }

            .ec-index-user-section-kicker {
                display: block;
                margin-bottom: 0.25rem;
                color: var(--ec-text-muted);
                font-size: 0.68rem;
                font-weight: 850;
                letter-spacing: 0.08em;
                text-transform: uppercase;
            }

            .ec-index-user-section-head h3 {
                margin: 0;
            }

            .ec-index-user-details-grid {
                grid-template-columns: repeat(2, minmax(0, 1fr));
            }

            .ec-index-rfid-card {
                border: 1px solid color-mix(in srgb, var(--ec-primary) 30%, var(--ec-border));
                background: linear-gradient(155deg, var(--ec-surface), var(--ec-primary-soft));
            }

            /* Keep the tenant dialog fully inside the viewport. Only its content body scrolls. */
            .ec-index-modal-portal {
                width: 100vw;
                height: 100dvh;
                overflow: hidden;
            }

            .ec-index-user-modal-backdrop {
                display: flex;
                align-items: center;
                justify-content: center;
                overflow: hidden;
                padding: max(12px, env(safe-area-inset-top)) max(12px, env(safe-area-inset-right)) max(12px, env(safe-area-inset-bottom)) max(12px, env(safe-area-inset-left));
            }

            .ec-index-user-modal {
                width: min(760px, calc(100vw - 24px));
                max-height: calc(100dvh - max(24px, env(safe-area-inset-top) + env(safe-area-inset-bottom)));
                margin: 0;
                overflow: hidden;
            }

            .ec-index-user-modal-hero {
                flex: 0 0 auto;
            }

            .ec-index-user-modal-scroll {
                flex: 1 1 auto;
                min-height: 0;
                overflow-y: auto;
                -webkit-overflow-scrolling: touch;
            }

            @media (max-width: 560px) {
                .ec-index-user-modal-hero {
                    align-items: flex-start;
                    padding-right: 3.7rem;
                }

                .ec-index-user-modal-photo {
                    width: 68px;
                    height: 68px;
                    border-radius: 18px;
                }

                .ec-index-user-details-grid {
                    grid-template-columns: 1fr;
                }
            }
        `;

        document.head.appendChild(style);
    }

    function unmount() {
        document.getElementById(STYLE_ID)?.remove();
    }

    return {
        mount,
        unmount
    };
})();

window.TenantDesign = TenantDesign;

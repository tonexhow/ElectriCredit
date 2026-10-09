/**
 * ElectriCredit - Public Hub Design
 * Shared layout/components remain in the global style.css.
 */

const HubDesign = (() => {
    const STYLE_ID = "ec-index-hub-style";

    function mount() {
        if (document.getElementById(STYLE_ID)) {
            return;
        }

        const style = document.createElement("style");
        style.id = STYLE_ID;

        style.textContent = `
            .ec-index-hub {
                padding-top: clamp(1.25rem, 3vw, 2.5rem);
            }

            .ec-index-hub-eyebrow {
                margin-bottom: var(--ec-space-3);
                background: var(--ec-primary-soft);
            }

            .ec-index-hub-count {
                min-width: 120px;
                padding: var(--ec-space-3) var(--ec-space-4);
                border: 1px solid var(--ec-border);
                border-radius: var(--ec-radius-md);
                background: var(--ec-surface);
                box-shadow: var(--ec-shadow-sm);
            }

            .ec-index-hub-count .ec-stat-value {
                display: block;
                margin-top: var(--ec-space-1);
                font-size: 1.5rem;
            }

            .ec-index-hub-grid {
                display: grid;
                grid-template-columns:
                    repeat(auto-fit, minmax(280px, 1fr));
                gap: var(--ec-space-4);
            }

            .ec-index-hub-card {
                min-width: 0;
                transition:
                    transform var(--ec-transition),
                    border-color var(--ec-transition),
                    box-shadow var(--ec-transition);
            }

            .ec-index-hub-card:hover {
                transform: translateY(-2px);
                border-color: var(--ec-border-strong);
                box-shadow: var(--ec-shadow-md);
            }

            .ec-index-hub-heading {
                min-width: 0;
                display: flex;
                align-items: center;
                gap: var(--ec-space-3);
            }

            .ec-index-hub-icon {
                width: 42px;
                height: 42px;
                flex: 0 0 auto;
                display: grid;
                place-items: center;
                border: 1px solid var(--ec-border);
                border-radius: 14px;
                background: var(--ec-primary-soft);
                color: var(--ec-accent-strong);
            }

            .ec-index-hub-icon .ec-icon,
            .ec-index-hub-empty-mark .ec-icon {
                width: 22px;
                height: 22px;
            }

            .ec-index-hub-type {
                display: block;
                margin-bottom: var(--ec-space-1);
                color: var(--ec-text-muted);
                font-size: 0.72rem;
                font-weight: 800;
                text-transform: uppercase;
                letter-spacing: 0.07em;
            }

            .ec-index-hub-heading .ec-card-title {
                overflow: hidden;
                text-overflow: ellipsis;
                white-space: nowrap;
            }

            .ec-index-hub-details {
                display: grid;
                gap: var(--ec-space-3);
                margin: 0;
            }

            .ec-index-hub-details > div {
                display: grid;
                grid-template-columns: minmax(90px, 0.35fr) minmax(0, 1fr);
                gap: var(--ec-space-3);
                padding-bottom: var(--ec-space-3);
                border-bottom: 1px solid var(--ec-border);
            }

            .ec-index-hub-details > div:last-child {
                padding-bottom: 0;
                border-bottom: 0;
            }

            .ec-index-hub-details dt {
                color: var(--ec-text-muted);
                font-size: 0.82rem;
                font-weight: 650;
            }

            .ec-index-hub-details dd {
                min-width: 0;
                margin: 0;
                overflow-wrap: anywhere;
                color: var(--ec-text);
                font-size: 0.9rem;
                font-weight: 650;
            }

            .ec-index-hub-details .ec-index-hub-session {
                padding: .8rem .9rem;
                border: 1px solid color-mix(in srgb, var(--ec-warning) 30%, var(--ec-border));
                border-radius: 12px;
                background: color-mix(in srgb, var(--ec-warning) 8%, var(--ec-surface));
            }

            .ec-index-hub-session dt {
                display: inline-flex;
                align-items: center;
                gap: .35rem;
            }

            .ec-index-hub-session .ec-icon {
                width: 16px;
                height: 16px;
            }

            [data-hub-status="idle"] {
                background: var(--ec-success);
            }

            [data-hub-status="in-use"] {
                background: var(--ec-warning);
            }

            [data-hub-status="disabled"] {
                background: var(--ec-danger);
            }

            .ec-index-hub-empty {
                min-height: 220px;
                display: flex;
                align-items: center;
                justify-content: center;
                gap: var(--ec-space-4);
            }

            .ec-index-hub-empty-mark {
                width: 54px;
                height: 54px;
                flex: 0 0 auto;
                display: grid;
                place-items: center;
                border-radius: var(--ec-radius-md);
                background: var(--ec-primary-soft);
                font-size: 1.2rem;
                font-weight: 850;
            }

            @media (max-width: 620px) {
                .ec-index-hub-count {
                    width: 100%;
                }

                .ec-index-hub-grid {
                    grid-template-columns: 1fr;
                }

                .ec-index-hub-details > div {
                    grid-template-columns: 1fr;
                    gap: var(--ec-space-1);
                }

                .ec-index-hub-empty {
                    flex-direction: column;
                    text-align: center;
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

window.HubDesign = HubDesign;

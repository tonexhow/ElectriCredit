/**
 * ElectriCredit - Public Registry Design
 * Reuses the global ElectriCredit design system.
 */

const RegistryDesign = (() => {
    const STYLE_ID = "ec-index-registry-style";

    function mount() {
        if (document.getElementById(STYLE_ID)) {
            return;
        }

        const style = document.createElement("style");
        style.id = STYLE_ID;

        style.textContent = `
            .ec-index-registry {
                padding-top: clamp(1.25rem, 3vw, 2.5rem);
            }

            .ec-index-registry-eyebrow {
                margin-bottom: var(--ec-space-3);
                background: var(--ec-primary-soft);
            }

            .ec-index-registry-count {
                min-width: 120px;
                padding: var(--ec-space-3) var(--ec-space-4);
                border: 1px solid var(--ec-border);
                border-radius: var(--ec-radius-md);
                background: var(--ec-surface);
                box-shadow: var(--ec-shadow-sm);
            }

            .ec-index-registry-count .ec-stat-value {
                display: block;
                margin-top: var(--ec-space-1);
                font-size: 1.5rem;
            }

            .ec-index-module-grid {
                display: grid;
                grid-template-columns:
                    repeat(auto-fit, minmax(280px, 1fr));
                gap: var(--ec-space-4);
            }

            .ec-index-module-card {
                min-width: 0;
                transition:
                    transform var(--ec-transition),
                    border-color var(--ec-transition),
                    box-shadow var(--ec-transition);
            }

            .ec-index-module-card:hover {
                transform: translateY(-2px);
                border-color: var(--ec-border-strong);
                box-shadow: var(--ec-shadow-md);
            }

            .ec-index-module-heading {
                min-width: 0;
                display: flex;
                align-items: center;
                gap: var(--ec-space-3);
            }

            .ec-index-module-icon {
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

            .ec-index-module-icon .ec-icon,
            .ec-index-registry-empty-mark .ec-icon {
                width: 22px;
                height: 22px;
            }

            .ec-index-module-type {
                display: block;
                margin-bottom: var(--ec-space-1);
                color: var(--ec-text-muted);
                font-size: 0.72rem;
                font-weight: 800;
                text-transform: uppercase;
                letter-spacing: 0.07em;
            }

            .ec-index-module-heading .ec-card-title {
                overflow: hidden;
                text-overflow: ellipsis;
                white-space: nowrap;
            }

            .ec-index-module-details {
                display: grid;
                gap: var(--ec-space-3);
                margin: 0;
            }

            .ec-index-module-details > div {
                display: grid;
                grid-template-columns: minmax(90px, 0.35fr) minmax(0, 1fr);
                gap: var(--ec-space-3);
                padding-bottom: var(--ec-space-3);
                border-bottom: 1px solid var(--ec-border);
            }

            .ec-index-module-details > div:last-child {
                padding-bottom: 0;
                border-bottom: 0;
            }

            .ec-index-module-details dt {
                color: var(--ec-text-muted);
                font-size: 0.82rem;
                font-weight: 650;
            }

            .ec-index-module-details dd {
                min-width: 0;
                margin: 0;
                overflow-wrap: anywhere;
                color: var(--ec-text);
                font-size: 0.9rem;
                font-weight: 650;
            }

            .ec-index-module-details .ec-index-module-session {
                padding: .8rem .9rem;
                border: 1px solid color-mix(in srgb, var(--ec-warning) 30%, var(--ec-border));
                border-radius: 12px;
                background: color-mix(in srgb, var(--ec-warning) 8%, var(--ec-surface));
            }

            .ec-index-module-session dt {
                display: inline-flex;
                align-items: center;
                gap: .35rem;
            }

            .ec-index-module-session .ec-icon {
                width: 16px;
                height: 16px;
            }

            [data-module-status="idle"] {
                background: var(--ec-success);
            }

            [data-module-status="in-use"] {
                background: var(--ec-warning);
            }

            [data-module-status="disabled"] {
                background: var(--ec-danger);
            }

            .ec-index-registry-empty {
                min-height: 220px;
                display: flex;
                align-items: center;
                justify-content: center;
                gap: var(--ec-space-4);
                text-align: left;
            }

            .ec-index-registry-empty-mark {
                width: 54px;
                height: 54px;
                flex: 0 0 auto;
                display: grid;
                place-items: center;
                border-radius: var(--ec-radius-md);
                background: var(--ec-primary-soft);
                font-weight: 850;
                font-size: 1.2rem;
            }

            @media (max-width: 620px) {
                .ec-index-registry-count {
                    width: 100%;
                }

                .ec-index-module-grid {
                    grid-template-columns: 1fr;
                }

                .ec-index-module-details > div {
                    grid-template-columns: 1fr;
                    gap: var(--ec-space-1);
                }

                .ec-index-registry-empty {
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

window.RegistryDesign = RegistryDesign;

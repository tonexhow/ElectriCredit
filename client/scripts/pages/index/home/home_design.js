/**
 * ElectriCredit - Public Home Design
 * Only Home-specific rules live here.
 * Shared sections/cards/buttons/layout remain in style.css.
 */

const HomeDesign = (() => {
    const STYLE_ID = "ec-index-home-style";

    function mount() {
        if (document.getElementById(STYLE_ID)) {
            return;
        }

        const style = document.createElement("style");
        style.id = STYLE_ID;

        style.textContent = `
            .ec-index-home {
                padding-top: clamp(1.25rem, 3.5vw, 3rem);
                padding-bottom: clamp(2.5rem, 7vw, 5rem);
            }

            .ec-index-home-hero {
                display: grid;
                grid-template-columns:
                    minmax(0, 1.35fr)
                    minmax(260px, 0.65fr);
                align-items: center;
                gap: clamp(2rem, 7vw, 5rem);
            }

            .ec-index-home-copy {
                max-width: 780px;
            }

            .ec-index-home-greeting {
                margin-bottom: var(--ec-space-4);
                background: var(--ec-primary-soft);
            }

            .ec-index-home-title {
                margin-bottom: var(--ec-space-4);
                font-size: clamp(2.5rem, 7vw, 5.2rem);
                line-height: 1;
                letter-spacing: -0.06em;
                font-weight: 820;
            }

            .ec-index-home-lead {
                max-width: 680px;
                margin-bottom: 0;
                color: var(--ec-text-muted);
                font-size: clamp(1rem, 2vw, 1.16rem);
            }

            .ec-index-price-card {
                position: relative;
                overflow: hidden;
                box-shadow: var(--ec-shadow-md);
            }

            .ec-index-price-card::before {
                content: "";
                position: absolute;
                inset: 0 0 auto;
                height: 4px;
                background: var(--ec-primary);
            }

            .ec-index-price-card .ec-card-body {
                display: flex;
                flex-direction: column;
                gap: var(--ec-space-2);
                padding: clamp(1.25rem, 4vw, 2rem);
            }

            .ec-index-price {
                color: var(--ec-text);
                font-size: clamp(1.75rem, 4vw, 2.7rem);
                line-height: 1.1;
                letter-spacing: -0.04em;
            }

            .ec-index-summary-card {
                height: 100%;
                display: flex;
                flex-direction: column;
                transition:
                    transform var(--ec-transition),
                    box-shadow var(--ec-transition),
                    border-color var(--ec-transition);
            }

            .ec-index-summary-card:hover {
                transform: translateY(-2px);
                border-color: var(--ec-border-strong);
                box-shadow: var(--ec-shadow-md);
            }

            .ec-index-summary-card .ec-card-body {
                flex: 1;
            }

            .ec-index-home-info {
                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: var(--ec-space-6);
                padding: clamp(1.25rem, 4vw, 2rem);
                background:
                    linear-gradient(
                        135deg,
                        var(--ec-primary-soft),
                        var(--ec-accent-soft)
                    );
            }

            .ec-index-home-info-label {
                display: inline-block;
                margin-bottom: var(--ec-space-2);
                color: var(--ec-accent);
                font-size: 0.76rem;
                font-weight: 800;
                letter-spacing: 0.09em;
            }

            @media (max-width: 820px) {
                .ec-index-home-hero {
                    grid-template-columns: 1fr;
                }

                .ec-index-price-card {
                    max-width: 520px;
                }
            }

            @media (max-width: 620px) {
                .ec-index-home-title {
                    font-size: clamp(2.3rem, 13vw, 3.7rem);
                }

                .ec-index-home-info {
                    align-items: stretch;
                    flex-direction: column;
                }

                .ec-index-home-info .ec-button {
                    width: 100%;
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

window.HomeDesign = HomeDesign;

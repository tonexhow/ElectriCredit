/**
 * ElectriCredit - Public About Design
 * Mobile-first About page rules only.
 */

const AboutDesign = (() => {
    const STYLE_ID = "ec-index-about-style";

    function mount() {
        if (document.getElementById(STYLE_ID)) {
            return;
        }

        const style = document.createElement("style");
        style.id = STYLE_ID;

        style.textContent = `
            .ec-index-about {
                padding-top: clamp(1.25rem, 4vw, 2.5rem);
                padding-bottom: clamp(1.75rem, 5vw, 3.5rem);
            }

            .ec-index-about-header {
                display: grid;
                grid-template-columns: 1fr;
                align-items: stretch;
                gap: var(--ec-space-5);
            }

            .ec-index-about-copy {
                max-width: 780px;
                text-align: left;
            }

            .ec-index-about-eyebrow {
                margin-bottom: var(--ec-space-4);
                background: var(--ec-primary-soft);
            }

            .ec-index-about-title {
                max-width: 760px;
                font-size: clamp(1.8rem, 7vw, 3.4rem);
                line-height: 1.06;
                letter-spacing: -0.045em;
            }

            .ec-index-about-lead {
                max-width: 720px;
                margin-top: var(--ec-space-3);
                font-size: clamp(0.96rem, 3vw, 1.08rem);
            }

            .ec-index-about-purpose {
                display: flex;
                flex-direction: column;
                justify-content: center;
                gap: var(--ec-space-2);
                min-width: 0;
                padding: var(--ec-space-5);
                background:
                    linear-gradient(
                        135deg,
                        var(--ec-primary-soft),
                        var(--ec-surface)
                    );
            }

            .ec-index-about-purpose strong {
                color: var(--ec-text);
                font-size: 1.08rem;
            }

            .ec-index-about-purpose p {
                margin: 0;
                color: var(--ec-text-muted);
                font-size: 0.9rem;
            }

            .ec-index-about-card {
                height: 100%;
                transition:
                    transform var(--ec-transition),
                    border-color var(--ec-transition),
                    box-shadow var(--ec-transition);
            }

            .ec-index-about-card:hover {
                transform: translateY(-2px);
                border-color: var(--ec-border-strong);
                box-shadow: var(--ec-shadow-md);
            }

            .ec-index-about-flow {
                display: grid;
                grid-template-columns: 1fr;
                gap: var(--ec-space-3);
            }

            .ec-index-about-flow-item {
                display: grid;
                grid-template-columns: auto minmax(0, 1fr);
                align-items: start;
                gap: var(--ec-space-3);
                padding: var(--ec-space-4);
            }

            .ec-index-about-flow-number {
                width: 40px;
                height: 40px;
                display: grid;
                place-items: center;
                border-radius: var(--ec-radius-sm);
                background: var(--ec-primary-soft);
                font-size: 0.78rem;
                font-weight: 850;
            }

            .ec-index-about-flow-item h3 {
                margin: 0 0 var(--ec-space-1);
                font-size: 0.98rem;
            }

            .ec-index-about-flow-item p {
                margin: 0;
                color: var(--ec-text-muted);
                font-size: 0.9rem;
            }

            .ec-index-about-team-grid {
                display: grid;
                grid-template-columns: 1fr;
                gap: var(--ec-space-4);
                align-items: start;
            }

            .ec-index-about-team-card {
                min-width: 0;
            }

            .ec-index-about-owner-body {
                display: flex;
                align-items: center;
                gap: var(--ec-space-4);
            }

            .ec-index-about-avatar {
                width: 64px;
                height: 64px;
                flex: 0 0 auto;
                object-fit: cover;
                border: 1px solid var(--ec-border);
                border-radius: 50%;
                background: var(--ec-surface-soft);
            }

            .ec-index-about-avatar-sm {
                width: 42px;
                height: 42px;
            }

            .ec-index-about-member-copy {
                min-width: 0;
                display: flex;
                flex: 1;
                align-items: flex-start;
                flex-direction: column;
                gap: var(--ec-space-1);
            }

            .ec-index-about-owner-card {
                position: relative;
                overflow: hidden;
            }

            .ec-index-about-owner-card::before {
                content: "";
                position: absolute;
                inset: 0 0 auto;
                height: 4px;
                background: var(--ec-primary);
            }

            .ec-index-about-team-label {
                display: block;
                margin-bottom: var(--ec-space-2);
                color: var(--ec-text-muted);
                font-size: 0.72rem;
                font-weight: 800;
                text-transform: uppercase;
                letter-spacing: 0.08em;
            }

            .ec-index-about-member-list {
                display: flex;
                flex-direction: column;
                gap: var(--ec-space-3);
            }

            .ec-index-about-member {
                display: flex;
                align-items: center;
                gap: var(--ec-space-3);
                padding-bottom: var(--ec-space-3);
                border-bottom: 1px solid var(--ec-border);
            }

            .ec-index-about-member:last-child {
                padding-bottom: 0;
                border-bottom: 0;
            }

            .ec-index-about-member span {
                color: var(--ec-text-muted);
                font-size: 0.82rem;
                text-align: left;
            }

            .ec-index-about-final {
                display: flex;
                align-items: stretch;
                flex-direction: column;
                gap: var(--ec-space-5);
                padding: clamp(1.25rem, 4vw, 2rem);
                background:
                    linear-gradient(
                        135deg,
                        var(--ec-primary-soft),
                        var(--ec-accent-soft)
                    );
            }

            .ec-index-about-final-label {
                display: inline-block;
                margin-bottom: var(--ec-space-2);
                color: var(--ec-accent);
                font-size: 0.75rem;
                font-weight: 800;
                letter-spacing: 0.09em;
            }

            .ec-index-about-final .ec-button {
                width: 100%;
            }

            @media (min-width: 681px) {
                .ec-index-about-flow {
                    grid-template-columns: repeat(2, minmax(0, 1fr));
                }

                .ec-index-about-team-grid {
                    grid-template-columns: repeat(2, minmax(0, 1fr));
                }

                .ec-index-about-owner-card {
                    grid-column: 1 / -1;
                }

                .ec-index-about-member-copy {
                    align-items: center;
                    flex-direction: row;
                    justify-content: space-between;
                    gap: var(--ec-space-3);
                }

                .ec-index-about-member span {
                    text-align: right;
                }

                .ec-index-about-final {
                    align-items: center;
                    justify-content: space-between;
                    flex-direction: row;
                    gap: var(--ec-space-6);
                }

                .ec-index-about-final .ec-button {
                    width: auto;
                }
            }

            @media (min-width: 961px) {
                .ec-index-about-header {
                    grid-template-columns: minmax(0, 1.45fr) minmax(260px, 0.55fr);
                    align-items: center;
                    gap: clamp(2rem, 6vw, 5rem);
                }

                .ec-index-about-purpose {
                    min-height: 180px;
                }

                .ec-index-about-flow {
                    grid-template-columns: repeat(5, minmax(0, 1fr));
                }

                .ec-index-about-flow-item {
                    grid-template-columns: 1fr;
                }

                .ec-index-about-team-grid {
                    grid-template-columns:
                        minmax(220px, 0.8fr)
                        repeat(2, minmax(260px, 1fr));
                }

                .ec-index-about-owner-card {
                    grid-column: auto;
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

window.AboutDesign = AboutDesign;

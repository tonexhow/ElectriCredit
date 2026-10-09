/**
 * ElectriCredit - Public Login and Account Recovery Design
 */

const LoginDesign = (() => {
    const STYLE_ID = "ec-index-login-style";

    function mount() {
        if (document.getElementById(STYLE_ID)) {
            return;
        }

        const style = document.createElement("style");
        style.id = STYLE_ID;

        style.textContent = `
            .ec-index-login-backdrop {
                position: fixed;
                inset: 0;
                z-index: 1100;
                display: grid;
                place-items: center;
                padding: var(--ec-space-3);
                background: rgba(0, 0, 0, 0.54);
                backdrop-filter: blur(5px);
            }

            .ec-index-login-card {
                position: relative;
                width: min(100%, 470px);
                max-height: 92vh;
                display: flex;
                flex-direction: column;
                overflow: hidden;
                box-shadow: var(--ec-shadow-lg);
            }

            .ec-index-login-header {
                align-items: flex-start;
                flex: 0 0 auto;
                padding-right: 4.25rem;
            }

            .ec-index-login-scroll {
                overflow-y: auto;
                overscroll-behavior: contain;
            }

            .ec-index-login-label {
                display: block;
                margin-bottom: var(--ec-space-1);
                color: var(--ec-text-muted);
                font-size: 0.72rem;
                font-weight: 800;
                letter-spacing: 0.08em;
            }

            .ec-index-login-close {
                position: absolute;
                top: 0.85rem;
                right: 0.85rem;
                z-index: 2;
                width: 38px;
                height: 38px;
                flex: 0 0 auto;
                display: grid;
                place-items: center;
                border: 1px solid var(--ec-border);
                border-radius: var(--ec-radius-sm);
                background: var(--ec-surface);
                color: var(--ec-text);
                cursor: pointer;
                font-size: 1.4rem;
                line-height: 1;
            }

            .ec-index-login-close:hover {
                background: var(--ec-surface-soft);
            }

            .ec-index-password-field {
                position: relative;
            }

            .ec-index-password-field .ec-input {
                padding-right: 3.25rem;
            }

            .ec-index-password-eye {
                position: absolute;
                top: 50%;
                right: 0.65rem;
                width: 36px;
                height: 36px;
                display: grid;
                place-items: center;
                transform: translateY(-50%);
                border: 0;
                border-radius: 50%;
                background: transparent;
                color: var(--ec-text-muted);
                cursor: pointer;
            }

            .ec-index-password-eye:hover {
                background: var(--ec-surface-soft);
                color: var(--ec-text);
            }

            .ec-index-password-eye svg {
                width: 19px;
                height: 19px;
                fill: currentColor;
            }

            .ec-index-password-eye .ec-eye-hidden {
                display: none;
            }

            .ec-index-password-eye[aria-pressed="true"] .ec-eye-visible {
                display: none;
            }

            .ec-index-password-eye[aria-pressed="true"] .ec-eye-hidden {
                display: inline-flex;
            }

            .ec-index-login-help-row {
                display: flex;
                justify-content: flex-end;
                margin-top: calc(var(--ec-space-2) * -1);
            }

            .ec-index-login-recovery-link,
            .ec-index-login-footer-action {
                border: 0;
                padding: 0;
                background: transparent;
                color: var(--ec-primary-strong);
                cursor: pointer;
                font: inherit;
                font-weight: 750;
                text-decoration: none;
            }

            .ec-index-login-recovery-link:hover,
            .ec-index-login-footer-action:hover {
                text-decoration: underline;
                text-underline-offset: 3px;
            }

            .ec-index-login-error {
                margin: 0;
                padding: var(--ec-space-3);
                border: 1px solid color-mix(
                    in srgb,
                    var(--ec-danger) 35%,
                    var(--ec-border)
                );
                border-radius: var(--ec-radius-sm);
                background: color-mix(
                    in srgb,
                    var(--ec-danger) 10%,
                    transparent
                );
                color: var(--ec-danger);
                font-size: 0.88rem;
            }

            .ec-index-login-footer {
                flex: 0 0 auto;
                color: var(--ec-text-muted);
                font-size: 0.8rem;
                text-align: center;
            }

            .ec-index-recovery-status {
                display: grid;
                justify-items: center;
                gap: var(--ec-space-3);
                padding: var(--ec-space-5) var(--ec-space-3);
                text-align: center;
                color: var(--ec-text-muted);
            }

            .ec-index-recovery-status strong {
                color: var(--ec-text);
                font-size: 1rem;
            }

            .ec-index-recovery-spinner {
                width: 42px;
                height: 42px;
                border: 3px solid var(--ec-border);
                border-top-color: var(--ec-primary-strong);
                border-radius: 50%;
                animation: ecRecoverySpin 0.8s linear infinite;
            }

            .ec-index-recovery-status-icon {
                width: 42px;
                height: 42px;
                display: grid;
                place-items: center;
                border-radius: 50%;
                background: color-mix(in srgb, var(--ec-warning) 18%, transparent);
                color: var(--ec-warning);
                font-size: 1.2rem;
                font-weight: 900;
            }

            .ec-index-recovery-note,
            .ec-index-recovery-hint {
                margin: 0;
                color: var(--ec-text-muted);
                font-size: 0.84rem;
                line-height: 1.55;
            }

            .ec-index-recovery-note {
                padding: var(--ec-space-3);
                border: 1px solid var(--ec-border);
                border-radius: var(--ec-radius-sm);
                background: var(--ec-surface-soft);
            }

            .ec-index-otp-grid {
                display: grid;
                grid-template-columns: repeat(6, minmax(0, 1fr));
                gap: 0.5rem;
                width: 100%;
            }

            .ec-index-otp-input {
                width: 100%;
                min-width: 0;
                height: 54px;
                border: 1px solid var(--ec-border);
                border-radius: var(--ec-radius-sm);
                background: var(--ec-surface);
                color: var(--ec-text);
                outline: none;
                text-align: center;
                font-size: 1.25rem;
                font-weight: 800;
                caret-color: var(--ec-primary-strong);
            }

            .ec-index-otp-input:focus {
                border-color: var(--ec-primary-strong);
                box-shadow: 0 0 0 3px var(--ec-primary-soft);
            }

            body.ec-login-open {
                overflow: hidden;
            }

            @keyframes ecRecoverySpin {
                to { transform: rotate(360deg); }
            }

            @media (max-width: 480px) {
                .ec-index-login-backdrop {
                    align-items: end;
                    padding: 0;
                }

                .ec-index-login-card {
                    width: 100%;
                    max-height: 94vh;
                    border-bottom-left-radius: 0;
                    border-bottom-right-radius: 0;
                }

                .ec-index-otp-grid {
                    gap: 0.38rem;
                }

                .ec-index-otp-input {
                    height: 50px;
                    font-size: 1.1rem;
                }
            }

            @media (prefers-reduced-motion: reduce) {
                .ec-index-recovery-spinner {
                    animation: none;
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

window.LoginDesign = LoginDesign;

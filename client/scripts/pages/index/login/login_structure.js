/**
 * ElectriCredit - Public Login and Account Recovery Structure
 */

const LoginStructure = (() => {
    function modalShell({
        id,
        label,
        title,
        description,
        body,
        footer = ""
    }) {
        return `
            <div
                class="ec-index-login-backdrop"
                id="${id}"
                role="dialog"
                aria-modal="true"
                aria-labelledby="${id}-title"
            >
                <div class="ec-card ec-index-login-card">
                    <div class="ec-card-header ec-index-login-header">
                        <div>
                            <span class="ec-index-login-label">
                                ${escapeHTML(label)}
                            </span>

                            <h2
                                class="ec-card-title"
                                id="${id}-title"
                            >
                                ${escapeHTML(title)}
                            </h2>

                            <p class="ec-card-description">
                                ${escapeHTML(description)}
                            </p>
                        </div>

                        <button
                            class="ec-index-login-close"
                            type="button"
                            data-login-close
                            aria-label="Close"
                        >
                            ×
                        </button>
                    </div>

                    <div class="ec-card-body ec-index-login-scroll">
                        ${body}
                    </div>

                    ${footer ? `
                        <div class="ec-card-footer ec-index-login-footer">
                            ${footer}
                        </div>
                    ` : ""}
                </div>
            </div>
        `;
    }

    function passwordControl({ id, name, label, autocomplete, placeholder }) {
        return `
            <div class="ec-form-group">
                <label class="ec-label" for="${id}">
                    ${escapeHTML(label)}
                </label>

                <div class="ec-index-password-field">
                    <input
                        class="ec-input"
                        id="${id}"
                        name="${name}"
                        type="password"
                        autocomplete="${autocomplete}"
                        placeholder="${escapeHTML(placeholder)}"
                        minlength="8"
                        maxlength="128"
                        required
                    >

                    <button
                        class="ec-index-password-eye"
                        type="button"
                        data-password-toggle
                        data-password-target="${id}"
                        aria-label="Show ${escapeHTML(label.toLowerCase())}"
                        aria-pressed="false"
                    >
                        <span class="ec-eye-visible" aria-hidden="true">${eyeIcon()}</span>
                        <span class="ec-eye-hidden" aria-hidden="true">${eyeOffIcon()}</span>
                    </button>
                </div>
            </div>
        `;
    }

    function render(options = {}) {
        const email = escapeHTML(options.email || "");

        return modalShell({
            id: "index-login-modal",
            label: "AUTHORIZED ACCESS",
            title: "Sign in to ElectriCredit",
            description: "For Administrator, Owner, and Developer accounts.",
            body: `
                <form class="ec-stack" id="index-login-form" novalidate>
                    <div class="ec-form-group">
                        <label class="ec-label" for="index-login-email">
                            Email
                        </label>

                        <input
                            class="ec-input"
                            id="index-login-email"
                            name="email"
                            type="email"
                            autocomplete="username"
                            inputmode="email"
                            placeholder="name@example.com"
                            value="${email}"
                            required
                        >
                    </div>

                    ${passwordControl({
                        id: "index-login-password",
                        name: "password",
                        label: "Password",
                        autocomplete: "current-password",
                        placeholder: "Enter your password"
                    })}

                    <div class="ec-index-login-help-row">
                        <button
                            class="ec-index-login-recovery-link"
                            type="button"
                            data-recover-account
                        >
                            Recover account
                        </button>
                    </div>

                    <p
                        class="ec-index-login-error ec-hidden"
                        id="index-login-error"
                        role="alert"
                    ></p>

                    <button
                        class="ec-button ec-button-primary ec-w-full"
                        id="index-login-submit"
                        type="submit"
                    >
                        Sign In
                    </button>
                </form>
            `,
            footer: `
                <span>
                    Access is limited to authorized ElectriCredit users.
                </span>
            `
        });
    }

    function renderRecoveryCheck() {
        return modalShell({
            id: "index-recovery-check-modal",
            label: "ACCOUNT RECOVERY",
            title: "Checking connection",
            description: "Email recovery requires an internet connection.",
            body: `
                <div class="ec-index-recovery-status" role="status" aria-live="polite">
                    <span class="ec-index-recovery-spinner" aria-hidden="true"></span>
                    <strong>Checking internet and email availability…</strong>
                    <span>This only checks whether recovery email can be used right now.</span>
                </div>
            `,
            footer: backToLoginButton()
        });
    }

    function renderRecoveryUnavailable(message) {
        return modalShell({
            id: "index-recovery-unavailable-modal",
            label: "ACCOUNT RECOVERY",
            title: "Recovery is offline",
            description: "ElectriCredit could not start email recovery.",
            body: `
                <div class="ec-index-recovery-status is-warning" role="alert">
                    <span class="ec-index-recovery-status-icon" aria-hidden="true">!</span>
                    <strong>Email recovery is unavailable right now.</strong>
                    <span>${escapeHTML(message || "Check the internet connection and try again.")}</span>
                </div>

                <button
                    class="ec-button ec-button-primary ec-w-full"
                    type="button"
                    data-recovery-retry
                >
                    Check Again
                </button>
            `,
            footer: backToLoginButton()
        });
    }

    function renderRecoveryEmail(options = {}) {
        const message = options.message
            ? `<p class="ec-index-recovery-note">${escapeHTML(options.message)}</p>`
            : "";

        return modalShell({
            id: "index-recovery-email-modal",
            label: "ACCOUNT RECOVERY",
            title: "Recover your account",
            description: "Enter the email connected to your authorized ElectriCredit account.",
            body: `
                <form class="ec-stack" id="index-recovery-email-form" novalidate>
                    ${message}

                    <div class="ec-form-group">
                        <label class="ec-label" for="index-recovery-email">
                            Account email
                        </label>

                        <input
                            class="ec-input"
                            id="index-recovery-email"
                            name="email"
                            type="email"
                            autocomplete="email"
                            inputmode="email"
                            placeholder="name@example.com"
                            value="${escapeHTML(options.email || "")}"
                            required
                        >
                    </div>

                    <p class="ec-index-recovery-hint">
                        One OTP can be active at a time. Each OTP expires after 15 minutes,
                        and an account can receive at most 5 recovery OTPs within 2 hours.
                    </p>

                    <p
                        class="ec-index-login-error ec-hidden"
                        id="index-recovery-error"
                        role="alert"
                    ></p>

                    <button
                        class="ec-button ec-button-primary ec-w-full"
                        id="index-recovery-send"
                        type="submit"
                    >
                        Send OTP
                    </button>
                </form>
            `,
            footer: backToLoginButton()
        });
    }

    function renderRecoveryOTP(options = {}) {
        return modalShell({
            id: "index-recovery-otp-modal",
            label: "VERIFY EMAIL",
            title: "Enter your 6-digit OTP",
            description: `We sent a recovery OTP to ${options.maskedEmail || options.email || "your email"}.`,
            body: `
                <form class="ec-stack" id="index-recovery-otp-form" novalidate>
                    ${options.message ? `
                        <p class="ec-index-recovery-note">
                            ${escapeHTML(options.message)}
                        </p>
                    ` : ""}

                    <div
                        class="ec-index-otp-grid"
                        id="index-recovery-otp-grid"
                        aria-label="6-digit one-time password"
                    >
                        ${Array.from({ length: 6 }, (_, index) => `
                            <input
                                class="ec-index-otp-input"
                                type="text"
                                inputmode="numeric"
                                pattern="[0-9]*"
                                maxlength="1"
                                autocomplete="${index === 0 ? "one-time-code" : "off"}"
                                aria-label="OTP digit ${index + 1}"
                                data-otp-digit="${index}"
                                required
                            >
                        `).join("")}
                    </div>

                    <p class="ec-index-recovery-hint" data-otp-expiry>
                        This OTP expires in 15 minutes.
                    </p>

                    <p
                        class="ec-index-login-error ec-hidden"
                        id="index-recovery-error"
                        role="alert"
                    ></p>

                    <button
                        class="ec-button ec-button-primary ec-w-full"
                        id="index-recovery-verify"
                        type="submit"
                    >
                        Verify OTP
                    </button>

                    <button
                        class="ec-button ec-button-secondary ec-w-full"
                        type="button"
                        data-recovery-change-email
                    >
                        Use Another Email
                    </button>
                </form>
            `,
            footer: backToLoginButton()
        });
    }

    function renderRecoveryPassword(options = {}) {
        return modalShell({
            id: "index-recovery-password-modal",
            label: "NEW PASSWORD",
            title: "Create a new password",
            description: "Your OTP was verified. Set a new password for this account.",
            body: `
                <form class="ec-stack" id="index-recovery-password-form" novalidate>
                    ${passwordControl({
                        id: "index-recovery-new-password",
                        name: "new_password",
                        label: "New password",
                        autocomplete: "new-password",
                        placeholder: "At least 8 characters"
                    })}

                    ${passwordControl({
                        id: "index-recovery-confirm-password",
                        name: "confirm_password",
                        label: "Confirm new password",
                        autocomplete: "new-password",
                        placeholder: "Enter the same password again"
                    })}

                    <p class="ec-index-recovery-hint">
                        The password reset authorization is temporary. Complete this step before it expires.
                    </p>

                    <p
                        class="ec-index-login-error ec-hidden"
                        id="index-recovery-error"
                        role="alert"
                    ></p>

                    <button
                        class="ec-button ec-button-primary ec-w-full"
                        id="index-recovery-reset"
                        type="submit"
                    >
                        Update Password
                    </button>
                </form>
            `,
            footer: backToLoginButton()
        });
    }

    function backToLoginButton() {
        return `
            <button
                class="ec-index-login-footer-action"
                type="button"
                data-recovery-back-login
            >
                Back to login
            </button>
        `;
    }

    function eyeIcon() {
        return `
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M12 5c5.5 0 9.5 4.6 10.7 6.2a1.3 1.3 0 0 1 0 1.6C21.5 14.4 17.5 19 12 19S2.5 14.4 1.3 12.8a1.3 1.3 0 0 1 0-1.6C2.5 9.6 6.5 5 12 5Zm0 2C8 7 4.8 10.1 3.3 12 4.8 13.9 8 17 12 17s7.2-3.1 8.7-5C19.2 10.1 16 7 12 7Zm0 1.8A3.2 3.2 0 1 1 12 15.2 3.2 3.2 0 0 1 12 8.8Zm0 2A1.2 1.2 0 1 0 12 13.2 1.2 1.2 0 0 0 12 10.8Z"/>
            </svg>
        `;
    }

    function eyeOffIcon() {
        return `
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="m3.3 2 18.7 18.7-1.3 1.3-3.1-3.1A11.5 11.5 0 0 1 12 20C6.5 20 2.5 15.4 1.3 13.8a1.3 1.3 0 0 1 0-1.6A20.2 20.2 0 0 1 5.2 8L2 4.8 3.3 2Zm3.4 7.4A17.7 17.7 0 0 0 3.3 13c1.5 1.9 4.7 5 8.7 5 1.5 0 2.8-.4 4-1l-2-2a4 4 0 0 1-5-5l-2.3-2.2Zm3.9 3.9a2 2 0 0 0 2 0l-2-2a2 2 0 0 0 0 2ZM12 6c5.5 0 9.5 4.6 10.7 6.2a1.3 1.3 0 0 1 0 1.6 18 18 0 0 1-2.6 3l-1.4-1.4A16.4 16.4 0 0 0 20.7 13C19.2 11.1 16 8 12 8c-.7 0-1.4.1-2 .3L8.4 6.7A11.7 11.7 0 0 1 12 6Z"/>
            </svg>
        `;
    }

    function escapeHTML(value) {
        return String(value ?? "")
            .replaceAll("&", "&amp;")
            .replaceAll("<", "&lt;")
            .replaceAll(">", "&gt;")
            .replaceAll('"', "&quot;")
            .replaceAll("'", "&#039;");
    }

    return {
        render,
        renderRecoveryCheck,
        renderRecoveryUnavailable,
        renderRecoveryEmail,
        renderRecoveryOTP,
        renderRecoveryPassword
    };
})();

window.LoginStructure = LoginStructure;

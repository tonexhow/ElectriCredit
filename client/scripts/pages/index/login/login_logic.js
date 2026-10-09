/**
 * ElectriCredit - Public Login and Account Recovery Logic
 */

const LoginLogic = (() => {
    let submitting = false;
    let keyHandler = null;
    let expiryTimer = null;
    let renderVersion = 0;
    let recoveryEmail = "";
    let resetToken = "";

    function open(options = {}) {
        window.ElectriCredit?.preloadAdminWorkspaceAssets?.().catch?.(() => {});
        recoveryEmail = String(options.email || "").trim();
        resetToken = "";
        renderLogin();
    }

    function renderLogin() {
        renderView(
            window.LoginStructure?.render?.({ email: recoveryEmail }) || "",
            "#index-login-email"
        );

        const root = ensureRoot();
        bindPasswordToggles(root);
        root.querySelector("#index-login-form")?.addEventListener("submit", handleLoginSubmit);
        root.querySelector("[data-recover-account]")?.addEventListener("click", openRecovery);
    }

    async function openRecovery() {
        recoveryEmail = "";
        resetToken = "";
        renderView(window.LoginStructure?.renderRecoveryCheck?.() || "");

        const version = renderVersion;

        if (navigator.onLine === false) {
            if (version === renderVersion) {
                renderRecoveryUnavailable("This device is currently offline. Connect to the internet and check again.");
            }
            return;
        }

        try {
            const result = await window.Auth?.getRecoveryStatus?.();

            if (version !== renderVersion) {
                return;
            }

            if (!result?.available || !result?.online) {
                renderRecoveryUnavailable(
                    result?.message || "ElectriCredit cannot reach the email service right now."
                );
                return;
            }

            renderRecoveryEmail();
        } catch (error) {
            if (version === renderVersion) {
                renderRecoveryUnavailable(
                    error?.message || "Unable to check email recovery availability."
                );
            }
        }
    }

    function renderRecoveryUnavailable(message) {
        renderView(
            window.LoginStructure?.renderRecoveryUnavailable?.(message) || ""
        );

        const root = ensureRoot();
        root.querySelector("[data-recovery-retry]")?.addEventListener("click", openRecovery);
    }

    function renderRecoveryEmail(options = {}) {
        renderView(
            window.LoginStructure?.renderRecoveryEmail?.({
                email: options.email ?? recoveryEmail,
                message: options.message || ""
            }) || "",
            "#index-recovery-email"
        );

        ensureRoot()
            .querySelector("#index-recovery-email-form")
            ?.addEventListener("submit", handleRecoveryEmailSubmit);
    }

    function renderRecoveryOTP(options = {}) {
        renderView(
            window.LoginStructure?.renderRecoveryOTP?.({
                email: recoveryEmail,
                maskedEmail: options.maskedEmail || maskEmail(recoveryEmail),
                message: options.message || ""
            }) || "",
            "[data-otp-digit='0']"
        );

        const root = ensureRoot();
        bindOTPInputs(root);
        root.querySelector("#index-recovery-otp-form")?.addEventListener("submit", handleRecoveryOTPSubmit);
        root.querySelector("[data-recovery-change-email]")?.addEventListener("click", () => {
            renderRecoveryEmail({ email: recoveryEmail });
        });

        if (options.expiresAt) {
            startExpiryCountdown(Number(options.expiresAt));
        }
    }

    function renderRecoveryPassword() {
        renderView(
            window.LoginStructure?.renderRecoveryPassword?.({ email: recoveryEmail }) || "",
            "#index-recovery-new-password"
        );

        const root = ensureRoot();
        bindPasswordToggles(root);
        root.querySelector("#index-recovery-password-form")?.addEventListener("submit", handleRecoveryPasswordSubmit);
    }

    async function handleLoginSubmit(event) {
        event.preventDefault();

        if (submitting) {
            return;
        }

        const form = event.currentTarget;

        if (!form.checkValidity()) {
            form.reportValidity();
            return;
        }

        const email = form.elements.email?.value.trim() || "";
        const password = form.elements.password?.value || "";

        clearError("index-login-error");
        setButtonBusy("index-login-submit", true, "Signing In…", "Sign In");
        submitting = true;

        try {
            const result = await window.Auth.login({ email, password });
            const user = result?.user || window.Auth.getUser?.() || result;
            const role = String(user?.role || "").trim().toLowerCase();

            if (!["owner", "admin", "developer"].includes(role)) {
                await safeLogout();
                throw new Error("This account is not authorized for administrative access.");
            }

            window.Notification?.success?.("Signed in successfully.");
            redirectByRole(role);
        } catch (error) {
            showError("index-login-error", error?.message || "Unable to sign in. Please try again.");
        } finally {
            submitting = false;
            setButtonBusy("index-login-submit", false, "Signing In…", "Sign In");
        }
    }

    async function handleRecoveryEmailSubmit(event) {
        event.preventDefault();

        if (submitting) {
            return;
        }

        const form = event.currentTarget;
        if (!form.checkValidity()) {
            form.reportValidity();
            return;
        }

        recoveryEmail = form.elements.email?.value.trim().toLowerCase() || "";
        clearError("index-recovery-error");
        submitting = true;
        setButtonBusy("index-recovery-send", true, "Sending OTP…", "Send OTP");

        try {
            const result = await window.Auth.requestRecoveryOTP(recoveryEmail);
            renderRecoveryOTP({
                expiresAt: result?.expires_at,
                message: result?.message || "OTP sent. Check your email inbox."
            });
        } catch (error) {
            if (error?.status === 409 && error?.data?.active) {
                renderRecoveryOTP({
                    expiresAt: error.data.expires_at,
                    message: "An OTP is already active for this account. Use the email you already received."
                });
                return;
            }

            if (error?.status === 429 && error?.data?.retry_after_seconds) {
                showError(
                    "index-recovery-error",
                    `OTP send limit reached. Try again in ${formatDuration(error.data.retry_after_seconds)}.`
                );
                return;
            }

            showError(
                "index-recovery-error",
                error?.message || "Unable to send the recovery OTP."
            );
        } finally {
            submitting = false;
            setButtonBusy("index-recovery-send", false, "Sending OTP…", "Send OTP");
        }
    }

    async function handleRecoveryOTPSubmit(event) {
        event.preventDefault();

        if (submitting) {
            return;
        }

        const otp = readOTP();
        if (otp.length !== 6) {
            showError("index-recovery-error", "Enter the complete 6-digit OTP.");
            return;
        }

        clearError("index-recovery-error");
        submitting = true;
        setButtonBusy("index-recovery-verify", true, "Verifying…", "Verify OTP");

        try {
            const result = await window.Auth.verifyRecoveryOTP(recoveryEmail, otp);
            resetToken = result?.reset_token || "";

            if (!resetToken) {
                throw new Error("Account recovery authorization was not returned.");
            }

            renderRecoveryPassword();
        } catch (error) {
            showError(
                "index-recovery-error",
                error?.message || "The OTP could not be verified."
            );
        } finally {
            submitting = false;
            setButtonBusy("index-recovery-verify", false, "Verifying…", "Verify OTP");
        }
    }

    async function handleRecoveryPasswordSubmit(event) {
        event.preventDefault();

        if (submitting) {
            return;
        }

        const form = event.currentTarget;
        if (!form.checkValidity()) {
            form.reportValidity();
            return;
        }

        const newPassword = form.elements.new_password?.value || "";
        const confirmPassword = form.elements.confirm_password?.value || "";

        if (newPassword !== confirmPassword) {
            showError("index-recovery-error", "New password and confirmation do not match.");
            return;
        }

        clearError("index-recovery-error");
        submitting = true;
        setButtonBusy("index-recovery-reset", true, "Updating…", "Update Password");

        try {
            await window.Auth.resetRecoveredPassword({
                email: recoveryEmail,
                reset_token: resetToken,
                new_password: newPassword,
                confirm_password: confirmPassword
            });

            const email = recoveryEmail;
            resetToken = "";
            window.Notification?.success?.("Password updated successfully. Sign in with your new password.");
            open({ email });
        } catch (error) {
            showError(
                "index-recovery-error",
                error?.message || "Unable to update the password."
            );
        } finally {
            submitting = false;
            setButtonBusy("index-recovery-reset", false, "Updating…", "Update Password");
        }
    }

    function bindPasswordToggles(root) {
        root.querySelectorAll("[data-password-toggle]").forEach(button => {
            button.addEventListener("click", () => {
                const input = root.querySelector(`#${button.dataset.passwordTarget}`);
                if (!input) {
                    return;
                }

                const showing = input.type === "text";
                input.type = showing ? "password" : "text";
                button.setAttribute("aria-pressed", String(!showing));
                button.setAttribute(
                    "aria-label",
                    `${showing ? "Show" : "Hide"} ${input.name.replaceAll("_", " ")}`
                );
            });
        });
    }

    function bindOTPInputs(root) {
        const inputs = [...root.querySelectorAll("[data-otp-digit]")];

        inputs.forEach((input, index) => {
            input.addEventListener("input", event => {
                const digits = String(event.target.value || "").replace(/\D/g, "");

                if (digits.length > 1) {
                    distributeOTP(inputs, digits, index);
                    return;
                }

                event.target.value = digits.slice(-1);
                if (event.target.value && inputs[index + 1]) {
                    inputs[index + 1].focus();
                }
            });

            input.addEventListener("keydown", event => {
                if (event.key === "Backspace" && !input.value && inputs[index - 1]) {
                    inputs[index - 1].focus();
                    return;
                }

                if (event.key === "ArrowLeft" && inputs[index - 1]) {
                    event.preventDefault();
                    inputs[index - 1].focus();
                }

                if (event.key === "ArrowRight" && inputs[index + 1]) {
                    event.preventDefault();
                    inputs[index + 1].focus();
                }
            });

            input.addEventListener("paste", event => {
                const digits = event.clipboardData?.getData("text")?.replace(/\D/g, "") || "";
                if (!digits) {
                    return;
                }

                event.preventDefault();
                distributeOTP(inputs, digits, index);
            });
        });
    }

    function distributeOTP(inputs, digits, startIndex = 0) {
        const values = digits.slice(0, inputs.length - startIndex).split("");

        values.forEach((digit, offset) => {
            if (inputs[startIndex + offset]) {
                inputs[startIndex + offset].value = digit;
            }
        });

        const nextIndex = Math.min(startIndex + values.length, inputs.length - 1);
        inputs[nextIndex]?.focus();
    }

    function readOTP() {
        return [...document.querySelectorAll("[data-otp-digit]")]
            .map(input => String(input.value || "").replace(/\D/g, "").slice(0, 1))
            .join("");
    }

    function startExpiryCountdown(expiresAt) {
        stopExpiryCountdown();

        const update = () => {
            const element = document.querySelector("[data-otp-expiry]");
            if (!element) {
                stopExpiryCountdown();
                return;
            }

            const remaining = Math.max(0, expiresAt - Math.floor(Date.now() / 1000));
            if (remaining <= 0) {
                element.textContent = "This OTP has expired. Return and request a new OTP.";
                stopExpiryCountdown();
                return;
            }

            element.textContent = `OTP expires in ${formatDuration(remaining)}.`;
        };

        update();
        expiryTimer = window.setInterval(update, 1000);
    }

    function stopExpiryCountdown() {
        if (expiryTimer) {
            window.clearInterval(expiryTimer);
            expiryTimer = null;
        }
    }

    function renderView(html, focusSelector = "") {
        window.LoginDesign?.mount();

        if (!window.LoginStructure) {
            console.error("[ElectriCredit Login] Login structure is unavailable.");
            return;
        }

        renderVersion++;
        stopExpiryCountdown();

        const root = ensureRoot();
        root.innerHTML = html;
        document.body.classList.add("ec-login-open");

        bindCommon(root);
        ensureEscapeHandler();

        if (focusSelector) {
            requestAnimationFrame(() => root.querySelector(focusSelector)?.focus());
        }
    }

    function bindCommon(root) {
        root.querySelectorAll("[data-login-close]").forEach(button => {
            button.addEventListener("click", close);
        });

        root.querySelectorAll("[data-recovery-back-login]").forEach(button => {
            button.addEventListener("click", () => open({ email: recoveryEmail }));
        });

        const backdrop = root.firstElementChild;
        backdrop?.addEventListener("click", event => {
            if (event.target === backdrop) {
                close();
            }
        });
    }

    function ensureEscapeHandler() {
        if (keyHandler) {
            return;
        }

        keyHandler = event => {
            if (event.key === "Escape") {
                close();
            }
        };

        document.addEventListener("keydown", keyHandler);
    }

    function showError(id, message) {
        const element = document.getElementById(id);
        if (!element) {
            return;
        }

        element.textContent = message;
        element.classList.remove("ec-hidden");
    }

    function clearError(id) {
        const element = document.getElementById(id);
        if (!element) {
            return;
        }

        element.textContent = "";
        element.classList.add("ec-hidden");
    }

    function setButtonBusy(id, busy, busyLabel, idleLabel) {
        const button = document.getElementById(id);
        if (!button) {
            return;
        }

        button.disabled = busy;
        button.textContent = busy ? busyLabel : idleLabel;
    }

    function redirectByRole(role) {
        if (["admin", "owner", "developer"].includes(String(role || "").toLowerCase())) {
            window.location.href = "admin.html";
            return;
        }
        window.location.href = "index.html";
    }

    async function safeLogout() {
        try {
            await window.Auth?.logout?.();
        } catch {
            // Ignore cleanup failure on a rejected public login.
        }
    }

    function maskEmail(email) {
        const [name = "", domain = ""] = String(email || "").split("@");
        if (!name || !domain) {
            return email;
        }

        const visible = name.slice(0, Math.min(2, name.length));
        return `${visible}${"*".repeat(Math.max(2, name.length - visible.length))}@${domain}`;
    }

    function formatDuration(seconds) {
        const total = Math.max(0, Math.ceil(Number(seconds) || 0));
        const minutes = Math.floor(total / 60);
        const remainder = total % 60;

        if (minutes > 0 && remainder > 0) {
            return `${minutes}m ${remainder}s`;
        }

        if (minutes > 0) {
            return `${minutes}m`;
        }

        return `${remainder}s`;
    }

    function ensureRoot() {
        let root = document.getElementById("index-login-root");

        if (!root) {
            root = document.createElement("div");
            root.id = "index-login-root";
            document.body.appendChild(root);
        }

        return root;
    }

    function close() {
        submitting = false;
        renderVersion++;
        stopExpiryCountdown();

        const root = document.getElementById("index-login-root");
        if (root) {
            root.innerHTML = "";
        }

        document.body.classList.remove("ec-login-open");

        if (keyHandler) {
            document.removeEventListener("keydown", keyHandler);
            keyHandler = null;
        }
    }

    function unmount() {
        close();
        window.LoginDesign?.unmount();
    }

    return {
        open,
        close,
        unmount
    };
})();

window.LoginLogic = LoginLogic;

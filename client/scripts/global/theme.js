/**
 * ElectriCredit - Global Theme Manager
 * Preferences: light, system (Auto), dark.
 */

const Theme = (() => {
    const STORAGE_KEY = "electricredit-theme";
    const VALID = ["light", "system", "dark"];
    let systemListenerBound = false;

    function initialize() {
        const preference = getPreference();
        apply(preference, { persist: false });
        bindSystemTheme();
        syncControls();
        return preference;
    }

    function set(preference) {
        return apply(preference, { persist: true });
    }

    function apply(preference, options = {}) {
        const selected = VALID.includes(preference)
            ? preference
            : "system";

        const resolved =
            selected === "system"
                ? getSystemTheme()
                : selected;

        document.documentElement.dataset.theme = resolved;
        document.documentElement.dataset.themePreference = selected;

        if (options.persist !== false) {
            localStorage.setItem(STORAGE_KEY, selected);
        }

        updateThemeColor(resolved);
        syncControls();

        window.dispatchEvent(
            new CustomEvent("electricredit:themechange", {
                detail: {
                    preference: selected,
                    theme: resolved
                }
            })
        );

        return selected;
    }

    function getPreference() {
        const saved = localStorage.getItem(STORAGE_KEY);
        return VALID.includes(saved) ? saved : "system";
    }

    function getCurrent() {
        return document.documentElement.dataset.theme || getSystemTheme();
    }

    function getSystemTheme() {
        return window.matchMedia("(prefers-color-scheme: dark)").matches
            ? "dark"
            : "light";
    }

    function bindControls(root = document) {
        root.querySelectorAll("[data-theme-option]").forEach(button => {
            if (button.dataset.themeBound === "true") {
                return;
            }

            button.dataset.themeBound = "true";

            button.addEventListener("click", () => {
                set(button.dataset.themeOption);
            });
        });

        syncControls(root);
    }

    function syncControls(root = document) {
        const preference = getPreference();

        root.querySelectorAll("[data-theme-option]").forEach(button => {
            const active = button.dataset.themeOption === preference;
            button.classList.toggle("is-active", active);
            button.setAttribute("aria-pressed", String(active));
        });

        root.querySelectorAll(".ec-index-theme-control").forEach(control => {
            control.dataset.themeValue = preference;
        });
    }

    function toggle() {
        return set(getCurrent() === "dark" ? "light" : "dark");
    }

    function bindToggle(selector = "#theme-toggle") {
        const element =
            typeof selector === "string"
                ? document.querySelector(selector)
                : selector;

        if (!element || element.dataset.themeBound === "true") {
            return;
        }

        element.dataset.themeBound = "true";
        element.addEventListener("click", toggle);
    }

    function bindSystemTheme() {
        if (systemListenerBound) {
            return;
        }

        systemListenerBound = true;

        const media = window.matchMedia("(prefers-color-scheme: dark)");
        media.addEventListener("change", () => {
            if (getPreference() === "system") {
                apply("system", { persist: false });
            }
        });
    }

    function updateThemeColor(theme) {
        const meta = document.querySelector('meta[name="theme-color"]');

        if (meta) {
            meta.setAttribute(
                "content",
                theme === "dark" ? "#111418" : "#f2b705"
            );
        }
    }

    return {
        initialize,
        set,
        apply,
        getPreference,
        getCurrent,
        getSystemTheme,
        bindControls,
        bindToggle,
        toggle
    };
})();

window.Theme = Theme;

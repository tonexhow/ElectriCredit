/**
 * ElectriCredit - Public Home Logic
 */

const HomeLogic = (() => {
    let mounted = false;

    async function mount(options = {}) {
        window.HomeDesign?.mount();

        const target = resolveTarget(options.target);

        if (!target || !window.HomeStructure) {
            console.error(
                "[ElectriCredit Home] Unable to mount the Home page."
            );
            return;
        }

        mounted = true;

        const data = await loadHomeData(options.data);

        if (!mounted) {
            return;
        }

        target.innerHTML = window.HomeStructure.render(data);
        bindNavigation(target);
    }

    async function loadHomeData(providedData) {
        if (providedData) {
            return normalizeData(providedData);
        }

        /*
         * Friendly fallback values are used until the corresponding
         * public REST endpoints are connected.
         */
        const fallback = {
            pricePerKWh: null,
            registryCount: 0,
            hubCount: 0,
            userCount: 0
        };

        if (!window.API?.get) {
            return fallback;
        }

        try {
            const data = await window.API.get("/public/home");
            return normalizeData(data);
        } catch {
            return fallback;
        }
    }

    function normalizeData(data = {}) {
        return {
            greeting: data.greeting,
            pricePerKWh:
                data.price_per_kwh ??
                data.pricePerKWh ??
                null,
            registryCount:
                data.registry_count ??
                data.registryCount ??
                0,
            hubCount:
                data.hub_count ??
                data.hubCount ??
                0,
            userCount:
                data.tenant_count ??
                data.tenantCount ??
                data.user_count ??
                data.userCount ??
                0
        };
    }

    function bindNavigation(target) {
        target
            .querySelectorAll("[data-home-route]")
            .forEach(button => {
                button.addEventListener("click", () => {
                    navigate(button.dataset.homeRoute);
                });
            });
    }

    function navigate(route) {
        if (window.ElectriCredit?.setRoute) {
            window.ElectriCredit.setRoute(route);
            return;
        }

        window.location.hash = route;
    }

    function resolveTarget(target) {
        if (target instanceof Element) {
            return target;
        }

        if (typeof target === "string") {
            return document.querySelector(target);
        }

        return (
            document.getElementById("page-content") ||
            document.getElementById("app")
        );
    }

    function unmount() {
        mounted = false;
        window.HomeDesign?.unmount();
    }

    return {
        mount,
        unmount
    };
})();

window.HomeLogic = HomeLogic;

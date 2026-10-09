/**
 * ElectriCredit - Public Registry Logic
 *
 * Ready for the future public REST endpoint:
 * GET /api/public/registries
 *
 * The UI does not expose backend implementation details.
 */

const RegistryLogic = (() => {
    let mounted = false;

    async function mount(options = {}) {
        window.RegistryDesign?.mount();

        const target = resolveTarget(options.target);

        if (!target || !window.RegistryStructure) {
            console.error(
                "[ElectriCredit Registry] Unable to mount Registry page."
            );
            return;
        }

        mounted = true;

        const registries = await loadRegistries(options.data);

        if (!mounted) {
            return;
        }

        target.innerHTML = window.RegistryStructure.render(registries);
    }

    async function loadRegistries(providedData) {
        if (Array.isArray(providedData)) {
            return normalizeRegistries(providedData);
        }

        if (!window.API?.get) {
            return [];
        }

        try {
            const response = await window.API.get("/public/registries");

            const records =
                Array.isArray(response)
                    ? response
                    : response?.registries;

            return normalizeRegistries(records);
        } catch {
            /*
             * Keep the public UI friendly if the endpoint is not implemented
             * yet. No server/connectivity information is displayed.
             */
            return [];
        }
    }

    function normalizeRegistries(records) {
        if (!Array.isArray(records)) {
            return [];
        }

        return records.map(record => ({
            id: record.id ?? null,
            registry_id:
                record.registry_id ??
                record.registryId ??
                null,
            public_id:
                record.public_id ??
                record.publicId ??
                record.registry_code ??
                null,
            name:
                record.name ??
                record.display_name ??
                record.displayName ??
                "Registry Station",
            location:
                record.location ??
                record.name_location ??
                record.nameLocation ??
                "Not specified",
            status:
                record.status ??
                "idle",
            active_card_id:
                record.active_card_id ??
                record.activeCardId ??
                null
        }));
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
        window.RegistryDesign?.unmount();
    }

    return {
        mount,
        unmount
    };
})();

window.RegistryLogic = RegistryLogic;

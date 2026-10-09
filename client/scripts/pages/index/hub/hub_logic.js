/**
 * ElectriCredit - Public Hub Logic
 *
 * Ready for the future public REST endpoint:
 * GET /api/public/hubs
 */

const HubLogic = (() => {
    let mounted = false;

    async function mount(options = {}) {
        window.HubDesign?.mount();

        const target = resolveTarget(options.target);

        if (!target || !window.HubStructure) {
            console.error(
                "[ElectriCredit Hub] Unable to mount Hub page."
            );
            return;
        }

        mounted = true;

        const hubs = await loadHubs(options.data);

        if (!mounted) {
            return;
        }

        target.innerHTML = window.HubStructure.render(hubs);
    }

    async function loadHubs(providedData) {
        if (Array.isArray(providedData)) {
            return normalizeHubs(providedData);
        }

        if (!window.API?.get) {
            return [];
        }

        try {
            const response = await window.API.get("/public/hubs");

            const records =
                Array.isArray(response)
                    ? response
                    : response?.hubs;

            return normalizeHubs(records);
        } catch {
            return [];
        }
    }

    function normalizeHubs(records) {
        if (!Array.isArray(records)) {
            return [];
        }

        return records.map(record => ({
            id: record.id ?? null,
            hub_id:
                record.hub_id ??
                record.hubId ??
                null,
            public_id:
                record.public_id ??
                record.publicId ??
                record.hub_code ??
                null,
            name:
                record.name ??
                record.display_name ??
                record.displayName ??
                "Hub Module",
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
        window.HubDesign?.unmount();
    }

    return {
        mount,
        unmount
    };
})();

window.HubLogic = HubLogic;

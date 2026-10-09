/**
 * ElectriCredit - Public About Logic
 *
 * Prepared for a future public team endpoint:
 * GET /api/public/about
 *
 * If no endpoint exists yet, the page still renders a friendly final UI.
 */

const AboutLogic = (() => {
    let mounted = false;

    async function mount(options = {}) {
        window.AboutDesign?.mount();

        const target = resolveTarget(options.target);

        if (!target || !window.AboutStructure) {
            console.error(
                "[ElectriCredit About] Unable to mount About page."
            );
            return;
        }

        mounted = true;

        const data = await loadAboutData(options.data);

        if (!mounted) {
            return;
        }

        target.innerHTML = window.AboutStructure.render(data);
        bindNavigation(target);
    }

    async function loadAboutData(providedData) {
        if (providedData && typeof providedData === "object") {
            return normalizeData(providedData);
        }

        const fallback = {
            owner: null,
            developers: [],
            admins: []
        };

        if (!window.API?.get) {
            return fallback;
        }

        try {
            const response = await window.API.get("/public/about");
            return normalizeData(response);
        } catch {
            return fallback;
        }
    }

    function normalizeData(data = {}) {
        return {
            owner: data.owner ? normalizePerson(data.owner, "Owner") : null,

            developers: normalizePeople(
                data.developers,
                "Developer"
            ),

            admins: normalizePeople(
                data.admins || data.administrators,
                "Administrator"
            )
        };
    }

    function normalizePeople(records, fallbackRole) {
        if (!Array.isArray(records)) {
            return [];
        }

        return records.map(record =>
            normalizePerson(record, fallbackRole)
        );
    }

    function normalizePerson(record = {}, fallbackRole) {
        if (typeof record === "string") {
            return {
                name: record,
                role: fallbackRole,
                image_url: "assets/default-image.png"
            };
        }

        return {
            name:
                record.name ??
                record.full_name ??
                record.fullName ??
                "Team Member",
            role:
                record.role ??
                record.title ??
                fallbackRole,
            image_url:
                record.image_url ??
                record.imageUrl ??
                "assets/default-image.png"
        };
    }

    function bindNavigation(target) {
        target
            .querySelectorAll("[data-about-route]")
            .forEach(button => {
                button.addEventListener("click", () => {
                    navigate(button.dataset.aboutRoute);
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
        window.AboutDesign?.unmount();
    }

    return {
        mount,
        unmount
    };
})();

window.AboutLogic = AboutLogic;

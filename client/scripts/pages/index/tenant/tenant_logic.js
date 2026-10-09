/**
 * ElectriCredit - Public Tenant Logic
 *
 * Prepared for:
 * GET /api/public/users
 * GET /api/public/users/{id}
 *
 * Public privacy masking is also applied in the UI. The backend should
 * ultimately decide which fields are safe to return publicly.
 */

const TenantLogic = (() => {
    let mounted = false;
    let users = [];
    let keyHandler = null;

    async function mount(options = {}) {
        window.TenantDesign?.mount();

        const target = resolveTarget(options.target);

        if (!target || !window.TenantStructure) {
            console.error(
                "[ElectriCredit Tenant] Unable to mount Tenant page."
            );
            return;
        }

        mounted = true;
        users = await loadUsers(options.data);

        if (!mounted) {
            return;
        }

        target.innerHTML = window.TenantStructure.render(users);
        bindUserActions(target);
    }

    async function loadUsers(providedData) {
        if (Array.isArray(providedData)) {
            return normalizeUsers(providedData);
        }

        if (!window.API?.get) {
            return [];
        }

        try {
            const response = await window.API.get("/public/users");

            const records =
                Array.isArray(response)
                    ? response
                    : response?.users;

            return normalizeUsers(records);
        } catch {
            return [];
        }
    }

    function normalizeUsers(records) {
        if (!Array.isArray(records)) {
            return [];
        }

        return records.map(record => ({
            id: record.id ?? null,
            name:
                record.name ??
                record.full_name ??
                record.fullName ??
                "User",
            email: record.email ?? "",
            phone:
                record.phone ??
                record.phone_number ??
                record.phoneNumber ??
                "",
            status: record.status ?? "active",
            image_url:
                record.image_url ??
                record.imageUrl ??
                "assets/default-image.png",
            cards: normalizeCards(record.cards)
        }));
    }

    function normalizeCards(cards) {
        if (!Array.isArray(cards)) {
            return [];
        }

        return cards.map(card => ({
            id: card.id ?? null,
            uid:
                card.uid ??
                card.rfid_uid ??
                card.rfidUid ??
                "",
            name:
                card.name ??
                card.label ??
                "RFID Card",
            status: card.status ?? "active",
            balance:
                card.balance ??
                card.balance_amount ??
                card.balanceAmount ??
                null,
            debt_limit:
                card.debt_limit ??
                card.debit_limit ??
                card.debtLimit ??
                null
        }));
    }

    function bindUserActions(target) {
        target
            .querySelectorAll("[data-user-view]")
            .forEach(button => {
                button.addEventListener("click", () => {
                    openUser(button.dataset.userView);
                });
            });
    }

    async function openUser(identifier) {
        let user = findUser(identifier);

        /*
         * If the list endpoint only returns summaries, the future backend
         * may provide additional public-safe details through this endpoint.
         */
        if (window.API?.get) {
            try {
                const response = await window.API.get(
                    `/public/users/${encodeURIComponent(identifier)}`
                );

                if (response?.user || response?.id) {
                    user = normalizeUsers([
                        response.user || response
                    ])[0];
                }
            } catch {
                // Keep using the already-loaded public summary.
            }
        }

        if (!user) {
            return;
        }

        let root = document.getElementById("users-modal-root");
        if (!root || root.parentElement !== document.body) {
            root?.remove();
            root = document.createElement("div");
            root.id = "users-modal-root";
            root.className = "ec-index-modal-portal";
            document.body.appendChild(root);
        }

        root.innerHTML = window.TenantStructure.renderUserModal(user);
        document.body.classList.add("ec-modal-open");

        root
            .querySelector("[data-user-modal-close]")
            ?.addEventListener("click", closeModal);

        root
            .querySelector("#index-user-modal")
            ?.addEventListener("click", event => {
                if (event.target.id === "index-user-modal") {
                    closeModal();
                }
            });

        keyHandler = event => {
            if (event.key === "Escape") {
                closeModal();
            }
        };

        document.addEventListener("keydown", keyHandler);

        root
            .querySelector("[data-user-modal-close]")
            ?.focus();
    }

    function findUser(identifier) {
        return users.find(user =>
            String(user.id) === String(identifier)
        );
    }

    function closeModal() {
        const root = document.getElementById("users-modal-root");

        if (root) {
            root.remove();
        }

        document.body.classList.remove("ec-modal-open");

        if (keyHandler) {
            document.removeEventListener("keydown", keyHandler);
            keyHandler = null;
        }
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
        closeModal();
        users = [];
        window.TenantDesign?.unmount();
    }

    return {
        mount,
        unmount
    };
})();

window.TenantLogic = TenantLogic;

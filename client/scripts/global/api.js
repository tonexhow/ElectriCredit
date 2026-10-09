/**
 * ElectriCredit - Global REST API Client
 */

const API = (() => {
    function resolveBaseURL() {
        if (window.ELECTRICREDIT_API_BASE) {
            return String(window.ELECTRICREDIT_API_BASE).replace(/\/$/, "");
        }

        const localHost =
            window.location.hostname === "127.0.0.1" ||
            window.location.hostname === "localhost";

        /*
         * When opened through the Go server, use same-origin /api.
         * When opened through VS Code Live Server locally, use :8080.
         */
        if (localHost && window.location.port && window.location.port !== "8080") {
            return `${window.location.protocol}//${window.location.hostname}:8080/api`;
        }

        return "/api";
    }

    async function request(endpoint, options = {}) {
        const baseURL = resolveBaseURL();
        const path = endpoint.startsWith("/") ? endpoint : `/${endpoint}`;

        const timeoutMs = Math.max(1000, Number(options.timeoutMs || 12000));
        const controller = typeof AbortController !== "undefined" ? new AbortController() : null;
        const timeout = controller
            ? window.setTimeout(() => controller.abort(), timeoutMs)
            : null;

        const config = {
            method: options.method || "GET",
            headers: {
                "Accept": "application/json",
                ...(options.headers || {})
            },
            credentials: "include",
            ...(controller ? { signal: controller.signal } : {})
        };

        if (options.body !== undefined) {
            config.headers["Content-Type"] = "application/json";
            config.body =
                typeof options.body === "string"
                    ? options.body
                    : JSON.stringify(options.body);
        }

        let response;
        try {
            response = await fetch(`${baseURL}${path}`, config);
        } catch (error) {
            if (error?.name === "AbortError") {
                const timeoutError = new Error("ElectriCredit server did not respond in time.");
                timeoutError.code = "REQUEST_TIMEOUT";
                throw timeoutError;
            }
            throw error;
        } finally {
            if (timeout !== null) window.clearTimeout(timeout);
        }

        const contentType = response.headers.get("content-type") || "";

        let data = null;

        if (response.status !== 204) {
            data = contentType.includes("application/json")
                ? await response.json()
                : await response.text();
        }

        if (!response.ok) {
            const message =
                data?.message ||
                data?.error ||
                `Request failed (${response.status})`;

            const error = new Error(message);
            error.status = response.status;
            error.data = data;
            throw error;
        }

        return data;
    }

    return {
        get(endpoint, options = {}) {
            return request(endpoint, options);
        },

        post(endpoint, body) {
            return request(endpoint, {
                method: "POST",
                body
            });
        },

        put(endpoint, body) {
            return request(endpoint, {
                method: "PUT",
                body
            });
        },

        patch(endpoint, body) {
            return request(endpoint, {
                method: "PATCH",
                body
            });
        },

        delete(endpoint) {
            return request(endpoint, {
                method: "DELETE"
            });
        },

        request,
        getBaseURL: resolveBaseURL
    };
})();

window.API = API;

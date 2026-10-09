const ECMarkdown = (() => {
    function escapeHTML(value) {
        return String(value ?? "")
            .replaceAll("&", "&amp;")
            .replaceAll("<", "&lt;")
            .replaceAll(">", "&gt;")
            .replaceAll('"', "&quot;")
            .replaceAll("'", "&#039;");
    }

    function safeURL(value) {
        const text = String(value || "").trim();
        if (/^https?:\/\//i.test(text)) return text;
        if (/^mailto:/i.test(text)) return text;
        return "";
    }

    function inline(value) {
        const codeTokens = [];
        let text = String(value ?? "").replace(/`([^`]+)`/g, (_, code) => {
            const token = `%%ECCODE${codeTokens.length}%%`;
            const url = safeURL(code);
            codeTokens.push(url
                ? `<a class="ec-md-link ec-md-code-link" href="${escapeHTML(url)}" target="_blank" rel="noopener noreferrer">${escapeHTML(code)}</a>`
                : `<code>${escapeHTML(code)}</code>`);
            return token;
        });

        text = escapeHTML(text);
        text = text.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+|mailto:[^\s)]+)\)/g, (_, label, url) => {
            const safe = safeURL(url);
            return safe ? `<a class="ec-md-link" href="${escapeHTML(safe)}" target="_blank" rel="noopener noreferrer">${label}</a>` : label;
        });
        text = text.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
        text = text.replace(/__([^_]+)__/g, "<strong>$1</strong>");
        text = text.replace(/(^|[^*])\*([^*\n]+)\*(?!\*)/g, "$1<em>$2</em>");
        text = text.replace(/(^|[^_])_([^_\n]+)_(?!_)/g, "$1<em>$2</em>");
        text = text.replace(/(^|[\s>(])((?:https?:\/\/)[^\s<]+)/g, (match, prefix, url) => {
            const clean = url.replace(/[.,;:!?]+$/, "");
            const suffix = url.slice(clean.length);
            return `${prefix}<a class="ec-md-link" href="${escapeHTML(clean)}" target="_blank" rel="noopener noreferrer">${escapeHTML(clean)}</a>${escapeHTML(suffix)}`;
        });
        codeTokens.forEach((html, index) => {
            text = text.replace(`%%ECCODE${index}%%`, html);
        });
        return text;
    }

    function isTableDivider(line) {
        const cells = String(line || "").trim().replace(/^\||\|$/g, "").split("|");
        return cells.length > 1 && cells.every(cell => /^\s*:?-{3,}:?\s*$/.test(cell));
    }

    function tableCells(line) {
        return String(line || "").trim().replace(/^\||\|$/g, "").split("|").map(cell => cell.trim());
    }

    function render(markdown = "") {
        const lines = String(markdown || "").replace(/\r\n?/g, "\n").split("\n");
        const out = [];
        let i = 0;

        while (i < lines.length) {
            const line = lines[i];
            const trimmed = line.trim();

            if (!trimmed) {
                i += 1;
                continue;
            }

            if (/^```/.test(trimmed)) {
                const language = trimmed.slice(3).trim();
                const code = [];
                i += 1;
                while (i < lines.length && !/^```/.test(lines[i].trim())) {
                    code.push(lines[i]);
                    i += 1;
                }
                if (i < lines.length) i += 1;
                out.push(`<div class="ec-md-code-wrap"><div class="ec-md-code-head"><span>${escapeHTML(language || "code")}</span><button type="button" data-md-copy>Copy</button></div><pre><code>${escapeHTML(code.join("\n"))}</code></pre></div>`);
                continue;
            }

            const heading = trimmed.match(/^(#{1,6})\s+(.+)$/);
            if (heading) {
                const level = Math.min(6, heading[1].length);
                out.push(`<h${level}>${inline(heading[2])}</h${level}>`);
                i += 1;
                continue;
            }

            if (/^(---+|___+|\*\*\*+)$/.test(trimmed)) {
                out.push("<hr>");
                i += 1;
                continue;
            }

            if (i + 1 < lines.length && trimmed.includes("|") && isTableDivider(lines[i + 1])) {
                const header = tableCells(line);
                i += 2;
                const rows = [];
                while (i < lines.length && lines[i].trim() && lines[i].includes("|")) {
                    rows.push(tableCells(lines[i]));
                    i += 1;
                }
                out.push(`<div class="ec-md-table-wrap"><table><thead><tr>${header.map(cell => `<th>${inline(cell)}</th>`).join("")}</tr></thead><tbody>${rows.map(row => `<tr>${header.map((_, index) => `<td>${inline(row[index] || "")}</td>`).join("")}</tr>`).join("")}</tbody></table></div>`);
                continue;
            }

            if (/^>\s?/.test(trimmed)) {
                const quote = [];
                while (i < lines.length && /^>\s?/.test(lines[i].trim())) {
                    quote.push(lines[i].trim().replace(/^>\s?/, ""));
                    i += 1;
                }
                out.push(`<blockquote>${quote.map(part => inline(part)).join("<br>")}</blockquote>`);
                continue;
            }

            const unordered = trimmed.match(/^[-*+]\s+(.+)$/);
            if (unordered) {
                const items = [];
                while (i < lines.length) {
                    const match = lines[i].trim().match(/^[-*+]\s+(.+)$/);
                    if (!match) break;
                    items.push(match[1]);
                    i += 1;
                }
                out.push(`<ul>${items.map(item => `<li>${inline(item)}</li>`).join("")}</ul>`);
                continue;
            }

            const ordered = trimmed.match(/^\d+[.)]\s+(.+)$/);
            if (ordered) {
                const items = [];
                while (i < lines.length) {
                    const match = lines[i].trim().match(/^\d+[.)]\s+(.+)$/);
                    if (!match) break;
                    items.push(match[1]);
                    i += 1;
                }
                out.push(`<ol>${items.map(item => `<li>${inline(item)}</li>`).join("")}</ol>`);
                continue;
            }

            const paragraph = [trimmed];
            i += 1;
            while (i < lines.length) {
                const next = lines[i].trim();
                if (!next || /^```/.test(next) || /^(#{1,6})\s+/.test(next) || /^[-*+]\s+/.test(next) || /^\d+[.)]\s+/.test(next) || /^>\s?/.test(next) || /^(---+|___+|\*\*\*+)$/.test(next)) break;
                if (i + 1 < lines.length && next.includes("|") && isTableDivider(lines[i + 1])) break;
                paragraph.push(next);
                i += 1;
            }
            out.push(`<p>${paragraph.map(part => inline(part)).join(" ")}</p>`);
        }

        return out.join("\n");
    }

    function bind(root) {
        root?.querySelectorAll?.("[data-md-copy]").forEach(button => {
            if (button.dataset.bound === "1") return;
            button.dataset.bound = "1";
            button.addEventListener("click", async () => {
                const code = button.closest(".ec-md-code-wrap")?.querySelector("code")?.textContent || "";
                try {
                    await navigator.clipboard.writeText(code);
                    const original = button.textContent;
                    button.textContent = "Copied";
                    setTimeout(() => { button.textContent = original; }, 1200);
                } catch (_) {}
            });
        });
    }

    return { render, bind };
})();

window.ECMarkdown = ECMarkdown;

/**
 * ElectriCredit - Global Footer
 */

const Footer = (() => {
    const icons = {
        facebook: `
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M14.5 8H17V4.5c-.43-.06-1.9-.18-3.65-.18-3.6 0-6.07 2.2-6.07 6.24V14H4v3.92h3.28V24h4.02v-6.08h3.36L15.2 14h-3.9v-3.05C11.3 9.82 11.6 8 14.5 8Z"/>
            </svg>`,
        github: `
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M12 .7a12 12 0 0 0-3.79 23.39c.6.11.82-.26.82-.58v-2.1c-3.34.73-4.04-1.42-4.04-1.42-.55-1.39-1.34-1.76-1.34-1.76-1.1-.75.08-.74.08-.74 1.21.09 1.85 1.25 1.85 1.25 1.08 1.85 2.83 1.31 3.52 1 .11-.78.42-1.31.77-1.61-2.67-.3-5.47-1.34-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.13-.3-.54-1.53.11-3.18 0 0 1.01-.32 3.3 1.23A11.5 11.5 0 0 1 12 6.62c1.02 0 2.05.14 3.01.41 2.29-1.55 3.3-1.23 3.3-1.23.65 1.65.24 2.88.12 3.18.77.84 1.23 1.91 1.23 3.22 0 4.6-2.81 5.62-5.49 5.92.43.37.82 1.1.82 2.22v3.17c0 .32.22.7.83.58A12 12 0 0 0 12 .7Z"/>
            </svg>`,
        linkedin: `
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M5.34 3.5A2.34 2.34 0 1 1 .66 3.5a2.34 2.34 0 0 1 4.68 0ZM1.06 7.2h4.56V22H1.06V7.2Zm7.29 0h4.37v2.02h.06c.61-1.15 2.1-2.37 4.31-2.37 4.61 0 5.46 3.03 5.46 6.98V22H18v-7.25c0-1.73-.03-3.95-2.41-3.95-2.41 0-2.78 1.88-2.78 3.82V22H8.35V7.2Z"/>
            </svg>`,
        contact: `
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M6.62 10.79a15.46 15.46 0 0 0 6.59 6.59l2.2-2.2a1 1 0 0 1 1.02-.24c1.12.37 2.33.57 3.57.57a1 1 0 0 1 1 1V20a1 1 0 0 1-1 1C10.61 21 3 13.39 3 4a1 1 0 0 1 1-1h3.5a1 1 0 0 1 1 1c0 1.25.2 2.45.57 3.57a1 1 0 0 1-.25 1.02l-2.2 2.2Z"/>
            </svg>`,
        email: `
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M20 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2Zm0 4-8 5-8-5V6l8 5 8-5v2Z"/>
            </svg>`,
        telegram: `
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M21.8 3.2 18.7 20c-.2 1.2-.9 1.5-1.9.9l-4.7-3.5-2.3 2.2c-.3.3-.5.5-1 .5l.3-4.8 8.8-8c.4-.3-.1-.5-.6-.2L6.4 14l-4.7-1.5c-1-.3-1-1 .2-1.5L20.2 4c.9-.3 1.8.2 1.6-.8Z"/>
            </svg>`,
        whatsapp: `
            <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M12 2a9.8 9.8 0 0 0-8.4 14.9L2 22l5.3-1.5A10 10 0 1 0 12 2Zm0 18a8 8 0 0 1-4.1-1.1l-.3-.2-3.1.9.9-3-.2-.3A8 8 0 1 1 12 20Zm4.4-6c-.2-.1-1.4-.7-1.6-.8-.2-.1-.4-.1-.6.1l-.7.9c-.1.2-.3.2-.5.1a6.6 6.6 0 0 1-3.2-2.8c-.2-.3 0-.4.1-.5l.5-.6c.1-.2.2-.3.3-.5.1-.2 0-.4 0-.5l-.7-1.7c-.2-.5-.4-.5-.6-.5h-.5c-.2 0-.5.1-.7.3-.2.3-1 1-1 2.4s1 2.8 1.2 3c.1.2 2 3.1 4.9 4.3.7.3 1.2.5 1.6.6.7.2 1.3.2 1.8.1.6-.1 1.4-.6 1.6-1.1.2-.6.2-1 .2-1.1 0-.1-.2-.2-.4-.3Z"/>
            </svg>`
    };

    function socialLink(name, label, href, options = {}) {
        const external = options.external !== false;
        const pending = options.pending === true;

        return `
            <a
                class="ec-footer-social-icon ${pending ? "is-pending" : ""}"
                href="${href}"
                aria-label="${label}"
                title="${label}${pending ? " - link pending" : ""}"
                ${pending ? 'aria-disabled="true" data-link-pending="true"' : ""}
                ${external && !pending ? 'target="_blank" rel="noopener noreferrer"' : ""}
            >
                ${icons[name]}
            </a>
        `;
    }

    function render() {
        const year = new Date().getFullYear();

        return `
            <footer class="ec-footer">
                <div class="ec-footer-content">
                    <div class="ec-footer-column ec-footer-left">
                        <div class="ec-footer-primary-line">
                            <strong>ElectriCredit</strong>
                            <span aria-hidden="true">&copy;</span>
                            <span>${year}</span>
                        </div>
                        <span class="ec-footer-secondary-line">
                            Property of Group 45
                        </span>
                    </div>

                    <div class="ec-footer-column ec-footer-center">
                        <a
                            class="ec-footer-link ec-footer-college-link"
                            href="https://buenavistacommunitycollege.edu.ph/"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            Buenavista Community College
                        </a>
                        <span class="ec-footer-secondary-line">
                            Capstone Project
                        </span>
                    </div>

                    <div class="ec-footer-column ec-footer-right">
                        <div class="ec-footer-developer-line">
                            <span>Developed by</span>
                            <a
                                class="ec-footer-link"
                                href="https://www.jpano.dev"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                Jhon Anthony Catamco Pano
                            </a>
                        </div>

                        <nav class="ec-footer-socials" aria-label="Developer links">
                            ${socialLink("facebook", "Facebook", "https://www.facebook.com/2.neeey")}
                            ${socialLink("github", "GitHub", "https://github.com/Dev-JPano")}
                            ${socialLink("linkedin", "LinkedIn", "https://www.linkedin.com/in/jhon-anthony-pano-830242335/")}
                            ${socialLink("contact", "Call +63 993 885 9567", "tel:+639938859567", { external: false })}
                            ${socialLink("email", "Email jacpano@gmail.com", "mailto:jacpano@gmail.com", { external: false })}
                            ${socialLink("telegram", "Telegram", "https://t.me/jacpano")}
                            ${socialLink("whatsapp", "WhatsApp", "https://wa.me/qr/FVFGZVOMUZJTA1")}
                        </nav>
                    </div>
                </div>
            </footer>
        `;
    }

    return {
        render
    };
})();

window.Footer = Footer;

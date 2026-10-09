package mailer

import (
	"context"
	"fmt"
	stdmail "net/mail"
	"strings"
)

func (m *Mailer) SendCardConnected(ctx context.Context, to, tenantName string, cardID int64, uid string) error {
	if !m.Configured() {
		return ErrNotConfigured
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("recipient email is required")
	}
	return m.send(ctx, to, m.buildCardConnectedMessage(to, tenantName, cardID, uid))
}

func (m *Mailer) buildCardConnectedMessage(to, tenantName string, cardID int64, uid string) []byte {
	from := (&stdmail.Address{Name: m.FromName, Address: m.Email}).String()
	subject := "Your ElectriCredit RFID card is ready"
	body := fmt.Sprintf(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light dark"><meta name="supported-color-schemes" content="light dark"><title>%s</title><style>
:root{color-scheme:light dark}.body{margin:0!important;padding:0!important;background:#f3f5f7;color:#172033;font-family:Arial,Helvetica,sans-serif;-webkit-text-size-adjust:100%%}.wrap{width:100%%;padding:12px 8px}.card{width:100%%;max-width:600px;background:#fff;border:1px solid #e1e6eb;border-radius:18px;overflow:hidden}.pad{padding:20px 18px}.title{font-size:23px;line-height:1.25;font-weight:800;margin:8px 0}.detail{display:block;margin:10px 0;padding:14px;border-radius:12px;background:#f7f8fa;border:1px solid #e6e9ee}.detail b{font-size:18px}.muted{font-size:13px;line-height:1.6;color:#657184}@media(min-width:520px){.wrap{padding:28px 16px}.pad{padding:28px 32px}.title{font-size:28px}}@media(prefers-color-scheme:dark){.body{background:#0f1318!important;color:#eef2f6!important}.card{background:#171c22!important;border-color:#303844!important}.detail{background:#11161c!important;border-color:#303844!important}.muted{color:#aab4c0!important}}</style></head><body class="body"><table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0"><tr><td align="center" class="wrap"><table role="presentation" class="card" cellspacing="0" cellpadding="0" border="0"><tr><td class="pad" style="background:#161b22;color:#fff;border-top:5px solid #f2b705"><div style="font-size:12px;font-weight:800;letter-spacing:.13em;color:#f2c94c">⚡ ELECTRICREDIT · RFID CARD</div><div class="title">Your electricity card is connected</div><div style="font-size:14px;line-height:1.65;color:#d9dee6">Hello %s, your RFID card was successfully registered to your ElectriCredit account.</div></td></tr><tr><td class="pad"><span class="detail"><span class="muted">Card ID</span><br><b>#%d</b></span><span class="detail"><span class="muted">RFID UID</span><br><b>%s</b></span><p class="muted">Keep the physical card secure. If you did not expect this registration, contact your property administrator.</p></td></tr></table><div class="muted" style="max-width:600px;padding:14px 8px 0;text-align:center">ElectriCredit · Capstone Project · Group 45</div></td></tr></table></body></html>`, subject, htmlSafe(tenantName), cardID, htmlSafe(uid))
	headers := []string{"From: " + from, "To: " + to, "Subject: " + subject, "MIME-Version: 1.0", "Content-Type: text/html; charset=UTF-8", "Content-Transfer-Encoding: 8bit"}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body)
}

func htmlSafe(v string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(v)
}

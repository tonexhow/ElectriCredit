package mailer

import (
	"fmt"
	"html"
	stdmail "net/mail"
	"strings"
)

func (m *Mailer) buildNotificationMessage(to, subject, message, preset string, manualHTML bool) []byte {
	from := (&stdmail.Address{Name: m.FromName, Address: m.Email}).String()
	contentType := "text/plain; charset=UTF-8"
	body := message

	if manualHTML {
		contentType = "text/html; charset=UTF-8"
	} else if preset != "none" && preset != "plain" {
		contentType = "text/html; charset=UTF-8"
		body = buildPresetNotificationHTML(subject, message, preset)
	}

	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + sanitizeHeader(subject),
		"MIME-Version: 1.0",
		"Content-Type: " + contentType,
		"Content-Transfer-Encoding: 8bit",
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body)
}

func buildPresetNotificationHTML(subject, message, preset string) string {
	accent := "#f2b705"
	label := "NOTICE"
	intro := "An update from ElectriCredit"

	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "alert":
		accent = "#c53030"
		label = "IMPORTANT"
		intro = "An important ElectriCredit notice"
	case "success":
		accent = "#2f855a"
		label = "UPDATE"
		intro = "An ElectriCredit update"
	case "info":
		accent = "#0f6e8c"
		label = "INFORMATION"
		intro = "Information from ElectriCredit"
	case "announcement":
		label = "ANNOUNCEMENT"
	}

	safeSubject := html.EscapeString(subject)
	safeMessage := html.EscapeString(message)
	safeMessage = strings.ReplaceAll(safeMessage, "\n", "<br>")

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1">
  <meta name="color-scheme" content="light dark">
  <meta name="supported-color-schemes" content="light dark">
  <title>%s</title>
  <style>
    :root{color-scheme:light dark}
    body{margin:0!important;padding:0!important;background:#f4f6f8;font-family:Arial,Helvetica,sans-serif;color:#172033;-webkit-text-size-adjust:100%%}
    table{border-collapse:separate}
    .wrap{width:100%%;padding:14px 10px}
    .card{width:100%%;max-width:600px;background:#fff;border:1px solid #e2e7ec;border-radius:18px;overflow:hidden}
    .pad{padding:22px 20px}
    .title{font-size:24px;line-height:1.25;font-weight:800;margin:8px 0 0}
    .copy{font-size:15px;line-height:1.7;color:#4f5b6d}
    @media(min-width:520px){.wrap{padding:28px 16px}.pad{padding:28px 32px}.title{font-size:28px}}
      @media(prefers-color-scheme:dark){body{background:#0f1318!important;color:#eef2f6!important}.card{background:#171c22!important;border-color:#303844!important}.copy{color:#b7c0cc!important}}
  </style>
</head>
<body>
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0">
    <tr><td align="center" class="wrap">
      <table role="presentation" class="card" cellspacing="0" cellpadding="0" border="0">
        <tr>
          <td class="pad" style="background:#161b22;color:#fff;border-top:5px solid %s">
            <div style="font-size:12px;font-weight:800;letter-spacing:.14em;color:%s">⚡ ELECTRICREDIT · %s</div>
            <div class="title">%s</div>
            <div style="font-size:13px;line-height:1.6;color:#d9dee6;margin-top:8px">%s</div>
          </td>
        </tr>
        <tr>
          <td class="pad">
            <div class="copy">%s</div>
            <div style="margin-top:22px;padding:14px 16px;border-radius:12px;background:#f7f8fa;border-left:4px solid %s;font-size:12px;line-height:1.6;color:#697586">
              This message was sent by ElectriCredit.
            </div>
          </td>
        </tr>
      </table>
      <div style="max-width:600px;padding:14px 8px 0;text-align:center;font-size:11px;line-height:1.5;color:#8d97a5">ElectriCredit · Capstone Project · Group 45</div>
    </td></tr>
  </table>
</body>
</html>`, safeSubject, accent, accent, label, safeSubject, intro, safeMessage, accent)
}

func sanitizeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

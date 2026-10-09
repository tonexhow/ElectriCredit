package mailer

import (
	"context"
	"fmt"
	stdmail "net/mail"
	"strings"
	"time"
)

func (m *Mailer) SendEmailChangeOTP(ctx context.Context, to, otp string, expiresAt time.Time) error {
	if !m.Configured() {
		return ErrNotConfigured
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("recipient email is required")
	}
	return m.send(ctx, to, m.buildEmailChangeOTPMessage(to, otp, expiresAt))
}

func (m *Mailer) buildEmailChangeOTPMessage(to, otp string, expiresAt time.Time) []byte {
	from := (&stdmail.Address{Name: m.FromName, Address: m.Email}).String()
	subject := "Verify your ElectriCredit email"
	expires := expiresAt.Local().Format("3:04 PM")

	body := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1">
  <meta name="color-scheme" content="light dark">
  <meta name="supported-color-schemes" content="light dark">
  <title>Verify your ElectriCredit email</title>
  <style>
    :root{color-scheme:light dark}
    body{margin:0!important;padding:0!important;background:#f4f6f8;font-family:Arial,Helvetica,sans-serif;color:#172033;-webkit-text-size-adjust:100%%}
    table{border-collapse:separate}.wrap{width:100%%;padding:12px 8px}.card{width:100%%;max-width:600px;background:#fff;border:1px solid #e2e7ec;border-radius:18px;overflow:hidden}.pad{padding:20px 18px}.title{font-size:23px;line-height:1.25;font-weight:800}.otp{font-size:30px;letter-spacing:.22em;padding:18px 8px}
    @media(min-width:520px){.wrap{padding:28px 16px}.pad{padding:28px 32px}.title{font-size:28px}.otp{font-size:38px;letter-spacing:.3em;padding:22px 12px}}
      @media(prefers-color-scheme:dark){body{background:#0f1318!important;color:#eef2f6!important}.card{background:#171c22!important;border-color:#303844!important}.copy{color:#b7c0cc!important}}
  </style>
</head>
<body>
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0"><tr><td align="center" class="wrap">
  <table role="presentation" class="card" cellspacing="0" cellpadding="0" border="0">
    <tr><td class="pad" style="background:#161b22;color:#fff;border-top:5px solid #f2b705">
      <div style="font-size:12px;font-weight:800;letter-spacing:.14em;color:#f2c94c">⚡ ELECTRICREDIT · EMAIL VERIFICATION</div>
      <div class="title" style="margin-top:10px">Confirm your new email</div>
      <div style="font-size:14px;line-height:1.65;margin-top:8px;color:#d9dee6">Use the code below to confirm that this email address belongs to you.</div>
    </td></tr>
    <tr><td class="pad">
      <div style="font-size:12px;font-weight:800;color:#687386;text-transform:uppercase;letter-spacing:.09em">Verification code</div>
      <div class="otp" style="margin:14px 0 18px;text-align:center;border:1px solid #ead994;border-radius:14px;background:#fff9df;color:#15191f;font-weight:800;white-space:nowrap">%s</div>
      <div style="font-size:14px;line-height:1.7;color:#5c6678">The code is valid for <strong style="color:#172033">15 minutes</strong>, until about <strong style="color:#172033">%s</strong>.</div>
      <div style="margin-top:18px;padding:14px 16px;border-radius:12px;background:#f7f8fa;font-size:12px;line-height:1.6;color:#677284">If you did not request this email change, you can safely ignore this message. Your account email will stay unchanged.</div>
    </td></tr>
  </table>
  <div style="max-width:600px;padding:14px 8px 0;text-align:center;font-size:11px;line-height:1.5;color:#98a1ae">ElectriCredit · Capstone Project · Group 45</div>
</td></tr></table>
</body></html>`, otp, expires)

	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body)
}

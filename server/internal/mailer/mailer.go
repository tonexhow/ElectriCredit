package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("SMTP mailer is not configured")

type Mailer struct {
	Host     string
	Port     int
	Email    string
	Password string
	FromName string
}

func NewFromEnv() *Mailer {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		host = "smtp.gmail.com"
	}

	port := 587
	if value := strings.TrimSpace(os.Getenv("SMTP_PORT")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 65535 {
			port = parsed
		}
	}

	fromName := strings.TrimSpace(os.Getenv("SMTP_FROM_NAME"))
	if fromName == "" {
		fromName = "ElectriCredit"
	}

	return &Mailer{
		Host:     host,
		Port:     port,
		Email:    strings.TrimSpace(os.Getenv("SMTP_EMAIL")),
		Password: strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
		FromName: fromName,
	}
}

func (m *Mailer) Configured() bool {
	return m != nil &&
		strings.TrimSpace(m.Host) != "" &&
		m.Port > 0 &&
		strings.TrimSpace(m.Email) != "" &&
		strings.TrimSpace(m.Password) != ""
}

func (m *Mailer) Available(ctx context.Context) error {
	if !m.Configured() {
		return ErrNotConfigured
	}

	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(
		ctx,
		"tcp",
		m.address(),
	)
	if err != nil {
		return fmt.Errorf("reach SMTP service: %w", err)
	}

	return conn.Close()
}

func (m *Mailer) SendRecoveryOTP(
	ctx context.Context,
	to string,
	otp string,
	expiresAt time.Time,
) error {
	if !m.Configured() {
		return ErrNotConfigured
	}

	to = strings.TrimSpace(to)
	if to == "" {
		return errors.New("recipient email is required")
	}

	message := m.buildRecoveryOTPMessage(to, otp, expiresAt)
	return m.send(ctx, to, message)
}

func (m *Mailer) send(ctx context.Context, recipient string, message []byte) error {
	conn, err := (&net.Dialer{Timeout: 8 * time.Second}).DialContext(
		ctx,
		"tcp",
		m.address(),
	)
	if err != nil {
		return fmt.Errorf("connect to SMTP service: %w", err)
	}

	client, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("start SMTP client: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); !ok {
		return errors.New("SMTP server does not support STARTTLS")
	}

	if err := client.StartTLS(&tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: m.Host,
	}); err != nil {
		return fmt.Errorf("secure SMTP connection: %w", err)
	}

	auth := smtp.PlainAuth("", m.Email, m.Password, m.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("authenticate SMTP account: %w", err)
	}

	if err := client.Mail(m.Email); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}

	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}

	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}

	if err := client.Quit(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("finish SMTP session: %w", err)
	}

	return nil
}

func (m *Mailer) address() string {
	return net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
}

// SendResult reports how many individual SMTP deliveries succeeded or failed.
type SendResult struct {
	Sent   int
	Failed int
}

// SendNotification sends one message per recipient so addresses are never
// exposed to other recipients. Presets build responsive ElectriCredit HTML;
// manual mode sends the supplied content exactly as HTML or plain text.
func (m *Mailer) SendNotification(
	ctx context.Context,
	recipients []string,
	subject string,
	message string,
	preset string,
	manualHTML bool,
) (SendResult, error) {
	if !m.Configured() {
		return SendResult{}, ErrNotConfigured
	}

	subject = strings.TrimSpace(subject)
	message = strings.TrimSpace(message)
	if subject == "" || message == "" {
		return SendResult{}, errors.New("subject and message are required")
	}

	result := SendResult{}
	var lastErr error
	for _, recipient := range recipients {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}

		payload := m.buildNotificationMessage(recipient, subject, message, preset, manualHTML)
		if err := m.send(ctx, recipient, payload); err != nil {
			result.Failed++
			lastErr = err
			continue
		}
		result.Sent++
	}

	if result.Sent == 0 && lastErr != nil {
		return result, lastErr
	}
	return result, nil
}

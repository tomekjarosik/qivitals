package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
)

type Sender interface {
	Send(ctx context.Context, to, subject, textBody, htmlBody string) error
}

// SMTPSender connects directly to an SMTP server (e.g., AWS SES)
type SMTPSender struct {
	From     string
	HostPort string // e.g., "email-smtp.eu-central-1.amazonaws.com:587"
	Username string
	Password string
}

func NewSMTPSender(from, hostPort, username, password string) *SMTPSender {
	return &SMTPSender{
		From:     from,
		HostPort: hostPort,
		Username: username,
		Password: password,
	}
}

func (s *SMTPSender) Send(ctx context.Context, to, subject, textBody, htmlBody string) error {
	// Safely split the host and port
	host, _, err := net.SplitHostPort(s.HostPort)
	if err != nil {
		return fmt.Errorf("invalid host format: %w", err)
	}

	var buf bytes.Buffer

	// Build MIME headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", s.From))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")

	writer := multipart.NewWriter(&buf)
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", writer.Boundary()))

	// Text part
	textHeaders := map[string][]string{
		"Content-Type":              {"text/plain; charset=utf-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	}
	textPart, _ := writer.CreatePart(textHeaders)
	qw := quotedprintable.NewWriter(textPart)
	qw.Write([]byte(textBody))
	qw.Close()

	// HTML part
	htmlHeaders := map[string][]string{
		"Content-Type":              {"text/html; charset=utf-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	}
	htmlPart, _ := writer.CreatePart(htmlHeaders)
	qw = quotedprintable.NewWriter(htmlPart)
	qw.Write([]byte(htmlBody))
	qw.Close()

	writer.Close()

	// Dial the connection respecting the Context timeout
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", s.HostPort)
	if err != nil {
		return fmt.Errorf("tcp dial failed: %w", err)
	}
	defer conn.Close()

	// Create the SMTP client
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client creation failed: %w", err)
	}
	defer client.Quit()

	// Upgrade to TLS
	if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
		return fmt.Errorf("smtp starttls failed: %w", err)
	}

	// Authenticate using just the host (not host:port)
	auth := smtp.PlainAuth("", s.Username, s.Password, host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth failed: %w", err)
	}

	// Send the email data
	if err := client.Mail(s.From); err != nil {
		return fmt.Errorf("smtp mail failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt failed: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data failed: %w", err)
	}
	if _, err = wc.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err = wc.Close(); err != nil {
		return fmt.Errorf("smtp close failed: %w", err)
	}

	return nil
}

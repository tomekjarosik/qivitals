package email

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"mime/quotedprintable"
	"os/exec"
	"strings"
)

// Sender defines the contract for sending emails.
type Sender interface {
	Send(ctx context.Context, to, subject, textBody, htmlBody string) error
}

// SystemSendmailSender uses the OS-level `sendmail` binary.
type SystemSendmailSender struct {
	From         string
	SendmailPath string
}

// NewSystemSender creates a sender that uses the system's sendmail binary.
func NewSystemSender(from string) *SystemSendmailSender {
	return &SystemSendmailSender{
		From:         from,
		SendmailPath: "sendmail",
	}
}

func (s *SystemSendmailSender) Send(ctx context.Context, to, subject, textBody, htmlBody string) error {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("From: %s\r\n", s.From))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")

	writer := multipart.NewWriter(&buf)
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", writer.Boundary()))

	textHeaders := map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}, "Content-Transfer-Encoding": {"quoted-printable"}}
	textPart, _ := writer.CreatePart(textHeaders)
	qw := quotedprintable.NewWriter(textPart)
	qw.Write([]byte(textBody))
	qw.Close()

	htmlHeaders := map[string][]string{"Content-Type": {"text/html; charset=utf-8"}, "Content-Transfer-Encoding": {"quoted-printable"}}
	htmlPart, _ := writer.CreatePart(htmlHeaders)
	qw = quotedprintable.NewWriter(htmlPart)
	qw.Write([]byte(htmlBody))
	qw.Close()

	writer.Close()

	// -t: Extract recipients from headers
	// -oi: Do not treat '.' on a line by itself as end of message
	cmd := exec.CommandContext(ctx, s.SendmailPath, "-t", "-oi")
	cmd.Stdin = &buf

	// Capture stderr for better error logging
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("system sendmail failed: %w, stderr: %s", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

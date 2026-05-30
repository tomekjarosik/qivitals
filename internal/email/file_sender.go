package email

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// EmailRecord represents a single email sent via the FileEmailSender.
type EmailRecord struct {
	To       string `json:"to"`
	Subject  string `json:"subject"`
	TextBody string `json:"text_body"`
	HTMLBody string `json:"html_body"`
}

// FileEmailSender writes emails to a file in JSONL format.
// Perfect for E2E testing as it avoids external SMTP dependencies.
type FileEmailSender struct {
	FilePath string
	mu       sync.Mutex
}

func NewFileEmailSender(filePath string) *FileEmailSender {
	return &FileEmailSender{FilePath: filePath}
}

func (s *FileEmailSender) Send(ctx context.Context, to, subject, textBody, htmlBody string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open email file: %w", err)
	}
	defer f.Close()

	record := EmailRecord{
		To:       to,
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}

	if err := json.NewEncoder(f).Encode(record); err != nil {
		return fmt.Errorf("write email record: %w", err)
	}

	return nil
}

package server

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/url"

	"github.com/tomekjarosik/qivitals/internal/auth"
	"github.com/tomekjarosik/qivitals/internal/email"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1 "github.com/tomekjarosik/qivitals/gen/api/qivitals/v1"
)

// MagicLinkSvcConfig holds application-level configuration for the server
type MagicLinkSvcConfig struct {
	AppBaseURL string // e.g., "https://app.qivitals.com"
	AppName    string // e.g., "Qivitals"
	FromEmail  string // e.g., "noreply@qivitals.com"
}

type magicLinkServer struct {
	v1.UnimplementedMagicLinkServiceServer
	authenticator *auth.Authenticator
	emailSender   email.Sender
	cfg           MagicLinkConfig
}

func NewMagicLinkServer(cfg MagicLinkConfig, authenticator *auth.Authenticator, emailSender email.Sender) v1.MagicLinkServiceServer {
	return &magicLinkServer{
		cfg:           cfg,
		authenticator: authenticator,
		emailSender:   emailSender,
	}
}

// --- Email Templates ---

const htmlTmplStr = `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .button { display: inline-block; padding: 12px 24px; background-color: #0052cc; color: #ffffff !important; text-decoration: none; border-radius: 6px; font-weight: bold; }
        .footer { margin-top: 40px; font-size: 12px; color: #666; border-top: 1px solid #eee; padding-top: 20px; }
    </style>
</head>
<body>
    <h2>Log in to {{.AppName}}</h2>
    <p>Click the button below to securely log in to your account. This link is valid for <strong>15 minutes</strong>.</p>
    <p style="text-align: center; margin: 30px 0;">
        <a href="{{.LoginURL}}" class="button">Log In to {{.AppName}}</a>
    </p>
    <p>If the button doesn't work, copy and paste this link into your browser:</p>
    <p><a href="{{.LoginURL}}">{{.LoginURL}}</a></p>
    <div class="footer">
        <p>If you didn't request this login link, you can safely ignore this email. Your account remains secure.</p>
    </div>
</body>
</html>
`

const textTmplStr = `Log in to {{.AppName}}

Click the link below to securely log in to your account. This link is valid for 15 minutes.

{{.LoginURL}}

If you didn't request this login link, you can safely ignore this email.
`

var (
	htmlTmpl = template.Must(template.New("html").Parse(htmlTmplStr))
	textTmpl = template.Must(template.New("text").Parse(textTmplStr))
)

type emailData struct {
	AppName  string
	LoginURL string
}

func (s *magicLinkServer) SendMagicLink(ctx context.Context, req *v1.SendMagicLinkRequest) (*v1.SendMagicLinkResponse, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	// Security: Prevent User Enumeration
	if !s.authenticator.IsEmailKnown(req.Email) {
		slog.Info("magic link requested for unknown email", "email", req.Email)
		// Return success to prevent attackers from mapping valid user emails
		return &v1.SendMagicLinkResponse{Sent: true}, nil
	}

	tokenStr, err := s.authenticator.GenerateMagicLink(req.Email)
	if err != nil {
		slog.Error("failed to generate magic link", "error", err, "email", req.Email)
		return nil, status.Error(codes.Internal, "failed to process request")
	}

	baseURL := s.cfg.AppBaseURL
	loginURL := fmt.Sprintf("https://%s/auth/verify?token=%s", baseURL, url.QueryEscape(tokenStr))

	// Render Templates
	data := emailData{
		AppName:  s.cfg.AppName,
		LoginURL: loginURL,
	}

	var htmlBody, textBody bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBody, data); err != nil {
		slog.Error("failed to render html template", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}
	if err := textTmpl.Execute(&textBody, data); err != nil {
		slog.Error("failed to render text template", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	// Send Email
	subject := fmt.Sprintf("Your %s Login Link", s.cfg.AppName)
	err = s.emailSender.Send(ctx, req.Email, subject, textBody.String(), htmlBody.String())
	if err != nil {
		slog.Error("failed to send magic link email", "error", err, "email", req.Email)
		// The token is in the DB, but the user didn't get the email.
		return nil, status.Error(codes.Internal, "failed to send email")
	}

	slog.Info("magic link sent successfully", "email", req.Email)
	return &v1.SendMagicLinkResponse{Sent: true}, nil
}

func (s *magicLinkServer) ValidateMagicLink(ctx context.Context, req *v1.ValidateMagicLinkRequest) (*v1.ValidateMagicLinkResponse, error) {
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	claims, err := s.authenticator.ParseAndValidateMagicLink(req.Token)
	if err != nil {
		slog.Warn("magic link validation failed", "error", err)
		return nil, status.Error(codes.Unauthenticated, "invalid or expired magic link")
	}

	sessionToken, err := s.authenticator.IssueSessionToken(claims.Email)
	if err != nil {
		slog.Error("failed to issue session token", "error", err)
		return nil, status.Error(codes.Internal, "failed to issue session")
	}

	slog.Info("user authenticated via magic link", "email", claims.Email)
	return &v1.ValidateMagicLinkResponse{
		SessionToken: sessionToken,
	}, nil
}

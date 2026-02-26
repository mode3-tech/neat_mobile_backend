package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"path/filepath"
	"strings"
	"time"
)

type Service struct {
	host string
	port string
	user string
	pass string
}

type otpTemplateData struct {
	Subject string
	OTP     string
	Year    int
}

func NewService(host, port, user, pass string) *Service {
	return &Service{
		host: strings.TrimSpace(host),
		port: strings.TrimSpace(port),
		user: strings.TrimSpace(user),
		pass: pass,
	}
}

func ParseTemplate[T any](fileName string, data T) (string, error) {
	templatePath := filepath.Join("templates", fileName+".html")

	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("template parsing error: %w", err)
	}

	var body bytes.Buffer
	if err := tpl.Execute(&body, data); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return body.String(), nil
}

func (s *Service) Send(ctx context.Context, to string, subject string, body string) error {
	_ = ctx

	data := otpTemplateData{
		Subject: subject,
		OTP:     strings.TrimSpace(body),
		Year:    time.Now().UTC().Year(),
	}

	return s.SendMail(to, "otp_email", subject, data)
}

func (s *Service) SendMail(to, templateName, subject string, data any, cc ...string) error {
	if s == nil {
		return fmt.Errorf("email service is nil")
	}

	if s.host == "" || s.port == "" || s.user == "" || s.pass == "" {
		return fmt.Errorf("incomplete SMTP configuration")
	}

	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("recipient email is required")
	}

	body, err := ParseTemplate(templateName, data)
	if err != nil {
		return fmt.Errorf("template processing failed: %w", err)
	}

	ccHeader := ""
	var recipients []string
	recipients = append(recipients, to)

	if len(cc) > 0 {
		trimmed := make([]string, 0, len(cc))
		for _, addr := range cc {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				continue
			}
			trimmed = append(trimmed, addr)
			recipients = append(recipients, addr)
		}
		if len(trimmed) > 0 {
			ccHeader = "Cc: " + strings.Join(trimmed, ", ") + "\r\n"
		}
	}

	cleanSubject := strings.ReplaceAll(subject, "\r", "")
	cleanSubject = strings.ReplaceAll(cleanSubject, "\n", "")

	msg := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			ccHeader+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n\r\n"+
			"%s",
		s.user,
		to,
		cleanSubject,
		body,
	)

	tlsConfig := &tls.Config{
		ServerName: s.host,
		MinVersion: tls.VersionTLS12,
	}

	client, err := s.newSMTPClient(tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	auth := smtp.PlainAuth("", s.user, s.pass, s.host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if err := client.Mail(s.user); err != nil {
		return fmt.Errorf("sender set failed: %w", err)
	}

	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("recipient set failed for %s: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data command failed: %w", err)
	}

	if _, err := w.Write([]byte(msg)); err != nil {
		_ = w.Close()
		return fmt.Errorf("message write failed: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("message finalize failed: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("SMTP quit failed: %w", err)
	}

	return nil
}

func (s *Service) newSMTPClient(tlsConfig *tls.Config) (*smtp.Client, error) {
	addr := s.host + ":" + s.port

	if s.port == "465" {
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("TLS connection failed: %w", err)
		}

		client, err := smtp.NewClient(conn, s.host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("SMTP client creation failed: %w", err)
		}

		return client, nil
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return nil, fmt.Errorf("SMTP connection failed: %w", err)
	}

	if ok, _ := client.Extension("STARTTLS"); !ok {
		_ = client.Close()
		return nil, fmt.Errorf("SMTP server does not support STARTTLS on port %s", s.port)
	}

	if err := client.StartTLS(tlsConfig); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("STARTTLS failed: %w", err)
	}

	return client, nil
}

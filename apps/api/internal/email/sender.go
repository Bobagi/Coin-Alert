// Package email sends transactional emails (password reset, email verification). It is intentionally
// integrated into the API rather than a separate service: volume is low and the messages are
// triggered synchronously by user actions. The Sender interface keeps the transport swappable (a
// hosted provider could replace SMTP later without touching the callers).
package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"os"
	"time"
)

// Message is a single email to one recipient, with both a plain-text and an HTML body.
type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// Sender delivers an email message.
type Sender interface {
	Send(sendContext context.Context, message Message) error
	// Enabled reports whether a real transport is configured (vs. the no-op logger).
	Enabled() bool
}

// SMTPSender sends through an SMTP server (e.g. Gmail on smtp.gmail.com:587 with an App Password).
type SMTPSender struct {
	host        string
	port        string
	username    string
	password    string
	fromAddress string
	fromName    string
}

func (sender *SMTPSender) Enabled() bool { return true }

// noopSender is used when SMTP is not configured: it logs instead of sending, so the app still runs.
type noopSender struct{}

func (noopSender) Enabled() bool { return false }
func (noopSender) Send(_ context.Context, message Message) error {
	log.Printf("email (not sent — SMTP not configured): to=%s subject=%q", message.To, message.Subject)
	return nil
}

// NewSenderFromEnv builds an SMTPSender from SMTP_* env vars, or a no-op sender when they are unset.
func NewSenderFromEnv() Sender {
	host := os.Getenv("SMTP_HOST")
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	if host == "" || username == "" || password == "" {
		return noopSender{}
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	fromAddress := os.Getenv("SMTP_FROM")
	if fromAddress == "" {
		fromAddress = username
	}
	fromName := os.Getenv("SMTP_FROM_NAME")
	if fromName == "" {
		fromName = "Coin Hub"
	}
	log.Printf("Email sending is enabled (SMTP %s:%s as %s)", host, port, fromAddress)
	return &SMTPSender{host: host, port: port, username: username, password: password, fromAddress: fromAddress, fromName: fromName}
}

// Send delivers the message over SMTP with STARTTLS and PLAIN auth.
func (sender *SMTPSender) Send(sendContext context.Context, message Message) error {
	if message.To == "" {
		return errors.New("email recipient is empty")
	}

	address := net.JoinHostPort(sender.host, sender.port)
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	connection, dialError := dialer.DialContext(sendContext, "tcp", address)
	if dialError != nil {
		return dialError
	}

	client, clientError := smtp.NewClient(connection, sender.host)
	if clientError != nil {
		_ = connection.Close()
		return clientError
	}
	defer client.Close()

	if hasStartTLS, _ := client.Extension("STARTTLS"); hasStartTLS {
		if tlsError := client.StartTLS(&tls.Config{ServerName: sender.host}); tlsError != nil {
			return tlsError
		}
	} else {
		return errors.New("SMTP server does not support STARTTLS; refusing to send credentials in the clear")
	}

	authentication := smtp.PlainAuth("", sender.username, sender.password, sender.host)
	if authError := client.Auth(authentication); authError != nil {
		return fmt.Errorf("SMTP authentication failed: %w", authError)
	}
	if mailError := client.Mail(sender.fromAddress); mailError != nil {
		return mailError
	}
	if rcptError := client.Rcpt(message.To); rcptError != nil {
		return rcptError
	}
	writer, dataError := client.Data()
	if dataError != nil {
		return dataError
	}
	if _, writeError := writer.Write(sender.buildMIME(message)); writeError != nil {
		return writeError
	}
	if closeError := writer.Close(); closeError != nil {
		return closeError
	}
	return client.Quit()
}

func (sender *SMTPSender) buildMIME(message Message) []byte {
	var builder bytes.Buffer
	boundary := "coinhub-" + randomBoundary()

	writeHeader(&builder, "From", fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", sender.fromName), sender.fromAddress))
	writeHeader(&builder, "To", message.To)
	writeHeader(&builder, "Subject", mime.QEncoding.Encode("utf-8", message.Subject))
	writeHeader(&builder, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&builder, "MIME-Version", "1.0")
	writeHeader(&builder, "Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", boundary))
	builder.WriteString("\r\n")

	writeBodyPart(&builder, boundary, "text/plain; charset=\"utf-8\"", message.TextBody)
	writeBodyPart(&builder, boundary, "text/html; charset=\"utf-8\"", message.HTMLBody)
	builder.WriteString("--" + boundary + "--\r\n")

	return builder.Bytes()
}

func writeHeader(builder *bytes.Buffer, name string, value string) {
	builder.WriteString(name)
	builder.WriteString(": ")
	builder.WriteString(value)
	builder.WriteString("\r\n")
}

func writeBodyPart(builder *bytes.Buffer, boundary string, contentType string, body string) {
	builder.WriteString("--" + boundary + "\r\n")
	writeHeader(builder, "Content-Type", contentType)
	writeHeader(builder, "Content-Transfer-Encoding", "quoted-printable")
	builder.WriteString("\r\n")
	quotedWriter := quotedprintable.NewWriter(builder)
	_, _ = quotedWriter.Write([]byte(body))
	_ = quotedWriter.Close()
	builder.WriteString("\r\n")
}

func randomBoundary() string {
	raw := make([]byte, 12)
	if _, randomError := rand.Read(raw); randomError != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

package common

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"slices"
	"strings"
	"time"
)

func generateMessageID() (string, error) {
	split := strings.Split(SMTPFrom, "@")
	if len(split) < 2 {
		return "", fmt.Errorf("invalid SMTP account")
	}
	domain := strings.Split(SMTPFrom, "@")[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain), nil
}

func shouldUseSMTPLoginAuth() bool {
	if SMTPForceAuthLogin {
		return true
	}
	return isOutlookServer(SMTPAccount) || slices.Contains(EmailLoginAuthServerList, SMTPServer)
}

func getSMTPAuth() smtp.Auth {
	return AutoSMTPAuth(SMTPAccount, SMTPToken)
}

func shouldAuthenticateSMTP() bool {
	return SMTPAccount != "" && SMTPToken != ""
}

func smtpTLSConfig() *tls.Config {
	return &tls.Config{
		ServerName:         SMTPServer,
		InsecureSkipVerify: SMTPInsecureSkipVerify, // #nosec G402 -- admin-controlled SMTP compatibility option.
	}
}

func newSMTPClient(addr string) (*smtp.Client, error) {
	if SMTPSSLEnabled || (SMTPPort == 465 && !SMTPStartTLSEnabled) {
		conn, err := tls.Dial("tcp", addr, smtpTLSConfig())
		if err != nil {
			return nil, err
		}
		client, err := smtp.NewClient(conn, SMTPServer)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		return client, nil
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return nil, err
	}

	if SMTPStartTLSEnabled {
		startTLSSupported, _ := client.Extension("STARTTLS")
		if !startTLSSupported {
			_ = client.Close()
			return nil, fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(smtpTLSConfig()); err != nil {
			_ = client.Close()
			return nil, err
		}
	}

	return client, nil
}

// MailSender is a connection to the SMTP server of the installation.
//
// A campaign writes many messages to one connection: the providers rate limit
// the connections and not the messages, and every new connection of a burst is
// a reason for the provider to slow the sender down.
type MailSender struct {
	client     *smtp.Client
	sent       int
	maxPerConn int
}

// NewMailSender opens a connection to the SMTP server and authenticates.
func NewMailSender() (*MailSender, error) {
	if SMTPFrom == "" { // for compatibility
		SMTPFrom = SMTPAccount
	}
	if SMTPServer == "" && SMTPAccount == "" {
		return nil, fmt.Errorf("SMTP 服务器未配置")
	}
	addr := fmt.Sprintf("%s:%d", SMTPServer, SMTPPort)
	client, err := newSMTPClient(addr)
	if err != nil {
		return nil, err
	}
	if shouldAuthenticateSMTP() {
		if err := client.Auth(getSMTPAuth()); err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	return &MailSender{client: client, maxPerConn: 40}, nil
}

// Send writes one message.
func (s *MailSender) Send(message MailMessage) error {
	body, err := BuildMailMessage(message)
	if err != nil {
		return err
	}
	if s.client == nil {
		return fmt.Errorf("the connection to the mail server is closed")
	}
	from := SMTPFrom
	if from == "" {
		from = SMTPAccount
	}
	if err := s.client.Mail(from); err != nil {
		return err
	}
	if err := s.client.Rcpt(message.To); err != nil {
		return err
	}
	writer, err := s.client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	s.sent++
	return nil
}

// Exhausted reports whether the connection carried as many messages as it
// should. The providers close a connection that lives too long, the campaign
// opens a new one then.
func (s *MailSender) Exhausted() bool {
	return s.maxPerConn > 0 && s.sent >= s.maxPerConn
}

// Close ends the session.
func (s *MailSender) Close() {
	if s.client == nil {
		return
	}
	_ = s.client.Quit()
	_ = s.client.Close()
	s.client = nil
}

// SendEmail sends one message to one receiver.
//
// The text is written with the headers of the installation; the signature that
// the callers of the older version used is kept, the message is HTML.
func SendEmail(subject string, receiver string, content string) error {
	sender, err := NewMailSender()
	if err != nil {
		return err
	}
	defer sender.Close()
	return sender.Send(MailMessage{To: receiver, Subject: subject, HTML: content})
}

// SendMailMessage sends one message with the headers that the caller needs.
func SendMailMessage(message MailMessage) error {
	sender, err := NewMailSender()
	if err != nil {
		return err
	}
	defer sender.Close()
	return sender.Send(message)
}

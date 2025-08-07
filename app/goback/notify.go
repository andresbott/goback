package goback

import (
	"fmt"
	"github.com/AndresBott/goback/internal/profile"
	"net/smtp"
	"strings"
	"time"
)

func NotifySuccess(cfg profile.EmailNotify, prfName string) error {
	var s strings.Builder

	s.WriteString("Subject: Goback Success Notification\r\n")
	s.WriteString("\r\n") // Email body starts here
	s.WriteString(fmt.Sprintf("✅ Backup completed successfully.\n\n"))
	s.WriteString(fmt.Sprintf("Profile: %s\n", prfName))
	s.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format(time.RFC1123)))
	s.WriteString("\nEverything went as expected.\n")

	return send(cfg, []byte(s.String()))
}

func NotifyFailure(cfg profile.EmailNotify, prfName string, err error) error {
	var s strings.Builder

	s.WriteString("Subject: Goback Failure Notification\r\n")
	s.WriteString("\r\n") // Email body starts here
	s.WriteString(fmt.Sprintf("❌ Backup failed.\n\n"))
	s.WriteString(fmt.Sprintf("Profile: %s\n", prfName))
	s.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format(time.RFC1123)))
	s.WriteString(fmt.Sprintf("Error: %v\n", err))
	s.WriteString("\nPlease investigate the issue.\n")

	return send(cfg, []byte(s.String()))
}

func send(cfg profile.EmailNotify, body []byte) error {
	// Authentication.
	auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)

	// Construct the email headers and body.
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", cfg.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(cfg.To, ", ")))
	msg.WriteString("MIME-version: 1.0;\r\n")
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\";\r\n")
	msg.Write(body)

	// Send the email.
	err := smtp.SendMail(cfg.Host+":"+cfg.Port, auth, cfg.User, cfg.To, []byte(msg.String()))
	if err != nil {
		return fmt.Errorf("unable to send email: %v", err)
	}
	return nil
}

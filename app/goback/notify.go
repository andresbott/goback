package goback

import (
	"fmt"
	"git.andresbott.com/Golang/goback/internal/profile"
	"net/smtp"
	"strings"
)

func NotifySuccess(cfg profile.EmailNotify) error {

	var s strings.Builder
	s.WriteString("Goback success")

	return send(cfg, []byte(s.String()))
}
func NotifyFailure(cfg profile.EmailNotify, err error) error {

	var s strings.Builder
	s.WriteString(fmt.Sprintf("Goback failed: %v", err))

	return send(cfg, []byte(s.String()))
}

func send(cfg profile.EmailNotify, msg []byte) error {

	// Authentication.
	auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)

	// Sending email.
	err := smtp.SendMail(cfg.Host+":"+cfg.Port, auth, cfg.User, cfg.To, msg)
	if err != nil {
		return fmt.Errorf("unable to send email: %v", err)
	}
	return nil
}

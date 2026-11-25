package main

import (
	"fmt"
	"net/smtp"
	"strings"
)

type EmailSender interface {
	Send(to, subject, body string) error
}

type MailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	AppURL   string
}

type SMTPEmailSender struct{ config MailConfig }

func NewSMTPEmailSender(config MailConfig) *SMTPEmailSender {
	return &SMTPEmailSender{config: config}
}

func (s *SMTPEmailSender) Send(to, subject, body string) error {
	address := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	var auth smtp.Auth
	if s.config.Username != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}
	message := strings.Join([]string{
		"From: " + s.config.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	return smtp.SendMail(address, auth, s.config.From, []string{to}, []byte(message))
}

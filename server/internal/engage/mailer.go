package engage

import (
	"fmt"
	"net/smtp"
	"strings"
)

// Mailer sends plain notification emails. With no host set, email is off.
type Mailer struct {
	Host, Port, User, Pass, From string
}

func (m Mailer) Enabled() bool { return m.Host != "" && m.From != "" }

func clean(s string) string { return strings.NewReplacer("\r", " ", "\n", " ").Replace(s) }

// Message builds the email, keeping line breaks out of the headers.
func (m Mailer) Message(to, subject, body string) []byte {
	return []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n",
		clean(m.From), clean(to), clean(subject), body))
}

func (m Mailer) Send(to, subject, body string) error {
	var auth smtp.Auth
	if m.User != "" {
		auth = smtp.PlainAuth("", m.User, m.Pass, m.Host)
	}
	return smtp.SendMail(m.Host+":"+m.Port, auth, m.From, []string{clean(to)}, m.Message(to, subject, body))
}

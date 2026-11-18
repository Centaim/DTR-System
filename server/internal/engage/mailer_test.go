package engage

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

func TestMessageBlocksHeaderInjection(t *testing.T) {
	m := Mailer{From: "dtr@example.test"}
	got := string(m.Message("a@example.test\r\nBcc: x@evil.test", "Hi\nBcc: y@evil.test", "body"))
	head, _, _ := strings.Cut(got, "\r\n\r\n")
	for _, l := range strings.Split(head, "\r\n") {
		if strings.HasPrefix(l, "Bcc:") {
			t.Fatalf("injected header: %q", l)
		}
	}
}

func TestSendTalksSMTP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		w := func(s string) { _, _ = c.Write([]byte(s + "\r\n")) }
		w("220 test")
		var data strings.Builder
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if strings.TrimSpace(line) == "." {
					inData = false
					got <- data.String()
					w("250 ok")
					continue
				}
				data.WriteString(line)
				continue
			}
			switch strings.ToUpper(strings.Fields(line)[0]) {
			case "DATA":
				inData = true
				w("354 go")
			case "QUIT":
				w("221 bye")
				return
			default:
				w("250 ok")
			}
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	m := Mailer{Host: "127.0.0.1", Port: port, From: "dtr@example.test"}
	if err := m.Send("emp@example.test", "Request approved", "Your request was approved."); err != nil {
		t.Fatal(err)
	}
	if msg := <-got; !strings.Contains(msg, "Subject: Request approved") || !strings.Contains(msg, "Your request was approved.") {
		t.Fatalf("message: %q", msg)
	}
}

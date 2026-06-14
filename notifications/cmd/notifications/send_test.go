package main

import (
	"bytes"
	"context"
	"io"
	"mime/quotedprintable"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

// fakeSMTPResult carries what the fake server captured during the DATA phase.
type fakeSMTPResult struct {
	data []byte
	err  error
}

// startFakeSMTPServer listens on a loopback port and speaks just enough SMTP to
// accept one message, returning the raw DATA payload (headers + body) over the
// channel after the client quits.
func startFakeSMTPServer(t *testing.T) (string, <-chan fakeSMTPResult) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake SMTP listener: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	resultCh := make(chan fakeSMTPResult, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			resultCh <- fakeSMTPResult{err: err}
			return
		}
		defer conn.Close()

		tc := textproto.NewConn(conn)
		_ = tc.PrintfLine("220 fake ESMTP")

		var captured []byte
		for {
			line, err := tc.ReadLine()
			if err != nil {
				resultCh <- fakeSMTPResult{err: err}
				return
			}
			cmd := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				_ = tc.PrintfLine("250 fake")
			case strings.HasPrefix(cmd, "MAIL FROM"):
				_ = tc.PrintfLine("250 OK")
			case strings.HasPrefix(cmd, "RCPT TO"):
				_ = tc.PrintfLine("250 OK")
			case strings.HasPrefix(cmd, "DATA"):
				_ = tc.PrintfLine("354 End data with <CR><LF>.<CR><LF>")
				// Read the payload raw (via the underlying buffered reader) to
				// preserve CRLF line endings; ReadDotBytes would normalize them
				// to LF and hide whether the wire lines respect the 998 limit.
				var b bytes.Buffer
				for {
					chunk, err := tc.R.ReadString('\n')
					if err != nil {
						resultCh <- fakeSMTPResult{err: err}
						return
					}
					if chunk == ".\r\n" {
						break
					}
					// Undo SMTP dot-stuffing on lines that begin with a dot.
					if strings.HasPrefix(chunk, ".") {
						chunk = chunk[1:]
					}
					b.WriteString(chunk)
				}
				captured = b.Bytes()
				_ = tc.PrintfLine("250 OK")
			case strings.HasPrefix(cmd, "QUIT"):
				_ = tc.PrintfLine("221 Bye")
				resultCh <- fakeSMTPResult{data: captured}
				return
			default:
				_ = tc.PrintfLine("250 OK")
			}
		}
	}()

	return ln.Addr().String(), resultCh
}

func TestSendEncodesBodyConsistentlyWithHeader(t *testing.T) {
	addr, resultCh := startFakeSMTPServer(t)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to split fake server addr: %v", err)
	}

	// A single long HTML line with Polish characters: forces quoted-printable
	// soft wrapping and =XX encoding so the round-trip and line-length checks
	// are meaningful.
	var bodyBuilder strings.Builder
	bodyBuilder.WriteString("<!doctype html><html><body><table>")
	for i := 0; i < 40; i++ {
		bodyBuilder.WriteString("<tr><td>Zaświadczenie ćwiczeń źdźbło ąęół</td></tr>")
	}
	bodyBuilder.WriteString("</table></body></html>")
	body := bodyBuilder.String()

	fixedNow := time.Date(2026, time.June, 13, 9, 30, 0, 0, time.UTC)
	mailer := SMTPMailer{cfg: SMTPConfig{
		Host:     host,
		Port:     port,
		From:     "biuro@naszaera.pl",
		FromName: "Powiadomienia BHP",
		TLSMode:  "none",
		Timeout:  5 * time.Second,
	}}

	err = mailer.Send(context.Background(), func() time.Time { return fixedNow }, EmailMessage{
		To:      "klient@example.com",
		Subject: "Zaświadczenia wygasające",
		Body:    body,
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	var result fakeSMTPResult
	select {
	case result = <-resultCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for fake SMTP server")
	}
	if result.err != nil {
		t.Fatalf("fake SMTP server error: %v", result.err)
	}

	raw := result.data
	rawStr := string(raw)

	// Header declares quoted-printable and not the previous 8bit value.
	if !strings.Contains(rawStr, "Content-Transfer-Encoding: quoted-printable") {
		t.Fatal("expected Content-Transfer-Encoding: quoted-printable header")
	}
	if strings.Contains(rawStr, "Content-Transfer-Encoding: 8bit") {
		t.Fatal("8bit encoding header must not be present alongside quoted-printable body")
	}

	// Date and Message-ID headers are present and well formed.
	wantDate := "Date: " + fixedNow.Format(time.RFC1123Z)
	if !strings.Contains(rawStr, wantDate) {
		t.Fatalf("expected %q in message, got headers:\n%s", wantDate, rawStr)
	}
	if !strings.Contains(rawStr, "Message-ID: <") || !strings.Contains(rawStr, "@naszaera.pl>") {
		t.Fatal("expected Message-ID header with the sender domain")
	}

	// No line may exceed the RFC 5321 limit of 998 octets.
	for _, line := range bytes.Split(raw, []byte("\r\n")) {
		if len(line) > 998 {
			t.Fatalf("found line of %d octets, exceeds RFC 5321 limit of 998", len(line))
		}
	}

	// The quoted-printable body must decode back to the original HTML, proving
	// the declared encoding matches the actual encoding.
	sep := []byte("\r\n\r\n")
	idx := bytes.Index(raw, sep)
	if idx < 0 {
		t.Fatal("could not find header/body separator in message")
	}
	encodedBody := raw[idx+len(sep):]
	decoded, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(encodedBody)))
	if err != nil {
		t.Fatalf("failed to decode quoted-printable body: %v", err)
	}
	// SMTP DATA terminates the message with a CRLF before the trailing dot; that
	// transport-level newline is not part of the body we constructed.
	decodedBody := strings.TrimSuffix(string(decoded), "\r\n")
	if decodedBody != body {
		t.Fatalf("decoded body does not match original\n got: %q\nwant: %q", decodedBody, body)
	}
}

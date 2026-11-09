package web

import (
	"bufio"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/certs"
)

func TestTLSOnlyIsSilentForPlainHTTP(t *testing.T) {
	is, err := certs.SelfSigned("swift-test.invalid", nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(is.CertPEM, is.KeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler:   http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		ErrorLog:  log.New(io.Discard, "", 0),
	}
	go func() { _ = srv.ServeTLS(TLSOnly(ln), "", "") }()
	defer srv.Close()
	addr := ln.Addr().String()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	got, _ := io.ReadAll(bufio.NewReader(conn))
	conn.Close()
	if len(got) != 0 {
		t.Fatalf("plain HTTP got a reply: %q", got)
	}

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // self-signed test server
	resp, err := client.Get("https://" + addr)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "ok" {
		t.Fatalf("TLS request: %q", body)
	}
}

package repository

import (
	"net/http"
	"testing"
)

func TestClientIPNormalizesRemoteAddress(t *testing.T) {
	req := &http.Request{RemoteAddr: "127.0.0.1:64134", Header: http.Header{}}
	if got := ClientIP(req); got != "127.0.0.1" {
		t.Fatalf("ClientIP = %q, want 127.0.0.1", got)
	}
}

func TestClientIPUsesFirstForwardedAddress(t *testing.T) {
	req := &http.Request{RemoteAddr: "127.0.0.1:64134", Header: http.Header{}}
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	req.Header.Set("X-Real-IP", "198.51.100.8")
	if got := ClientIP(req); got != "203.0.113.10" {
		t.Fatalf("ClientIP = %q, want 203.0.113.10", got)
	}
}

func TestClientIPUsesRealIPWhenForwardedForMissing(t *testing.T) {
	req := &http.Request{RemoteAddr: "127.0.0.1:64134", Header: http.Header{}}
	req.Header.Set("X-Real-IP", "198.51.100.8")
	if got := ClientIP(req); got != "198.51.100.8" {
		t.Fatalf("ClientIP = %q, want 198.51.100.8", got)
	}
}

func TestClientIPNormalizesIPv6RemoteAddress(t *testing.T) {
	req := &http.Request{RemoteAddr: "[::1]:64134", Header: http.Header{}}
	if got := ClientIP(req); got != "::1" {
		t.Fatalf("ClientIP = %q, want ::1", got)
	}
}

func TestClientIPFallsBackToRemoteAddrWhenSplitFails(t *testing.T) {
	req := &http.Request{RemoteAddr: "unix-socket", Header: http.Header{}}
	if got := ClientIP(req); got != "unix-socket" {
		t.Fatalf("ClientIP = %q, want unix-socket", got)
	}
}

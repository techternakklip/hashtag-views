package cli

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIFetchReportAndExportWorkflow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "cli.db")
	exportDir := filepath.Join(tmp, "exports")
	t.Setenv("DB_PATH", dbPath)

	var stdout, stderr bytes.Buffer

	if err := Run([]string{"fetch", "--hashtag", "#GTA", "--platforms", "ig,fb", "--limit", "10", "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("fetch run 1 failed: %v (stderr=%s)", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "RINGKASAN TOTAL REAL VIEWS: #gta") {
		t.Fatalf("expected summary output, got: %s", stdout.String())
	}

	stdout.Reset()
	if err := Run([]string{"fetch", "--hashtag", "gta", "--platforms", "ig,fb", "--limit", "10", "--mock"}, &stdout, &stderr); err != nil {
		t.Fatalf("fetch run 2 failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Delta vs Run Lalu : +1.327.350 views (+4 video baru)") {
		t.Fatalf("expected formatted delta in run 2 output, got: %s", stdout.String())
	}

	stdout.Reset()
	if err := Run([]string{"report", "--hashtag", "gta"}, &stdout, &stderr); err != nil {
		t.Fatalf("report failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Top 10 Video berdasarkan Views:") {
		t.Fatalf("expected top 10 table in report, got: %s", stdout.String())
	}

	stdout.Reset()
	if err := Run([]string{"export", "--hashtag", "gta", "--format", "csv", "--out", exportDir}, &stdout, &stderr); err != nil {
		t.Fatalf("export csv failed: %v", err)
	}
	csvFile := filepath.Join(exportDir, "hashtag-gta-views.csv")
	info, err := os.Stat(csvFile)
	if err != nil || info.Size() == 0 {
		t.Fatalf("expected non-empty CSV file at %s, err=%v", csvFile, err)
	}
}

func TestFormatDots(t *testing.T) {
	if got := FormatDots(1327350); got != "1.327.350" {
		t.Fatalf("FormatDots(1327350) = %q, want 1.327.350", got)
	}
	if got := FormatDots(-5000); got != "-5.000" {
		t.Fatalf("FormatDots(-5000) = %q, want -5.000", got)
	}
	if got := FormatDots(42); got != "42" {
		t.Fatalf("FormatDots(42) = %q, want 42", got)
	}
}

func TestFormatListenAddrAndLoopbackBind(t *testing.T) {
	cases := []struct {
		host     string
		port     string
		wantAddr string
		wantURL  string
	}{
		{host: "127.0.0.1", port: "8095", wantAddr: "127.0.0.1:8095", wantURL: "http://127.0.0.1:8095"},
		{host: "0.0.0.0", port: "8080", wantAddr: "0.0.0.0:8080", wantURL: "http://0.0.0.0:8080"},
		{host: "::1", port: "8080", wantAddr: "[::1]:8080", wantURL: "http://[::1]:8080"},
		{host: "[::1]", port: "8080", wantAddr: "[::1]:8080", wantURL: "http://[::1]:8080"},
		{host: "", port: "8080", wantAddr: ":8080", wantURL: "http://localhost:8080"},
	}

	for _, tc := range cases {
		gotAddr := formatListenAddr(tc.host, tc.port)
		if gotAddr != tc.wantAddr {
			t.Fatalf("formatListenAddr(%q, %q) = %q, want %q", tc.host, tc.port, gotAddr, tc.wantAddr)
		}
		if gotURL := formatLogURL(gotAddr); gotURL != tc.wantURL {
			t.Fatalf("formatLogURL(%q) = %q, want %q", gotAddr, gotURL, tc.wantURL)
		}
	}

	ln, err := net.Listen("tcp", formatListenAddr("127.0.0.1", "0"))
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer ln.Close()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener addr type = %T, want *net.TCPAddr", ln.Addr())
	}
	if got := tcpAddr.IP.String(); got != "127.0.0.1" {
		t.Fatalf("bound IP = %q, want 127.0.0.1", got)
	}
}

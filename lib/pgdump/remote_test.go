package pgdump

import (
	"testing"
)

func TestWriteFromRemote(t *testing.T) {
	// This test would require a real SSH client, so we'll skip it for now
	// In a real implementation, you'd need to mock the SSH client or have a real connection
	t.Skip("SSH test requires real SSH client")
}

func TestNewRemote(t *testing.T) {
	cfg := RemoteCfg{
		BinPath: "/usr/bin/pg_dump",
		User:    "testuser",
		Pw:      "testpass",
		DbName:  "testdb",
	}

	h, err := NewRemote(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.binPath != "/usr/bin/pg_dump" {
		t.Errorf("expected bin path /usr/bin/pg_dump, got: %s", h.binPath)
	}

	if h.user != "testuser" {
		t.Errorf("expected user testuser, got: %s", h.user)
	}

	if h.pw != "testpass" {
		t.Errorf("expected pw testpass, got: %s", h.pw)
	}

	if h.dbName != "testdb" {
		t.Errorf("expected db name testdb, got: %s", h.dbName)
	}
}

func TestNewRemoteDefaultBinPath(t *testing.T) {
	cfg := RemoteCfg{
		User:   "testuser",
		Pw:     "testpass",
		DbName: "testdb",
	}

	h, err := NewRemote(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.binPath != "pg_dump" {
		t.Errorf("expected default bin path pg_dump, got: %s", h.binPath)
	}
}

func TestRemoteCmd(t *testing.T) {
	h := RemoteHandler{
		binPath: "/usr/bin/pg_dump",
		user:    "testuser",
		pw:      "testpass",
		dbName:  "testdb",
	}

	cmd := h.Cmd()
	expected := "/usr/bin/pg_dump -U testuser -W --clean --if-exists --create --verbose testdb"
	if cmd != expected {
		t.Errorf("expected command: %s, got: %s", expected, cmd)
	}
}

func TestRemoteCmdNoUser(t *testing.T) {
	h := RemoteHandler{
		binPath: "/usr/bin/pg_dump",
		user:    "",
		pw:      "",
		dbName:  "testdb",
	}

	cmd := h.Cmd()
	expected := "/usr/bin/pg_dump --clean --if-exists --create --verbose testdb"
	if cmd != expected {
		t.Errorf("expected command: %s, got: %s", expected, cmd)
	}
}

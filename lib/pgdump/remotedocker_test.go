package pgdump

import (
	"testing"
)

func TestWriteFromSshDocker(t *testing.T) {
	// This test would require a real SSH client, so we'll skip it for now
	// In a real implementation, you'd need to mock the SSH client or have a real connection
	t.Skip("SSH+Docker test requires real SSH client")
}

func TestNewSshDocker(t *testing.T) {
	cfg := SshDockerCfg{
		ContainerName: "test-container",
		BinPath:       "/usr/bin/pg_dump",
		User:          "testuser",
		Pw:            "testpass",
		DbName:        "testdb",
	}

	h, err := NewSshDocker(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.containerName != "test-container" {
		t.Errorf("expected container name test-container, got: %s", h.containerName)
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

func TestNewSshDockerDefaultBinPath(t *testing.T) {
	cfg := SshDockerCfg{
		ContainerName: "test-container",
		User:          "testuser",
		Pw:            "testpass",
		DbName:        "testdb",
	}

	h, err := NewSshDocker(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.binPath != "pg_dump" {
		t.Errorf("expected default bin path pg_dump, got: %s", h.binPath)
	}
}

func TestSshDockerCmd(t *testing.T) {
	h := SshDockerHandler{
		binPath:       "/usr/bin/pg_dump",
		containerName: "test-container",
		user:          "testuser",
		pw:            "testpass",
		dbName:        "testdb",
	}

	cmd := h.Cmd()
	expected := "/usr/bin/pg_dump -U testuser -W --clean --if-exists --create --verbose testdb"
	if cmd != expected {
		t.Errorf("expected command: %s, got: %s", expected, cmd)
	}
}

func TestSshDockerCmdNoUser(t *testing.T) {
	h := SshDockerHandler{
		binPath:       "/usr/bin/pg_dump",
		containerName: "test-container",
		user:          "",
		pw:            "",
		dbName:        "testdb",
	}

	cmd := h.Cmd()
	expected := "/usr/bin/pg_dump --clean --if-exists --create --verbose testdb"
	if cmd != expected {
		t.Errorf("expected command: %s, got: %s", expected, cmd)
	}
}

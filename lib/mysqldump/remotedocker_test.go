package mysqldump

import (
	"context"
	"strings"
	"testing"

	"github.com/AndresBott/goback/lib/ssh"
)

func TestGetSshDockerBinPathRequiresDocker(t *testing.T) {
	skipInCI(t) // skip test if running in CI

	ctx := context.Background()
	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = sshServer.Terminate(ctx)
	}()

	cl, err := ssh.New(ssh.Cfg{
		Host:          sshServer.host,
		Port:          sshServer.port,
		Auth:          ssh.Password,
		User:          "pwuser",
		Password:      "1234",
		IgnoreHostKey: true,
	})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	_ = cl.Connect()

	// Test that it fails when Docker is not available
	// Since the test container doesn't have Docker, this should fail
	_, err = GetSshDockerBinPath(cl, "mysql-container")
	if err == nil {
		t.Fatalf("expected error when Docker is not available, but got none")
	}

	// Check that the error message indicates Docker is required
	if !strings.Contains(err.Error(), "docker is not available") {
		t.Errorf("expected error to mention Docker not available, got: %v", err)
	}
}

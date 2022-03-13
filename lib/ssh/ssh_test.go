package ssh

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func skipIfCi(t *testing.T) {
	skip := os.Getenv("SKIP_CI_TEST")
	if skip == "true" {
		t.Skip("skipping because env \"SKIP_CI_TEST\"  is set to true")
	}
}

type sshContainer struct {
	testcontainers.Container
	host string
	port int
}

func setupContainer(ctx context.Context) (*sshContainer, error) {

	privKey, _ := os.ReadFile("./sampledata/public.key") // passphrase for private key is "pass"

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "./sampledata",
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{"22/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Server listening on 0.0.0.0 port 22"),
		),
		Env: map[string]string{
			"PW_USER":      "kTZ8GVSkARoNg", // user: pwuser pw: 1234
			"SHH_KEY_USER": string(privKey), // user privkey, private key:private.key passphrase: pass
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		return nil, err
	}

	//losg,_ := container.Logs(ctx)
	//lines,_ := io.ReadAll(losg)
	//fmt.Println(string(lines))

	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "22")
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(mappedPort.Port())

	sshCont := &sshContainer{
		Container: container,
		host:      ip,
		port:      port,
	}

	return sshCont, nil
}

func TestSshConnect(t *testing.T) {
	skipIfCi(t) // skip test if running in CI

	ctx := context.Background()
	sshServer, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = sshServer.Terminate(ctx)
	}()

	t.Run("connect using user password", func(t *testing.T) {
		cl, err := New(Cfg{
			Host:          sshServer.host,
			Port:          sshServer.port,
			Auth:          Password,
			User:          "pwuser",
			Password:      "1234",
			IgnoreHostKey: true,
		})

		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		err = cl.Connect()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer cl.Disconnect()

		session, err := cl.Session()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer session.Close()

		got, err := session.CombinedOutput("pwd")
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		want := []byte("/home/pwuser\n")

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("connect using private key", func(t *testing.T) {
		cl, err := New(Cfg{
			Host:          sshServer.host,
			Port:          sshServer.port,
			Auth:          PrivateKey,
			User:          "privkey",
			PrivateKey:    "./sampledata/private.key",
			PassPhrase:    "pass",
			IgnoreHostKey: true,
		})

		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		err = cl.Connect()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer cl.Disconnect()

		session, err := cl.Session()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		defer session.Close()

		got, err := session.CombinedOutput("pwd")
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		want := []byte("/home/privkey\n")

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("output mismatch (-want +got):\n%s", diff)
		}
	})

}

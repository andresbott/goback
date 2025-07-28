package rcp

import (
	"bytes"
	"context"
	"fmt"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func skipInCI(t *testing.T) {
	envSet := os.Getenv("RUN_TESTCONTAINERS")
	if envSet == "" {
		t.Skip("skipping because env \"RUN_TESTCONTAINERS\" is not set to true")
	}
}

type sshContainer struct {
	testcontainers.Container
	host string
	port int
	id   string
	name string
}

func setupContainer(ctx context.Context) (*sshContainer, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %v", err)
	}

	// Find project root by looking for go.mod file
	projectRoot := findProjectRoot(cwd)
	if projectRoot == "" {
		return nil, fmt.Errorf("failed to find project root (go.mod file not found in any parent directory)")
	}

	// Calculate relative path from project root to internal/rcp directory
	rcpDir := filepath.Join(projectRoot, "internal", "rcp")

	// Set paths relative to project root
	outputPath := filepath.Join(rcpDir, "sampledata", "backend")
	sourcePath := filepath.Join(rcpDir, "dummybackend", "backend.go")
	contextPath := filepath.Join(rcpDir, "sampledata")

	// Compile the backend binary
	cmd := exec.Command("go", "build", "-o", outputPath, sourcePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to compile backend: %v, stderr: %s", err, stderr.String())
	}

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       contextPath,
			Dockerfile:    "Dockerfile",
			PrintBuildLog: false, // set to true to troubleshoot docker build issues
		},
		ExposedPorts: []string{"22/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Server listening on 0.0.0.0 port 22"),
		),
		Env: map[string]string{
			"PW_USER": "kTZ8GVSkARoNg", // user: pwuser pw: 1234
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

	cname, err := container.Name(ctx)
	if err != nil {
		return nil, err
	}
	sshCont := &sshContainer{
		Container: container,
		host:      ip,
		port:      port,
		id:        container.GetContainerID(),
		name:      cname,
	}

	return sshCont, nil
}

//func TestBla(t *testing.T) {
//	// Create pipe to simulate interactive input
//	inputReader, inputWriter := io.Pipe()
//	outPutReader, outputWriter := io.Pipe()
//	var errBuff bytes.Buffer
//
//	// Start the remote session handler in a goroutine
//	done := make(chan any, 1)
//	go func() {
//		handleRemoteSession(inputReader, outputWriter, &errBuff)
//		done <- nil
//	}()
//
//	time.Sleep(50 * time.Millisecond)
//
//	localClient(outPutReader, inputWriter, &errBuff)
//
//	<-done
//}

func TestDockerExec(t *testing.T) {

	skipInCI(t) // Ensure this test is skipped if running in CI without "RUN_TESTCONTAINERS"

	ctx := context.Background()

	// Start the container
	container, err := setupContainer(ctx)
	if err != nil {
		panic(err) // You may choose to handle the error more gracefully
	}
	defer container.Terminate(ctx)

	// Open a shell to the container and run 'sudo mysqldump'
	cmd := exec.Command("docker", "exec", container.id, "sh", "-c", "mysqldump --all-databases")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("Error running mysqldump:", stderr.String())
		panic(err)
	}

	// Capture the output of mysqldump in a string and print it
	output := stdout.String()
	fmt.Println("Mysqldump output:", output)
}

// findProjectRoot looks for go.mod file in the current directory and parent directories
// to determine the project root
func findProjectRoot(dir string) string {
	// Check if go.mod exists in the current directory
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return dir
	}

	// If we've reached the root directory, return empty string
	parent := filepath.Dir(dir)
	if parent == dir {
		return ""
	}

	// Recursively check parent directories
	return findProjectRoot(parent)
}

func TestSsh(t *testing.T) {

	skipInCI(t) // Ensure this test is skipped if running in CI without "RUN_TESTCONTAINERS"

	ctx := context.Background()

	// Start the container
	container, err := setupContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Terminate(ctx)

	// Import the SSH package
	sshClient, err := ssh.New(ssh.Cfg{
		Host:          container.host,
		Port:          container.port,
		Auth:          ssh.Password,
		User:          "pwuser",
		Password:      "1234",
		IgnoreHostKey: true,
	})
	if err != nil {
		fmt.Println("Error creating SSH client:", err)
		t.Fatal(err)
	}

	// Connect to the container via SSH
	err = sshClient.Connect()
	if err != nil {
		fmt.Println("Error connecting to container via SSH:", err)
		t.Fatal(err)
	}
	defer sshClient.Disconnect()

	// Create a new SSH session
	session, err := sshClient.Session()
	if err != nil {
		fmt.Println("Error creating SSH session:", err)
		t.Fatal(err)
	}
	defer session.Close()

	// Run sudo mysqldump command with the correct path
	output, err := session.CombinedOutput("sudo mysqldump --all-databases")
	if err != nil {
		fmt.Println("Error running mysqldump via SSH:", err)
		t.Fatal(err)
	}
	fmt.Println("Mysqldump output:", string(output))

	// Create a new SSH session
	session, err = sshClient.Session()
	if err != nil {
		fmt.Println("Error creating SSH session:", err)
		t.Fatal(err)
	}
	defer session.Close()

	// sudo backend
	output, err = session.CombinedOutput("sudo backend")
	if err != nil {
		fmt.Println("Error running backend via SSH:", err)
		t.Fatal(err)
	}
	fmt.Println("backend output:", string(output))
}

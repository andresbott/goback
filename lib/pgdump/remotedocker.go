package pgdump

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/AndresBott/goback/lib/ssh"
)

type SshDockerCfg struct {
	// Docker configuration
	ContainerName string
	BinPath       string
	User          string
	Pw            string
	DbName        string
}

func WriteFromSshDocker(sshc *ssh.Client, cfg SshDockerCfg, writer io.Writer) (err error) {
	dbHandler, err := NewSshDocker(cfg)
	if err != nil {
		return err
	}

	// get default pg_dump path if not specified
	if cfg.BinPath == "" {
		binPath, err := GetSshDockerBinPath(sshc, cfg.ContainerName)
		if err != nil {
			return fmt.Errorf("unable to get path for pg_dump: %w", err)
		}
		dbHandler.SetBinPath(binPath)
	}

	err = dbHandler.Run(sshc, writer)
	if err != nil {
		return err
	}
	return nil
}

type SshDockerHandler struct {
	binPath       string
	containerName string
	dbName        string
	user          string
	pw            string
}

// NewSshDocker returns a new SSH+Docker pg_dump handler used to dump postgres in docker containers on remote servers
func NewSshDocker(cfg SshDockerCfg) (*SshDockerHandler, error) {
	if cfg.BinPath == "" {
		cfg.BinPath = "pg_dump"
	}

	h := SshDockerHandler{
		binPath:       cfg.BinPath,
		containerName: cfg.ContainerName,
		dbName:        cfg.DbName,
		user:          cfg.User,
		pw:            cfg.Pw,
	}

	return &h, nil
}

func (h *SshDockerHandler) SetBinPath(binPath string) {
	h.binPath = binPath
}

func (h *SshDockerHandler) Cmd() string {
	args := getArgs(h.user, h.pw, h.dbName)
	return h.binPath + " " + strings.Join(args, " ")
}

// GetSshDockerBinPath will check if pg_dump is available in the Docker container on the remote server
func GetSshDockerBinPath(sshc *ssh.Client, containerName string) (string, error) {
	// First check if docker is available
	sess, err := sshc.Session()
	if err != nil {
		return "", fmt.Errorf("unable to create ssh session: %v", err)
	}
	defer func() {
		// we ignore the EOF error on close since it is expected if session was closed by wait()
		if cErr := sess.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
			err = errors.Join(err, cErr)
		}
	}()

	// Check if docker is available
	_, err = sess.CombinedOutput("which docker")
	if err != nil {
		return "", fmt.Errorf("docker is not available on the remote server: %v", err)
	}

	// Docker is available, check if pg_dump is available in the container
	cmd := fmt.Sprintf("docker exec %s which pg_dump", containerName)

	sess2, err := sshc.Session()
	if err != nil {
		return "", fmt.Errorf("unable to create second ssh session: %v", err)
	}
	defer func() {
		// we ignore the EOF error on close since it is expected if session was closed by wait()
		if cErr := sess2.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
			err = errors.Join(err, cErr)
		}
	}()

	output, err := sess2.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("pg_dump not found in container %s: %v", containerName, err)
	}

	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", fmt.Errorf("pg_dump path is empty in container %s", containerName)
	}

	return path, nil
}

// Run will execute pg_dump in the Docker container on the remote server and write the output to the writer
func (h *SshDockerHandler) Run(sshc *ssh.Client, writer io.Writer) (err error) {
	args := getArgs(h.user, h.pw, h.dbName)

	// Check if docker is available using a separate session
	dockerCheckSess, err := sshc.Session()
	if err != nil {
		return fmt.Errorf("unable to create ssh session for docker check: %v", err)
	}
	defer func() {
		// we ignore the EOF error on close since it is expected if session was closed by wait()
		if cErr := dockerCheckSess.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
			err = errors.Join(err, cErr)
		}
	}()

	_, err = dockerCheckSess.CombinedOutput("which docker")
	if err != nil {
		return fmt.Errorf("docker is not available on the remote server: %v", err)
	}

	// Docker is available, build the docker exec command
	dockerCmd := fmt.Sprintf("docker exec %s %s %s",
		h.containerName,
		h.binPath,
		strings.Join(args, " "))

	// Set environment variables for PostgreSQL authentication
	if h.pw != "" {
		dockerCmd = fmt.Sprintf("docker exec -e PGPASSWORD=%s %s %s %s",
			h.pw,
			h.containerName,
			h.binPath,
			strings.Join(args, " "))
	}

	execSess, err := sshc.Session()
	if err != nil {
		return fmt.Errorf("unable to create ssh session for docker execution: %v", err)
	}
	defer func() {
		// we ignore the EOF error on close since it is expected if session was closed by wait()
		if cErr := execSess.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
			err = errors.Join(err, cErr)
		}
	}()

	outPipe, err := execSess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to set stdout pipe, %v", err)
	}

	err = execSess.Start(dockerCmd)
	if err != nil {
		return fmt.Errorf("unable to start ssh command: %v", err)
	}

	// Copy output to writer
	_, err = io.Copy(writer, outPipe)
	if err != nil {
		return fmt.Errorf("unable to copy pg_dump output: %v", err)
	}

	err = execSess.Wait()
	if err != nil {
		return fmt.Errorf("error with ssh command: %v", err)
	}

	return nil
}

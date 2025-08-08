package pgdump

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type DockerCfg struct {
	ContainerName string
	BinPath       string
	User          string
	Pw            string
	DbName        string
}

func WriteFromDocker(ctx context.Context, cfg DockerCfg, writer io.Writer) (err error) {
	dbHandler, err := NewDocker(cfg)
	if err != nil {
		return err
	}
	defer func() {
		cErr := dbHandler.Close()
		if cErr != nil {
			err = errors.Join(err, cErr)
		}
	}()

	// get default pg_dump path
	if cfg.BinPath == "" {
		binPath, err := GetDockerBinPath(ctx, dbHandler.DockerClient(), cfg.ContainerName)
		if err != nil {
			return fmt.Errorf("unable to get path for pg_dump: %w", err)
		}
		dbHandler.SetBinPath(binPath)
	}

	err = dbHandler.Run(ctx, writer)
	if err != nil {
		return err
	}
	return nil
}

type DockerHandler struct {
	binPath       string
	containerName string
	dbName        string
	user          string
	pw            string

	client *docker.Client
}

// NewDocker returns a new docker pg_dump handler used to dump postgres in docker containers
func NewDocker(cfg DockerCfg) (*DockerHandler, error) {

	if cfg.BinPath == "" {
		cfg.BinPath = "pg_dump"
	}
	h := DockerHandler{
		binPath:       cfg.BinPath,
		containerName: cfg.ContainerName,
		dbName:        cfg.DbName,
		user:          cfg.User,
		pw:            cfg.Pw,
	}

	// Create Docker client
	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("unable to create docker client: %v", err)
	}
	h.client = dockerClient

	return &h, nil
}

func (h *DockerHandler) DockerClient() *docker.Client {
	return h.client
}

func (h *DockerHandler) ContainerName() string {
	return h.containerName
}

func (h *DockerHandler) SetBinPath(binPath string) {
	h.binPath = binPath
}

func (h *DockerHandler) Close() error {
	return h.client.Close()
}

func (h *DockerHandler) Cmd() string {
	args := getArgs(h.user, h.pw, h.dbName)
	return h.binPath + " " + strings.Join(args, " ")
}

// Run will execute pg_dump and write the output into the passed writer
func (h *DockerHandler) Run(ctx context.Context, zipWriter io.Writer) error {

	args := getArgs(h.user, h.pw, h.dbName)

	// Create the exec
	execResp, err := h.client.ContainerExecCreate(ctx, h.containerName, container.ExecOptions{
		Cmd:          append([]string{h.binPath}, args...),
		AttachStdout: true,
		AttachStderr: true,
		Env:          []string{},
	})
	if err != nil {
		return fmt.Errorf("unable to create container exec: %v", err)
	}

	// Set environment variables for PostgreSQL authentication
	if h.pw != "" {
		execResp, err = h.client.ContainerExecCreate(ctx, h.containerName, container.ExecOptions{
			Cmd:          append([]string{h.binPath}, args...),
			AttachStdout: true,
			AttachStderr: true,
			Env:          []string{"PGPASSWORD=" + h.pw},
		})
		if err != nil {
			return fmt.Errorf("unable to create container exec with password: %v", err)
		}
	}

	// Attach to the exec to get the output
	output, err := h.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("unable to attach to container exec: %v", err)
	}
	defer output.Close()

	// Use stdcopy to properly demultiplex the stream instead of manually skipping headers
	_, err = stdcopy.StdCopy(zipWriter, io.Discard, output.Reader)
	if err != nil {
		return fmt.Errorf("unable to copy pg_dump output to zip: %v", err)
	}

	// Wait for the exec to complete and get the exit code
	inspectResp, err := h.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return fmt.Errorf("unable to inspect container exec: %v", err)
	}

	if inspectResp.ExitCode != 0 {
		return fmt.Errorf("pg_dump failed with exit code %d", inspectResp.ExitCode)
	}

	return nil
}

// GetDockerBinPath will check if pg_dump installed and return the corresponding absolute path
func GetDockerBinPath(ctx context.Context, dockerClient *docker.Client, containerName string) (string, error) {
	output := ""

	// Try to find pg_dump using 'which' command
	whichResp, err := dockerClient.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		Cmd:          []string{"which", "pg_dump"},
		AttachStdout: true,
		AttachStderr: false,
	})
	if err != nil {
		return output, fmt.Errorf("unable to create which command exec: %v", err)
	}

	whichOutput, err := dockerClient.ContainerExecAttach(ctx, whichResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return output, fmt.Errorf("unable to attach to which command exec: %v", err)
	}
	defer whichOutput.Close()

	// Read the output to get the pg_dump path
	header := make([]byte, 8)
	_, err = whichOutput.Reader.Read(header)
	if err != nil {
		return output, fmt.Errorf("unable to read Docker protocol header: %v", err)
	}

	pathBytes, err := io.ReadAll(whichOutput.Reader)
	if err != nil {
		return output, fmt.Errorf("unable to read which command output: %v", err)
	}

	// Check if which command found pg_dump
	whichInspect, err := dockerClient.ContainerExecInspect(ctx, whichResp.ID)
	if err != nil {
		return output, fmt.Errorf("unable to inspect which command exec: %v", err)
	}

	if whichInspect.ExitCode == 0 && len(pathBytes) > 0 {
		// Use the found path, trim whitespace
		output = string(pathBytes)
		output = strings.TrimSpace(output)
	}
	return output, nil
}

package pgdump

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/AndresBott/goback/lib/ssh"
)

type RemoteCfg struct {
	BinPath string
	User    string
	Pw      string
	DbName  string
}

func WriteFromRemote(sshc *ssh.Client, cfg RemoteCfg, writer io.Writer) (err error) {
	dbHandler, err := NewRemote(cfg)
	if err != nil {
		return err
	}

	// get default pg_dump path if not provided
	if cfg.BinPath == "" {
		binPath, err := GetRemoteBinPath(sshc)
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

type RemoteHandler struct {
	binPath string
	dbName  string
	user    string
	pw      string
}

// NewRemote returns a new remote pg_dump handler used to dump postgres on remote machines via SSH
func NewRemote(cfg RemoteCfg) (*RemoteHandler, error) {
	h := RemoteHandler{
		binPath: cfg.BinPath,
		dbName:  cfg.DbName,
		user:    cfg.User,
		pw:      cfg.Pw,
	}

	// get default pg_dump path if not provided
	if h.binPath == "" {
		h.binPath = "pg_dump"
	}

	return &h, nil
}

func (h *RemoteHandler) SetBinPath(binPath string) {
	h.binPath = binPath
}

func (h *RemoteHandler) Cmd() string {
	args := getArgs(h.user, h.pw, h.dbName)
	return h.binPath + " " + strings.Join(args, " ")
}

// GetRemoteBinPath will check if pg_dump is installed on the remote machine and return the corresponding absolute path
func GetRemoteBinPath(sshc *ssh.Client) (out string, err error) {
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

	// Try to find pg_dump using 'which' command
	output, err := sess.CombinedOutput("which pg_dump")
	if err != nil {
		return "", fmt.Errorf("unable to execute which command: %v", err)
	}

	// Check if which command found pg_dump
	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", fmt.Errorf("pg_dump not found on remote machine")
	}

	return path, nil
}

// Run will execute pg_dump on the remote machine via SSH and write the output to the zip file
func (h *RemoteHandler) Run(sshc *ssh.Client, writer io.Writer) (err error) {
	sess, err := sshc.Session()
	if err != nil {
		return fmt.Errorf("unable to create ssh session: %w", err)
	}
	defer func() {
		// we ignore the EOF error on close since it is expected if session was closed by wait()
		if cErr := sess.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
			err = errors.Join(err, cErr)
		}
	}()

	outPipe, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to set stdout pipe, %w", err)
	}

	// Set environment variables for PostgreSQL authentication
	if h.pw != "" {
		err := sess.Setenv("PGPASSWORD", h.pw)
		if err != nil {
			return fmt.Errorf("unable to set env var: %w", err)
		}
	}

	err = sess.Start(h.Cmd())
	if err != nil {
		return fmt.Errorf("unable to start ssh command: %w", err)
	}

	if _, err := io.Copy(writer, outPipe); err != nil {
		return fmt.Errorf("error writing output to writer: %w", err)
	}

	err = sess.Wait()
	if err != nil {
		return fmt.Errorf("error with ssh command: %w", err)
	}

	return nil
}

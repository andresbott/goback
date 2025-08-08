package pgdump

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

type LocalCfg struct {
	BinPath string
	User    string
	Pw      string
	DbName  string
}

func WriteLocal(cfg LocalCfg, writer io.Writer) error {
	dbHandler, err := NewLocal(cfg)
	if err != nil {
		return err
	}
	err = dbHandler.Run(writer)
	if err != nil {
		return err
	}
	return nil
}

type LocalHandler struct {
	binPath string
	dbName  string
	user    string
	pw      string
}

// NewLocal returns a new local pg_dump handler used to dump a specific database
func NewLocal(cfg LocalCfg) (*LocalHandler, error) {
	h := LocalHandler{
		binPath: cfg.BinPath,
		dbName:  cfg.DbName,
		user:    cfg.User,
		pw:      cfg.Pw,
	}

	// get default pg_dump path
	if h.binPath == "" {
		binPath, err := GetLocalBinPath()
		if err != nil {
			return nil, fmt.Errorf("unable to get path for pg_dump: %w", err)
		}
		h.binPath = binPath
	}

	// only try to read user/pw from postgres config if it is not explicitly set
	if cfg.User == "" || cfg.Pw == "" {
		err := h.loadCnfFiles(PostgresIniLocations())
		if err != nil {
			return nil, err
		}
	}
	return &h, nil
}

// GetLocalBinPath will check if pg_dump installed and return the corresponding absolute path
func GetLocalBinPath() (string, error) {
	// check for pg_dump installed
	binPath, err := exec.LookPath("pg_dump")
	if err != nil {
		return "", err
	}
	binPath, err = filepath.Abs(binPath)
	if err != nil {
		return "", err
	}

	return binPath, nil
}

// loadCnfFiles will try to extract the user/pw from known postgres ini files,
// if the information is not found, an error is returned
func (h *LocalHandler) loadCnfFiles(files []string) error {

	usr := ""
	pw := ""
	for _, file := range files {
		cfg, err := ini.Load(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		c := cfg.Section("postgres")
		if c.HasKey("user") {
			k, err := c.GetKey("user")
			if err != nil {
				return err
			}
			usr = k.String()
		}
		if c.HasKey("password") {
			k, err := c.GetKey("password")
			if err != nil {
				return err
			}
			pw = k.String()
		}
	}

	h.user = usr
	h.pw = pw

	return nil
}

func (h *LocalHandler) Cmd() (string, []string) {
	args := getArgs(h.user, h.pw, h.dbName)
	return h.binPath, args
}

// getArgs returns the cmd parameters to be used when we invoke pg_dump
func getArgs(user, pass, dbname string) []string {

	var args []string
	if user != "" {
		args = append(args, "-U", sanitizeString(user))
	}
	if pass != "" {
		args = append(args, "-W")
		// Note: pg_dump will prompt for password via environment variable PGPASSWORD
		// or through interactive prompt, but we'll handle this differently
	}
	args = append(args,
		"--clean",
		"--if-exists",
		"--create",
		"--verbose",
		sanitizeString(dbname),
	)
	return args
}

// Run will execute pg_dump and write the output into the passed writer
func (h *LocalHandler) Run(w io.Writer) error {

	// Check if binPath exists
	if _, err := os.Stat(h.binPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("binary not found at path: %s", h.binPath)
		}
		return fmt.Errorf("failed to check binary path: %w", err)
	}

	args := getArgs(h.user, h.pw, h.dbName)
	// #nosec G204 -- bin and args need to be provided by the caller
	cmd := exec.Command(h.binPath, args...)

	// Set environment variables for PostgreSQL authentication
	if h.pw != "" {
		cmd.Env = append(os.Environ(), "PGPASSWORD="+h.pw)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("error starting pg_dump command: %v", err)
	}

	if _, err := io.Copy(w, stdoutPipe); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("error writing output to writer: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("error running pg_dump: %v", err)
	}
	return nil
}

func sanitizeString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "\n", "_")
	s = strings.ReplaceAll(s, "\r", "_")
	return s
}

// PostgresIniLocations return a sorted list of locations to check for user/pw configuration
func PostgresIniLocations() []string {
	usr, _ := user.Current()
	return []string{
		"/etc/postgresql/postgresql.conf",
		filepath.Join(usr.HomeDir, "/.pgpass"),
		filepath.Join(usr.HomeDir, "/.psqlrc"),
	}
}

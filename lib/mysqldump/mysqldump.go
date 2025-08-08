package mysqldump

import (
	"errors"
	"fmt"
	"gopkg.in/ini.v1"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
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

// NewLocal returns a new local mysqldump handler used to dump a specific database
func NewLocal(cfg LocalCfg) (*LocalHandler, error) {
	h := LocalHandler{
		binPath: cfg.BinPath,
		dbName:  cfg.DbName,
		user:    cfg.User,
		pw:      cfg.Pw,
	}

	// get default mysqldump path
	if h.binPath == "" {
		binPath, err := GetLocalBinPath()
		if err != nil {
			return nil, fmt.Errorf("unable to get path for mysqldump: %w", err)
		}
		h.binPath = binPath
	}

	// only try to read user/pw from mysql config if it is not explicitly set
	if cfg.User == "" || cfg.Pw == "" {
		err := h.loadCnfFiles(MysqlIniLocations())
		if err != nil {
			return nil, err
		}
	}
	return &h, nil
}

// GetLocalBinPath will check if mysqldump installed and return the corresponding absolute path
func GetLocalBinPath() (string, error) {
	// check for mysqldump installed
	binPath, err := exec.LookPath("mysqldump")
	if err != nil {
		return "", err
	}
	binPath, err = filepath.Abs(binPath)
	if err != nil {
		return "", err
	}

	return binPath, nil
}

// loadCnfFiles will try to extract the user/pw from known mysql ini files,
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
		c := cfg.Section("client")
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

// getArgs returns the cmd parameters to be used when we invoke mysqldump
func getArgs(user, pass, dbname string) []string {

	var args []string
	if user != "" {
		args = append(args, "-u", sanitizeString(user))
	}
	if pass != "" {
		args = append(args, "-p"+sanitizeString(pass))
	}
	args = append(args,
		"--add-drop-database",
		"--databases",
		sanitizeString(dbname),
	)
	return args
}

// Run will execute mysqldump and write the output into the passed writer
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

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("error starting mysqldump command: %v", err)
	}

	if _, err := io.Copy(w, stdoutPipe); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("error writing output to writer: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("error running mysqldump: %v", err)
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

// MysqlIniLocations return a sorted list of locations to check for user/pw configuration
func MysqlIniLocations() []string {
	usr, _ := user.Current()
	return []string{
		"/etc/mysql/debian.cnf",
		filepath.Join(usr.HomeDir, "/.my.cnf"),
	}
}

package mysqldump

import (
	"errors"
	"fmt"
	"gopkg.in/ini.v1"
	"io"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

type Handler struct {
	binPath string
	dbName  string
	user    string
	pw      string
}

type Cfg struct {
	BinPath string
	User    string
	Pw      string
	DbName  string
}

// New returns a new mysqldump handler used to dump a specific database
func New(cfg Cfg) (*Handler, error) {
	h := Handler{
		binPath: cfg.BinPath,
		dbName:  cfg.DbName,
		user:    cfg.User,
		pw:      cfg.Pw,
	}

	// only try to read user/pw from mysql config if it is not explicitly set
	if cfg.User == "" || cfg.Pw == "" {
		err := h.userFromCnf(MysqlIniLocations())
		if err != nil {
			return nil, err
		}
	}
	return &h, nil
}

// userFromCnf will try to extract the user/pw from known mysql ini files,
// if the information is not found, an error is returned
func (h *Handler) userFromCnf(files []string) error {

	usr := ""
	pw := ""
	for _, file := range files {
		cfg, err := ini.Load(file)
		if err != nil {
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

	if usr == "" || pw == "" {
		return errors.New("user or password is empty")
	}
	h.user = usr
	h.pw = pw

	return nil
}

func (h Handler) Cmd() string {
	cmd, args := h.getCmd()
	return cmd + " " + strings.Join(args, " ")
}

// getCmd returns the cmd parameters to be used when we invoke mysqldump
func (h Handler) getCmd() (string, []string) {

	args := []string{
		"-u",
		h.user,
		"-p" + h.pw,
		"--add-drop-database",
		"--databases",
		h.dbName,
	}
	return h.binPath, args
}

// Exec will execute mysqldump and write the output into the passed writer
func (h Handler) Exec(w io.Writer) error {

	bin, args := h.getCmd()
	cmd := exec.Command(bin, args...)

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

// GetBinPath will check if mysqldump installed and return the corresponding absolute path
func GetBinPath() (string, error) {
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

// MysqlIniLocations return a sorted list of locations to check for user/pw configuration
func MysqlIniLocations() []string {
	usr, _ := user.Current()
	return []string{
		"/etc/mysql/debian.cnf",
		filepath.Join(usr.HomeDir, "/.my.cnf"),
	}
}

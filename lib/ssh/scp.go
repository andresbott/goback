package ssh

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
)

type ScpClient struct {
	scp.Client
}

// NewScp create a new scpClient that extend the one of go-scp adding some functions
func NewScp(conn *ssh.Client) (*ScpClient, error) {

	cl, err := scp.NewClientBySSH(conn)
	if err != nil {
		return nil, fmt.Errorf("error creating new SSH session from existing connection: %v", err)
	}
	scpcl := ScpClient{
		cl,
	}
	return &scpcl, nil
}

// WalkDir is a re-implementation of filepath.WalkDir over ssh
func (sshc *Client) WalkDir(root string, fn WalkFunc) error {
	info, err := sshc.Stat(root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = sshc.walkDir(root, &info, fn)
	}
	if errors.Is(err, ErrSkipDir) {
		return nil
	}
	return err
}

// walkDir recursively descends path, calling walkDirFn.
func (sshc *Client) walkDir(path string, d FileInfo, userFn WalkFunc) error {
	// call the user fn
	if err := userFn(path, d, nil); err != nil || !d.IsDir() {
		if errors.Is(err, ErrSkipDir) && d.IsDir() {
			// Successfully skipped directory.
			err = nil
		}
		// regardless if error is nil or not, if it is a file the process stops here
		// and the value of err is returned
		return err
	}

	dirs, err := sshc.readDir(path)
	if err != nil {
		// Second call, to report ReadDir error, and allow the user func handle the error
		err = userFn(path, d, err)
		if err != nil {
			return err
		}
	}

	for _, d1 := range dirs {

		path1 := filepath.Join(path, d1.Name())
		if err := sshc.walkDir(path1, d1, userFn); err != nil {
			if errors.Is(err, ErrSkipDir) {
				break
			}
			return err
		}
	}
	return nil
}

var ErrNotADir = errors.New("the given path is not a directory")

// Stat returns a FileInfo describing the named file from the file system.
func (sshc *Client) Stat(name string) (fstat FileStat, err error) {

	session, err := sshc.Session()
	if err != nil {
		return fstat, err
	}
	defer func() {
		// we ignore the EOF error on close since it is expected if session was closed by wait()
		if cErr := session.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
			err = errors.Join(err, cErr)
		}
	}()

	cmd := `stat --printf="%F_%n\n\n" "` + name + `"`

	// run command and capture stdout/stderr
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		if strings.Contains(string(output), "No such file or directory") {
			return fstat, os.ErrNotExist
		}
		return fstat, err
	}

	fs, err := parseStatLine(string(output))
	if err != nil {
		return fstat, fmt.Errorf("unable to get filestat: %v", err)
	}

	return fs, nil
}

func (sshc *Client) ReadDir(name string) ([]FileStat, error) {
	return sshc.readDir(name)
}

// readDir is an internal implementation that gts dir information of the remote path
func (sshc *Client) readDir(name string) (fstats []FileStat, err error) {
	sess, err := sshc.Session()
	if err != nil {
		return nil, err
	}
	defer func() {
		// we ignore the EOF error on close since it is expected if session was closed by wait()
		if cErr := sess.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
			err = errors.Join(err, cErr)
		}
	}()

	//cmd := fmt.Sprintf(`if [ -d %q ] && [ "$(ls -A %q)" ]; then stat --printf="%%F_%%n\n" %q/*; fi`, name, name, name)
	cmd := fmt.Sprintf(`
dir=%q

if [ ! -e "$dir" ]; then
  echo "__STAT_NO_EXISTS__"
elif [ ! -d "$dir" ]; then
  echo "__STAT_NO_DIR__"
elif [ "$(ls -A "$dir")" ]; then
  find "$dir" -mindepth 1 -maxdepth 1 -exec stat --printf="%%F_%%n\n" {} \;
fi
`, name)

	output, err := sess.Output(cmd)
	if err != nil {
		return nil, fmt.Errorf("command failed: %w", err)
	}
	if strings.Contains(string(output), "__STAT_NO_EXISTS__") {
		return nil, fmt.Errorf("%w: %s", os.ErrNotExist, name)
	}
	if strings.Contains(string(output), "__STAT_NO_DIR__") {
		return nil, fmt.Errorf("%w: %s", ErrNotADir, name)
	}

	lines := strings.Split(string(output), "\n")
	fsl := []FileStat{}
	for _, line := range lines {

		if line == "" {
			continue
		}
		fiStat, err := parseStatLine(line)
		if err != nil {
			return nil, err
		}
		fsl = append(fsl, fiStat)
	}
	return fsl, nil

}

// parseStatLine takes a line of a stat output  stat --printf="%F_%n\n\n" "/path"
// and generates a FileStat from it
func parseStatLine(in string) (FileStat, error) {
	in = strings.Trim(in, "\n")
	if strings.HasPrefix(in, "directory_") {
		path := filepath.Base(in[10:])
		fs := FileStat{
			isDir: true,
			name:  path,
		}
		return fs, nil
	}

	if strings.HasPrefix(in, "regular file_") {
		path := filepath.Base(in[13:])
		fs := FileStat{
			isDir: false,
			name:  path,
		}
		return fs, nil
	}

	if strings.HasPrefix(in, "regular empty file_") {
		path := filepath.Base(in[19:])
		fs := FileStat{
			isDir: false,
			name:  path,
		}
		return fs, nil
	}

	return FileStat{}, errors.New("file type not recognized: " + in[20:] + "...")
}

type WalkFunc func(path string, info FileInfo, err error) error

type FileInfo interface {
	Name() string // base name of the file
	IsDir() bool  // abbreviation for Mode().IsDir()
}

// ErrSkipDir is used as a return value from WalkFuncs to indicate that
// the directory named in the call is to be skipped. It is not returned
// as an error by any function.
var ErrSkipDir = errors.New("skip this directory")

// FileStat holds the information needed about files/directories to recursively walk them and call the user function
type FileStat struct {
	isDir bool
	name  string
}

func (f FileStat) IsDir() bool {
	return f.isDir
}

func (f FileStat) Name() string {
	return f.name
}

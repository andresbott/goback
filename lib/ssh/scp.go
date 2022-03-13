package ssh

import (
	"errors"
	"fmt"
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
func (scpc *Client) WalkDir(root string, fn WalkFunc) error {
	info, err := scpc.Stat(root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = scpc.walkDir(root, &info, fn)
	}
	if err == SkipDir {
		return nil
	}
	return err
}

// walkDir recursively descends path, calling walkDirFn.
func (scpc *Client) walkDir(path string, d FileInfo, userFn WalkFunc) error {
	// call the user fn
	if err := userFn(path, d, nil); err != nil || !d.IsDir() {
		if err == SkipDir && d.IsDir() {
			// Successfully skipped directory.
			err = nil
		}
		// regardless if error is nil or not, if it is a file the process stops here
		// and the value of err is returned
		return err
	}

	dirs, err := scpc.readDir(path)
	if err != nil {
		// Second call, to report ReadDir error, and allow the user func handle the error
		err = userFn(path, d, err)
		if err != nil {
			return err
		}
	}

	for _, d1 := range dirs {

		path1 := filepath.Join(path, d1.Name())
		if err := scpc.walkDir(path1, d1, userFn); err != nil {
			if err == SkipDir {
				break
			}
			return err
		}
	}
	return nil
}

// Stat returns a FileInfo describing the named file from the file system.
func (scpc *Client) Stat(name string) (FileStat, error) {

	session, err := scpc.Session()
	if err != nil {
		return FileStat{}, err
	}
	defer session.Close()

	cmd := `stat --printf="%F_%n\n\n" "` + name + `"`

	// run command and capture stdout/stderr
	output, err := session.Output(cmd)
	if err != nil {
		return FileStat{}, err
	}

	fs, err := parseStatLine(string(output))
	if err != nil {
		return FileStat{}, fmt.Errorf("unable to get filestat: %v", err)
	}

	return fs, nil
}

// ReadDir reads the directory named by dirname and returns
// a list of directory entries.
func (scpc *Client) ReadDir(name string) ([]FileStat, error) {

	fs, err := scpc.Stat(name)
	if err != nil {
		return nil, err
	}
	if !fs.isDir {
		return nil, errors.New("passed path is not a directory")
	}
	return scpc.readDir(name)
}

// readDir is an internal implementation that does not verify that the passed path is indeed a directory
func (scpc *Client) readDir(name string) ([]FileStat, error) {
	sess, err := scpc.Session()
	if err != nil {
		return nil, err
	}

	cmd := `stat --printf="%F_%n\n" "` + name + `"/*`

	// run command and capture stdout/stderr
	output, err := sess.Output(cmd)
	sess.Close()

	if err != nil {
		// this command does not allow to verify if the folder is empty or not
		// hence if this stat fails we check for empty dir
		sess2, err2 := scpc.Session()
		if err2 != nil {
			return nil, err2
		}

		cmd2 := `ls -A "` + name + `"`
		out, err := sess2.Output(cmd2)
		sess2.Close()

		// ls command is empty, henge empty folder
		if string(out) == "" {
			return []FileStat{}, nil
		}

		return nil, err
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

	return FileStat{}, errors.New("file type not recognized: " + in[20:] + "...")
}

type WalkFunc func(path string, info FileInfo, err error) error

type FileInfo interface {
	Name() string // base name of the file
	IsDir() bool  // abbreviation for Mode().IsDir()
}

// SkipDir is used as a return value from WalkFuncs to indicate that
// the directory named in the call is to be skipped. It is not returned
// as an error by any function.
var SkipDir = errors.New("skip this directory")

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

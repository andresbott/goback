package goback

import (
	"errors"
	"fmt"
	"github.com/AndresBott/goback/internal/profile"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/AndresBott/goback/lib/zip"
	"github.com/pkg/sftp"
	"os"
	"path/filepath"
)

type fileAdder interface {
	AddFile(origin string, dest string) error
	AddSymlink(origin string, dest string) error
}

// copyLocalFiles takes a single backup dir, recursively traverses the files and adds them to the zip handler
func copyLocalFiles(dir profile.BackupDir, fa fileAdder) error {

	rootDir := dir.Root

	// check if dir exists
	finfo, err := os.Lstat(rootDir)
	if err != nil {
		return err
	}

	// if root is a symlink follow it
	if finfo.Mode()&os.ModeSymlink == os.ModeSymlink {

		d, lErr := filepath.EvalSymlinks(rootDir)
		if lErr != nil {
			return fmt.Errorf("unable to evaluate symlink: %v", err)
		}
		finfo, err = os.Lstat(d)
		if err != nil {
			return err
		}
		rootDir = d
	}

	if !finfo.IsDir() {
		return errors.New("the path is not a directory")
	}

	// this function is called for every file/dir when walking the file system
	fn := func(path string, info os.FileInfo, err error) error {

		if err != nil {
			return fmt.Errorf("error waling directory: %v", err)
		}
		// skip directories, they are created by the zip handler
		if info.IsDir() {
			return nil
		}

		// skip excluded glob patterns
		for _, g := range dir.Exclude {
			if g.Match(path) {
				return nil
			}
		}

		// transform the origin to absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		// and back to relative for the destination
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// add the directory base to the destination
		// here we use the profile root not the calculated one in case of  symlink
		relPath = filepath.Join(filepath.Base(dir.Root), relPath)

		// if target is a symlink add the symlink
		if info.Mode()&os.ModeSymlink == os.ModeSymlink { // & is a bit AND
			err := fa.AddSymlink(absPath, relPath)
			if err != nil {
				return err
			}
			return nil
		}

		err = fa.AddFile(absPath, relPath)
		if err != nil {
			return err
		}
		return nil
	}

	err = filepath.Walk(rootDir, fn)

	if err != nil {
		return err
	}
	return nil
}

func copyRemoteFiles(sshc *ssh.Client, dir profile.BackupDir, zh *zip.Handler) error {

	sftpc, err := sftp.NewClient(sshc.Connection())
	if err != nil {
		return fmt.Errorf("unable to create sftp client %v", err)
	}
	defer sftpc.Close()

	rootDir := dir.Root
	if !filepath.IsAbs(rootDir) {
		wd, err := sftpc.Getwd()
		if err != nil {
			return fmt.Errorf("unable to get working dir %v", err)
		}
		rootDir = filepath.Join(wd, rootDir)
	}

	// check if dir exists
	finfo, err := sftpc.Stat(rootDir)
	if err != nil {
		return fmt.Errorf("error checking dir %s, %v", rootDir, err)
	}
	if !finfo.IsDir() {
		return errors.New("the path is not a directory")
	}

	w := sftpc.Walk(rootDir)

OUTER:
	for w.Step() {
		if w.Err() != nil {
			return fmt.Errorf("error walking directory")
		}
		info := w.Stat()

		// skip directories, they are created by the zip handler
		if info.IsDir() {
			continue OUTER
		}

		// skip excluded glob patterns
		for _, g := range dir.Exclude {
			if g.Match(w.Path()) {
				continue OUTER
			}
		}

		// transform to absolute path
		absPath := w.Path()
		if !filepath.IsAbs(w.Path()) {
			wd, err := sftpc.Getwd()
			if err != nil {
				return fmt.Errorf("unable to get working dir %v", err)
			}
			absPath = filepath.Join(wd, w.Path())
		}

		// and back to relative for the destination
		relPath, err := filepath.Rel(rootDir, absPath)
		if err != nil {
			return err
		}

		// add the directory base to the destination
		relPath = filepath.Join(filepath.Base(rootDir), relPath)

		f, err := sftpc.Open(w.Path())
		if err != nil {
			return fmt.Errorf("unable to open remote file %s cause: %v", w.Path(), err)
		}

		err = zh.WriteFile(f, relPath)
		if err != nil {
			return err
		}

	}

	return nil
}

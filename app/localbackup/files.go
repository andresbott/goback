package localbackup

import (
	"fmt"
	"github.com/AndresBott/goback/internal/profile"
	"os"
	"path/filepath"
)

type fileAdder interface {
	AddFile(origin string, dest string) error
	AddSymlink(origin string, dest string) error
}

func CopyFiles(dir profile.BackupDir, fileAdder fileAdder) error {

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
		// if the root is a single file, just add the file

		info, err := os.Stat(rootDir)
		if err != nil {
			return err
		}
		return addFile(rootDir, rootDir, info, fileAdder)

	}

	// walker function
	// this function is called for every file/dir when walking the file system
	fn := func(path string, info os.FileInfo, err error) error {

		if err != nil {
			return fmt.Errorf("error walking directory: %v", err)
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
		return addFile(rootDir, path, info, fileAdder)

	}

	err = filepath.Walk(rootDir, fn)

	if err != nil {
		return err
	}
	return nil
}

func addFile(rootDir, path string, info os.FileInfo, fileAdder fileAdder) error {
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
	relPath = filepath.Join(filepath.Base(rootDir), relPath)

	// if target is a symlink add the symlink
	if info.Mode()&os.ModeSymlink == os.ModeSymlink { // & is a bit AND
		err := fileAdder.AddSymlink(absPath, relPath)
		if err != nil {
			return err
		}
		return nil
	}

	err = fileAdder.AddFile(absPath, relPath)
	if err != nil {
		return err
	}
	return nil
}

package goback

import (
	"errors"
	"fmt"
	"github.com/AndresBott/goback/lib/zip"
	"os"
	"path/filepath"

	"github.com/AndresBott/goback/internal/profile"
)

func backupLocalFs(dirs []profile.BackupDir, dbs []profile.MysqlBackup, dest string) error {
	// here we create a zip file
	zh, err := zip.New(dest)
	if err != nil {
		return deleteZipErr(dest, err)
	}

	// dump file systemd data into zip
	for _, bkpDir := range dirs {
		err := dumpFileSystem(bkpDir, zh)
		if err != nil {
			return deleteZipErr(dest, err)
		}
	}

	// dump databases
	if len(dbs) > 0 {
		err = dumpDatabases(dbs, zh)
		if err != nil {
			return deleteZipErr(dest, err)
		}
	}

	// close the zip file at the end
	zh.Close()
	return nil
}

type fileAdder interface {
	AddFile(origin string, dest string) error
}

// dumpFileSystem takes a single backup dir, recursively traverses the files and adds them to the zip handler
func dumpFileSystem(dir profile.BackupDir, fa fileAdder) error {

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

	//spew.Dump(rootDir)
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

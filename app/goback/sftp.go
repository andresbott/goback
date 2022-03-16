package goback

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/AndresBott/goback/internal/profile"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/AndresBott/goback/lib/zip"
	"github.com/gobwas/glob"
	"github.com/pkg/sftp"
)

// syncBackups will pull backup files from a remote location to a local one,
// only downloading backups that are not present locally
func syncBackups(sshc *ssh.Client, remoteOrigin string, localDestination string, name string) error {

	sftpc, err := sftp.NewClient(sshc.Connection())
	if err != nil {
		return fmt.Errorf("unable to create sftp client %v", err)
	}
	defer sftpc.Close()

	// check local location
	locFinfo, err := os.Stat(localDestination)
	if err != nil {
		return fmt.Errorf("error checking dir %s, %v", localDestination, err)
	}
	if !locFinfo.IsDir() {
		return errors.New("the destination path is not a directory")
	}
	locFileInfos, err := os.ReadDir(localDestination)
	if err != nil {
		return fmt.Errorf("error reading dir %s, %v", localDestination, err)
	}

	localFile := []string{}
	for _, f := range locFileInfos {
		if !f.IsDir() {
			localFile = append(localFile, f.Name())
		}
	}

	// remote location
	remFinfo, err := sftpc.Stat(remoteOrigin)
	if err != nil {
		return fmt.Errorf("error checking dir %s, %v", remoteOrigin, err)
	}
	if !remFinfo.IsDir() {
		return errors.New("the path is not a directory")
	}

	remFileInfos, err := sftpc.ReadDir(remoteOrigin)
	if err != nil {
		return fmt.Errorf("error reading dir %s, %v", remoteOrigin, err)
	}

	remoteFiles := []string{}
	for _, f := range remFileInfos {
		if !f.IsDir() {
			remoteFiles = append(remoteFiles, f.Name())
		}
	}

	// compare files in both
	if name == "" {
		return errors.New("profile name cannot be empty")
	}

	diff := findDifferentProfiles(remoteFiles, localFile, name)

	for _, f := range diff {
		err = sftpDownload(sftpc, filepath.Join(remoteOrigin, f), filepath.Join(localDestination, f))
		if err != nil {
			return fmt.Errorf("unable to donwload file: %s, %v", f, err)
		}
	}

	_ = diff
	return nil
}

// sftpDownload uses an sft client to download a remote file to a local destination
func sftpDownload(sc *sftp.Client, remoteFile, localDest string) (err error) {

	// Note: SFTP To Go doesn't support O_RDWR mode
	srcFile, err := sc.OpenFile(remoteFile, (os.O_RDONLY))
	if err != nil {
		return fmt.Errorf("unable to open remote file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(localDest)
	if err != nil {
		return fmt.Errorf("unable to open local file: %v", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("unable to download remote file: %v", err)
	}

	return nil
}

// findDifferentProfiles takes two lists of profile names, and a name as pattern
// and returns a list of files to be pulled from remote
func findDifferentProfiles(remote []string, local []string, name string) []string {

	// this glob patter matches any file with the pattern: name_2006_02_01-15:04:05_backup.zip
	pattern := name + "_[0-9][0-9][0-9][0-9]_[0-9][0-9]_[0-9][0-9]-[0-9][0-9]:[0-9][0-9]:[0-9][0-9]_backup.zip"
	g, _ := glob.Compile(pattern)

	var remoteMatches []string
	for _, f := range remote {
		if g.Match(f) {
			remoteMatches = append(remoteMatches, f)
		}
	}

	missingLocally := []string{}
OUTER:
	for _, f := range remoteMatches {
		for _, l := range local {
			if f == l {
				continue OUTER
			}
		}
		missingLocally = append(missingLocally, f)
	}

	return missingLocally
}

func backupSftp(sshc *ssh.Client, dirs []profile.BackupDir, dbs []profile.MysqlBackup, dest string) error {
	// here we create a zip file
	zh, err := zip.New(dest)
	if err != nil {
		return deleteZipErr(dest, err)
	}

	// dump file systemd data into zip
	for _, bkpDir := range dirs {
		err := dumpSftp(sshc, bkpDir, zh)
		if err != nil {
			return deleteZipErr(dest, err)
		}
	}

	// dump databases
	if len(dbs) > 0 {
		err = dumpSshDatabases(sshc, dbs, zh)
		if err != nil {
			return deleteZipErr(dest, err)
		}
	}

	// close the zip file at the end
	zh.Close()
	return nil
}

// dumpFileSystem takes a single backup dir, recursively traverses the files and adds them to the zip handler
func dumpSftp(sshc *ssh.Client, dir profile.BackupDir, zh *zip.Handler) error {

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

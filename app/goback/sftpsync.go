package goback

import (
	"errors"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/pkg/sftp"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// copyRemoteFiles takes a single backup dir, connects over ssh and recursively traverses the files and adds them to the zip handler
func syncRemoteBackups(sftpc *sftp.Client, remotePath, profileName, destPath string, log *slog.Logger) error {

	// check local location
	locFileInfos, err := os.ReadDir(destPath)
	if err != nil {
		return fmt.Errorf("error reading dir %s, %v", destPath, err)
	}

	localFile := []string{}
	for _, f := range locFileInfos {
		if !f.IsDir() {
			localFile = append(localFile, f.Name())
		}
	}

	// remote location
	remFinfo, err := sftpc.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("error checking dir %s, %v", remotePath, err)
	}
	if !remFinfo.IsDir() {
		return errors.New("the path is not a directory")
	}

	remFileInfos, err := sftpc.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("error reading dir %s, %v", remotePath, err)
	}

	remoteFiles := []string{}
	for _, f := range remFileInfos {
		if !f.IsDir() {
			remoteFiles = append(remoteFiles, f.Name())
		}
	}

	// compare files in both
	if profileName == "" {
		return errors.New("profile name cannot be empty")
	}

	diff, err := findDifferentProfiles(remoteFiles, localFile, profileName)
	if err != nil {
		return err
	}

	for _, f := range diff {
		log.Debug("downloading remote file", "file", f)

		err = sftpDownload(sftpc, filepath.Join(remotePath, f), filepath.Join(destPath, f))
		if err != nil {
			return fmt.Errorf("unable to donwload file: %s, %v", f, err)
		}
	}
	_ = diff

	return err
}

// sftpDownload uses an sftp client to download a remote file to a local destination
func sftpDownload(sc *sftp.Client, remoteFile, localDest string) (err error) {

	// Note: SFTP To Go doesn't support O_RDWR mode
	srcFile, err := sc.OpenFile(remoteFile, os.O_RDONLY)
	if err != nil {
		return fmt.Errorf("unable to open remote file: %v", err)
	}
	defer func() {
		cErr := srcFile.Close()
		if cErr != nil {
			err = errors.Join(err, cErr)
		}
	}()
	// #nosec G304 -- path controlled by internal var
	dstFile, err := os.Create(localDest)
	if err != nil {
		return fmt.Errorf("unable to open local file: %v", err)
	}
	defer func() {
		cErr := dstFile.Close()
		if cErr != nil {
			err = errors.Join(err, cErr)
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("unable to download remote file: %v", err)
	}

	return nil
}

// findDifferentProfiles takes two lists of profile names, and a name as pattern
// and returns a list of files to be pulled from remote
func findDifferentProfiles(remote []string, local []string, name string) ([]string, error) {

	// this glob patter matches any file with the pattern: name_2006_02_01-15:04:05_backup.zip
	pattern := name + "_[0-9][0-9][0-9][0-9]_[0-9][0-9]_[0-9][0-9]-[0-9][0-9]:[0-9][0-9]:[0-9][0-9]_backup.zip"
	g, err := glob.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("glop pattern for '%s' does not compile: %v", name, err)
	}

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

	return missingLocally, nil
}

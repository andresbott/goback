package goback

import (
	"errors"
	"fmt"
	"github.com/AndresBott/goback/internal/clilog"
	"github.com/AndresBott/goback/lib/ssh"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AndresBott/goback/internal/profile"
)

type Printer interface {
	Print(msg string)
}

// date string used to format the backup profiles
// resulting profiles will be <name>_2006_02_01-15:04:05_backup.zip
const dateStr = "2006_02_01-15:04:05"

// BackupRunner is the entry point to the application
type BackupRunner struct {
	Printer  clilog.CliOut
	profiles []profile.Profile
}

// LoadProfile adds a single profile file to the list of profiles to be executed
func (br *BackupRunner) LoadProfile(file string) error {

	br.Printer.Print(fmt.Sprintf("Loading profile from file: \"%s\"", file))

	prfl, err := profile.LoadProfile(file)
	if err != nil {
		return err
	}
	br.profiles = append(br.profiles, prfl)
	return nil
}

// LoadProfiles adds all the profiles found in the directory to the list of profiles to be executed
func (br *BackupRunner) LoadProfiles(dir string) error {

	br.Printer.Print(fmt.Sprintf("Loading profiles from dorectory: \"%s\"", dir))

	prfl, err := profile.LoadProfiles(dir)
	if err != nil {
		return err
	}
	br.profiles = append(br.profiles, prfl...)
	return nil
}

// Run executes all the profiles loaded
func (br *BackupRunner) Run() error {

	capturedErrs := false

	for _, prfl := range br.profiles {

		// handle copying of files
		if len(prfl.Dirs) > 0 {
			br.Printer.Print(fmt.Sprintf("Running backup for profile: \"%s\"", prfl.Name))
			br.Printer.AddIndent()
			if len(prfl.Dirs) > 0 || len(prfl.Mysql) > 0 {
				start := time.Now()

				var err error
				err = br.BackupProfile(prfl)
				if err != nil {
					if prfl.Notify {
						// ignore notification error
						_ = NotifyFailure(prfl.NotifyCfg, err)
					}
					capturedErrs = true
					br.Printer.Print(fmt.Sprintf("[X] Error in backup of profile \"" + prfl.Name + "\": " + err.Error()))
					br.Printer.RemIndent()
					continue
				}

				t := time.Now()
				elapsed := t.Sub(start)
				br.Printer.Print(fmt.Sprintf("Backup took: \"%s\"", elapsed.String()))
			}
			br.Printer.RemIndent()
		}

		// sync remote backups
		if prfl.SyncBackup.RemotePath != "" {
			br.Printer.Print(fmt.Sprintf("Running sync of backups for profile: \"%s\"", prfl.Name))
			br.Printer.AddIndent()
			start2 := time.Now()
			err := br.SyncBackupsProfile(prfl)
			if err != nil {
				if prfl.Notify {
					// ignore notification error
					_ = NotifyFailure(prfl.NotifyCfg, err)
				}
				capturedErrs = true
				br.Printer.Print(fmt.Sprintf("[X] Error syncing backups files for profile \"" + prfl.Name + "\": " + err.Error()))
				br.Printer.RemIndent()
				continue
			}
			br.Printer.RemIndent()
			t2 := time.Now()
			elapsed2 := t2.Sub(start2)
			br.Printer.Print(fmt.Sprintf("Sync took: \"%s\"", elapsed2.String()))
			br.Printer.RemIndent()
		}

		// delete old backup files

		br.Printer.Print(fmt.Sprintf("Deleting older backups for profile: \"%s\"", prfl.Name))
		br.Printer.AddIndent()

		err := br.ExpurgeDir(prfl.Destination, prfl.Keep, prfl.Name)
		if err != nil {
			if prfl.Notify {
				// ignore notification error
				_ = NotifyFailure(prfl.NotifyCfg, err)
			}
			capturedErrs = true
			br.Printer.Print(fmt.Sprintf("[X] Error deleting files for profile \"" + prfl.Name + "\": " + err.Error()))
			br.Printer.RemIndent()
			continue
		}
		br.Printer.RemIndent()

		if prfl.Notify {
			_ = NotifySuccess(prfl.NotifyCfg)
		}
	}

	if capturedErrs {
		return errors.New("at least one profile execution was not successful")
	}
	return nil
}

// SyncBackupsProfile takes a remote (sftp) location from the profile and donwloads remote backups fiels
// to the local location
func (br BackupRunner) SyncBackupsProfile(prfl profile.Profile) error {
	// check if destination dir exists
	fInfo, err := os.Stat(prfl.Destination)
	if err != nil {
		return fmt.Errorf("destination not found: %v", err)
	}
	if !fInfo.IsDir() {
		return errors.New("the output path is not a directory")
	}

	// handle file backup
	if !prfl.IsRemote {
		return errors.New("sync backups only works with remote location enabled")
	}

	port, err := strconv.Atoi(prfl.Remote.Port)
	if err != nil {
		return fmt.Errorf("error parsisng port: %v", err)
	}

	sshC, err := ssh.New(ssh.Cfg{
		Host:          prfl.Remote.Host,
		Port:          port,
		Auth:          ssh.GetAuthType(prfl.Remote.RemoteType),
		User:          prfl.Remote.User,
		Password:      prfl.Remote.Password,
		PrivateKey:    prfl.Remote.PrivateKey,
		PassPhrase:    prfl.Remote.PassPhrase,
		IgnoreHostKey: false, // no need to expose this for the moment
	})

	if err != nil {
		return fmt.Errorf("error creating ssh client: %v", err)
	}
	err = sshC.Connect()
	if err != nil {
		return fmt.Errorf("error connecting ssh: %v", err)
	}
	defer func() {
		_ = sshC.Disconnect()
	}()

	err = br.syncBackups(sshC, prfl.SyncBackup.RemotePath, prfl.Destination, prfl.Name)
	if err != nil {
		return fmt.Errorf("error running sftp backup profile: %v", err)
	}
	return nil
}

// BackupProfile takes a single profile as input and generates a single Zip backup as output
// the sources of backup can be either local fs or sftp connection
func (br BackupRunner) BackupProfile(prfl profile.Profile) error {
	// check if destination dir exists
	fInfo, err := os.Stat(prfl.Destination)
	if err != nil {
		return fmt.Errorf("destination not found: %v", err)
	}
	if !fInfo.IsDir() {
		return errors.New("the output path is not a directory")
	}

	dest := filepath.Join(prfl.Destination, getZipName(prfl.Name))

	// handle file backup
	if prfl.IsRemote {

		port, err := strconv.Atoi(prfl.Remote.Port)
		if err != nil {
			return fmt.Errorf("error parsisng port: %v", err)
		}

		sshC, err := ssh.New(ssh.Cfg{
			Host:          prfl.Remote.Host,
			Port:          port,
			Auth:          ssh.GetAuthType(prfl.Remote.RemoteType),
			User:          prfl.Remote.User,
			Password:      prfl.Remote.Password,
			PrivateKey:    prfl.Remote.PrivateKey,
			PassPhrase:    prfl.Remote.PassPhrase,
			IgnoreHostKey: false, // no need to expose this for the moment
		})

		if err != nil {
			return fmt.Errorf("error creating ssh client: %v", err)
		}
		err = sshC.Connect()
		if err != nil {
			return fmt.Errorf("error connecting ssh: %v", err)
		}
		defer func() {
			_ = sshC.Disconnect()
		}()

		br.Printer.Print(fmt.Sprintf("Copying data from remote server: \"%s\"", prfl.Remote.Host))
		err = backupSftp(sshC, prfl.Dirs, prfl.Mysql, dest)
		if err != nil {
			return fmt.Errorf("error running sftp backup profile: %v", err)
		}

	} else {
		br.Printer.Print(fmt.Sprintf("Copying local data"))
		err := backupLocalFs(prfl.Dirs, prfl.Mysql, dest)
		if err != nil {
			return fmt.Errorf("error running local profile: %v", err)
		}
	}

	// change file ownership
	if prfl.Owner != "" {
		err := chown(dest, prfl.Owner)
		if err != nil {
			return fmt.Errorf("unable to change owner of file: \"%s\", %v", dest, err)
		}
	}

	// change file mode
	if prfl.Mode != "" {
		err := chmod(dest, prfl.Mode)
		if err != nil {
			return fmt.Errorf("unable to change perm of file: \"%s\", %v", dest, err)
		}
	}
	return nil
}

// deleteZipErr deletes the incomplete zip file in case onf an error, and returns the error
// if the delete opeation fails a new error is created that states both problems
func deleteZipErr(dest string, err error) error {
	//try to delete the zip file
	e := os.Remove(dest)
	if e != nil {
		return fmt.Errorf("unable to delete incomplete zip file due to: %v while handling error: %v", e, err)
	}
	return err
}

func chown(file string, owner string) error {
	usr, err := user.Lookup(owner)
	if err != nil {
		return fmt.Errorf("unable to find user: %s", err)
	}

	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return fmt.Errorf("user to id conversion: %v", err)
	}

	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return fmt.Errorf("user to gid conversion: %v", err)
	}

	err = os.Chown(file, uid, gid)
	if err != nil {
		return fmt.Errorf("chown failed: %v", err)
	}
	return nil
}

func chmod(file string, mode string) error {
	octal, err := strconv.ParseInt(mode, 8, 64)
	if err != nil {
		return fmt.Errorf("type conversion: %v", err)
	}

	err = os.Chmod(file, os.FileMode(octal))
	if err != nil {
		return fmt.Errorf("chmod failed: %v", err)
	}
	return nil
}

// getZipName generates the name of the output zip based on the input and a date combinations
func getZipName(in string) string {
	dt := time.Now()
	return in + "_" + dt.Format(dateStr) + "_backup.zip"
}

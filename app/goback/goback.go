package goback

import (
	"errors"
	"fmt"
	"github.com/AndresBott/goback/lib/ssh"
	"log/slog"
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
	Logger   *slog.Logger
	profiles []profile.Profile
}

// LoadProfileFile adds a single profile file to the list of profiles to be executed
func (br *BackupRunner) LoadProfileFile(file string) error {

	br.Logger.Info("Loading profile", "file", file)

	prfl, err := profile.LoadProfile(file)
	if err != nil {
		return err
	}
	br.profiles = append(br.profiles, prfl)
	return nil
}

// LoadProfilesDir adds all the profiles found in the directory to the list of profiles to be executed
func (br *BackupRunner) LoadProfilesDir(dir string) error {

	br.Logger.Info("Loading profile directory", "dor", dir)

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

			br.Logger.Info("Loading profile", "name", prfl.Name)

			if len(prfl.Dirs) > 0 || len(prfl.Mysql) > 0 {
				start := time.Now()

				err := br.BackupProfile(prfl)
				if err != nil {
					if prfl.Notify {
						// ignore notification error
						_ = NotifyFailure(prfl.NotifyCfg, err)
					}
					capturedErrs = true
					br.Logger.Error("Error in backup of profile", "err", err)
					continue
				}

				t := time.Now()
				elapsed := t.Sub(start)
				br.Logger.Info("Backup duration", "dur", elapsed)
			}
		}

		// delete old backup files
		br.Logger.Info("Deleting older backups for profile", "name", prfl.Name)

		err := br.ExpurgeDir(prfl.Destination, prfl.Keep, prfl.Name)
		if err != nil {
			if prfl.Notify {
				// ignore notification error
				_ = NotifyFailure(prfl.NotifyCfg, err)
			}
			capturedErrs = true
			br.Logger.Error("Error deleting files for profile", "err", err)
			continue
		}

		if prfl.Notify {
			_ = NotifySuccess(prfl.NotifyCfg)
		}
	}

	if capturedErrs {
		return errors.New("at least one profile execution was not successful")
	}
	return nil
}

// BackupProfile takes a single profile as input and generates a single Zip backup as output
// the sources of backup can be either local fs or sftp connection
func (br *BackupRunner) BackupProfile(prfl profile.Profile) error {
	// check if destination dir exists
	fInfo, err := os.Stat(prfl.Destination)
	if err != nil {
		// create dir if it does not exists
		if errors.Is(err, os.ErrNotExist) {
			mkdirErr := os.Mkdir(prfl.Destination, 0750)
			if mkdirErr != nil {
				return fmt.Errorf("unable to create backup destination: %v", err)
			}
			fInfo, err = os.Stat(prfl.Destination)
			if err != nil {
				return fmt.Errorf("unable to stat destination: %v", err)
			}
		} else {
			return fmt.Errorf("unable to stat destination: %v", err)
		}
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

		//br.Printer.Print(fmt.Sprintf("Copying data from remote server: \"%s\"", prfl.Remote.Host))
		err = backupSftp(sshC, prfl.Dirs, prfl.Mysql, dest)
		if err != nil {
			return fmt.Errorf("error running sftp backup profile: %v", err)
		}

	} else {
		//br.Printer.Print("Copying local data")
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

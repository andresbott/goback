package goback

import (
	"errors"
	"fmt"
	"github.com/AndresBott/goback/lib/mysqldump"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/AndresBott/goback/lib/zip"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AndresBott/goback/internal/profile"
)

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

	hadErr := false

	for _, prfl := range br.profiles {

		br.Logger.Info("Loading profile", "name", prfl.Name)
		start := time.Now()

		// run backup
		err := BackupProfile(prfl, br.Logger, getZipName(prfl.Name))
		if err != nil {
			hadErr = true
			br.Logger.Error("Error in backup of profile", "err", err)
			if prfl.Notify {
				err2 := NotifyFailure(prfl.NotifyCfg, err)
				if err2 != nil {
					br.Logger.Error("Error while sending notification", "err", err)
				}
			}
			continue
		}

		t := time.Now()
		elapsed := t.Sub(start)
		br.Logger.Info("Backup duration", "dur", elapsed)

		// delete old backup files
		br.Logger.Info("Deleting older backups for profile", "name", prfl.Name)
		err = ExpurgeDir(prfl.Destination, prfl.Keep, prfl.Name, br.Logger)
		if err != nil {
			hadErr = true
			br.Logger.Error("Error deleting files for profile", "err", err)
			if prfl.Notify {
				err2 := NotifyFailure(prfl.NotifyCfg, err)
				if err2 != nil {
					br.Logger.Error("Error while sending notification", "err", err)
				}
			}
			continue
		}

		// notify about completion
		if prfl.Notify {
			err2 := NotifySuccess(prfl.NotifyCfg)
			if err2 != nil {
				br.Logger.Error("Error while sending notification", "err", err)
			}
		}
	}

	if hadErr {
		return errors.New("at least one profile execution was not successful")
	}
	return nil
}

// BackupProfile takes a single profile as input and generates a single Zip backup as output
// the sources of backup can be either local fs or sftp connection
func BackupProfile(prfl profile.Profile, log *slog.Logger, zipName string) error {

	if len(prfl.Dirs) <= 0 && len(prfl.Mysql) <= 0 {
		log.Warn("Nothing to backup, skipping.")
		return nil
	}

	// check if destination dir exists, or create
	err := prepDest(prfl.Destination)
	if err != nil {
		return err
	}
	destZip := filepath.Join(prfl.Destination, zipName)

	// handle file backup
	if prfl.IsRemote {
		err = backupRemote(prfl, destZip)
		if err != nil {
			return delZipAndErr(destZip, err)
		}
	} else {
		err = backupLocal(prfl, destZip)
		if err != nil {
			return delZipAndErr(destZip, err)
		}
	}

	// change file ownership
	if prfl.Owner != "" {
		err := chown(destZip, prfl.Owner)
		if err != nil {
			return fmt.Errorf("unable to change owner of file: \"%s\", %v", destZip, err)
		}
	}

	// change file mode
	if prfl.Mode != "" {
		err := chmod(destZip, prfl.Mode)
		if err != nil {
			return fmt.Errorf("unable to change perm of file: \"%s\", %v", destZip, err)
		}
	}
	return nil
}

// prepDest will create the destination if it does not exist
//
//nolint:nestif // accepted error handling
func prepDest(dest string) error {
	fInfo, err := os.Stat(dest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			mkdirErr := os.Mkdir(dest, 0750)
			if mkdirErr != nil {
				return fmt.Errorf("unable to create backup destination: %v", err)
			}
			fInfo, err = os.Stat(dest)
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
	return nil
}

// backupLocal will run all the backup steps when running on the same machine
func backupLocal(prfl profile.Profile, zipDestination string) error {

	zipHandler, err := zip.New(zipDestination)
	if err != nil {
		return err
	}

	// copy files into the zip
	for _, bkpDir := range prfl.Dirs {
		err = copyLocalFiles(bkpDir, zipHandler)
		if err != nil {
			return err
		}
	}

	// dump mysql DBs into the zip
	if len(prfl.Mysql) > 0 {

		// check for mysqldump installed
		binPath, err := mysqldump.GetBinPath()
		if err != nil {
			return err
		}

		for _, db := range prfl.Mysql {
			err := copyLocalMysql(binPath, db, zipHandler)
			if err != nil {
				return err
			}
		}
	}

	// close the zip file at the end
	zipHandler.Close()
	return nil
}

// exposed internally for testing purposes only
var ignoreHostKey = false

// backupRemote will open an ssh connection to a remote location and run copy of files and dbs
func backupRemote(prfl profile.Profile, dest string) error {
	port, err := strconv.Atoi(prfl.Remote.Port)
	if err != nil {
		return fmt.Errorf("error parsisng port: %v", err)
	}

	sshC, err := ssh.New(ssh.Cfg{
		Host:          prfl.Remote.Host,
		Port:          port,
		Auth:          ssh.GetAuthType(prfl.Remote.AuthType),
		User:          prfl.Remote.User,
		Password:      prfl.Remote.Password,
		PrivateKey:    prfl.Remote.PrivateKey,
		PassPhrase:    prfl.Remote.PassPhrase,
		IgnoreHostKey: ignoreHostKey, // set to false and only exposed for testing
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

	zipHandler, err := zip.New(dest)
	if err != nil {
		return err
	}

	// dump filesystem data into zip
	for _, bkpDir := range prfl.Dirs {
		err := copyRemoteFiles(sshC, bkpDir, zipHandler)
		if err != nil {
			return err
		}
	}

	// dump mysql DBs into the zip
	if len(prfl.Mysql) > 0 {
		binPath, err := sshC.Which("mysqldump")
		if err != nil {
			return fmt.Errorf("error checking mysql binary: %v", err)
		}
		for _, db := range prfl.Mysql {
			err := copyRemoteMysql(sshC, binPath, db, zipHandler)
			if err != nil {
				return err
			}
		}
	}

	// close the zip file at the end
	zipHandler.Close()
	return nil
}

// delZipAndErr deletes the incomplete zip file in case onf an error, and returns the error
// if the delete operation fails a new error is created that states both problems
func delZipAndErr(dest string, err error) error {
	//try to delete the temp zip file
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
	octal, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("type conversion: %v", err)
	}

	err = os.Chmod(file, os.FileMode(uint32(octal))) // safe cast
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

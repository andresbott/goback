package goback

import (
	"errors"
	"fmt"
	"github.com/AndresBott/goback/lib/mysqldump"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/AndresBott/goback/lib/zip"
	"github.com/pkg/sftp"
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
// note that profile.LoadProfiles returns a partial correct result, hence even if there were errors
// the callers can still run the correct profiles.
func (br *BackupRunner) LoadProfilesDir(dir string) error {

	br.Logger.Info("Loading profile directory", "dor", dir)

	prfl, err := profile.LoadProfiles(dir)
	// we still want to append the correct profiles
	br.profiles = append(br.profiles, prfl...)
	if err != nil {
		return err
	}
	return nil
}

// Run executes all the profiles loaded
func (br *BackupRunner) Run() error {

	var errs error

	for _, prfl := range br.profiles {
		err := br.RunProfile(prfl)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("profile %s failed: %w", prfl.Name, err))
		}
	}

	if errs != nil {
		// we don't need to unwarp the errors, they are logged already (?)
		return errors.New("at least one profile execution was not successful")
	}
	return nil
}

// RunProfile Runs a single backup profile
func (br *BackupRunner) RunProfile(prfl profile.Profile) error {
	br.Logger.Info("Loading profile", "name", prfl.Name)
	start := time.Now()

	type runnerFn func(profile.Profile, *slog.Logger) error
	var runFn runnerFn
	switch prfl.Type {
	case profile.TypeLocal:
		runFn = runLocalProfile
	case profile.TypeRemote:
		runFn = runRemoteProfile
	case profile.TypeSftpSync:
		runFn = runSyncProfile
	default:
		return fmt.Errorf("unknown profile type: %s", prfl.Type)
	}

	err := RunWithNotify(prfl, br.Logger, runFn)
	if err != nil {
		return fmt.Errorf("profile %s failed: %w", prfl.Name, err)
	}

	t := time.Now()
	elapsed := t.Sub(start)
	br.Logger.Info("Backup duration", "dur", elapsed)
	return nil
}

// RunWithNotify ias a wrapper function to the different profile runner functions, it will call the run function
// and if the profile notification is defined it will send the profile owner notification out.
func RunWithNotify(prfl profile.Profile, log *slog.Logger, fn func(prfl profile.Profile, log *slog.Logger) error) error {
	err := fn(prfl, log)
	if err != nil {
		if prfl.Notify.HasValues() {
			err2 := NotifyFailure(prfl.Notify, prfl.Name, err)
			if err2 != nil {
				log.Error("Error while sending notification", "err", err2)
			}
		}
		return err
	}
	if prfl.Notify.HasValues() {
		err2 := NotifySuccess(prfl.Notify, prfl.Name)
		if err2 != nil {
			log.Error("Error while sending notification", "err", err2)
		}
	}
	return nil
}

// runLocalProfile takes a single profile as input and generates a single Zip backup as output
// the sources of backup MUST  be a local profile
func runLocalProfile(prfl profile.Profile, log *slog.Logger) error {

	// check if destination dir exists, or create
	err := prepareDestination(prfl.Destination.Path)
	if err != nil {
		return err
	}
	destZip := filepath.Join(prfl.Destination.Path, getZipName(prfl.Name))

	log.Info("backing up local profile to file", "destination", destZip)
	err = backupLocal(prfl, destZip, log)
	if err != nil {
		return delZipAndErr(destZip, err)
	}

	// change file ownership
	if prfl.Destination.Owner != "" {
		err := chown(destZip, prfl.Destination.Owner)
		if err != nil {
			return fmt.Errorf("unable to change owner of file: \"%s\", %v", destZip, err)
		}
	}

	// change file mode
	if prfl.Destination.Mode != "" {
		err := chmod(destZip, prfl.Destination.Mode)
		if err != nil {
			return fmt.Errorf("unable to change perm of file: \"%s\", %v", destZip, err)
		}
	}

	if prfl.Destination.Keep > 0 {
		// delete old backup files
		log.Info("Deleting older backups for profile", "name", prfl.Name)
		err = ExpurgeDir(prfl.Destination.Path, prfl.Destination.Keep, prfl.Name, log)
		if err != nil {
			return fmt.Errorf("error expurging old backup files: %w", err)
		}
	} else {
		log.Info("skipping deleting older backups because", "name", prfl.Name)
	}

	return nil
}

// backupLocal will run all the backup steps when running on the same machine
func backupLocal(prfl profile.Profile, zipDestination string, log *slog.Logger) error {

	zipHandler, err := zip.New(zipDestination)
	if err != nil {
		return err
	}

	// copy files into the zip
	for _, bkpDir := range prfl.Dirs {
		log.Info("backing up directory", "dir", bkpDir.Path)
		err = copyLocalFiles(bkpDir, zipHandler)
		if err != nil {
			return err
		}
	}

	// dump mysql DBs into the zip
	if len(prfl.Dbs) > 0 {
		for _, db := range prfl.Dbs {
			switch db.Type {
			case profile.DbMysql, profile.DbMaria:
				// check for mysqldump installed
				binPath, err := mysqldump.GetBinPath()
				if err != nil {
					return err
				}

				log.Info("backing up mysql/mariaDB database", "db", db.Name)
				err = copyLocalMysql(binPath, db, zipHandler)
				if err != nil {
					return err
				}

			default:
				return fmt.Errorf("unknown db type: %s", db.Type)
			}
		}
	}

	// close the zip file at the end
	zipHandler.Close()
	return nil
}

// runLocalProfile takes a single profile as input and generates a single Zip backup as output
// the sources of backup MUST be a remote profile
func runRemoteProfile(prfl profile.Profile, log *slog.Logger) error {

	// check if destination dir exists, or create
	err := prepareDestination(prfl.Destination.Path)
	if err != nil {
		return err
	}
	destZip := filepath.Join(prfl.Destination.Path)

	log.Info("backing up remote profile to file", "destination", destZip)
	err = backupRemote(prfl, destZip, log)
	if err != nil {
		return delZipAndErr(destZip, err)
	}

	// change file ownership
	if prfl.Destination.Owner != "" {
		err := chown(destZip, prfl.Destination.Owner)
		if err != nil {
			return fmt.Errorf("unable to change owner of file: \"%s\", %v", destZip, err)
		}
	}

	// change file mode
	if prfl.Destination.Mode != "" {
		err := chmod(destZip, prfl.Destination.Mode)
		if err != nil {
			return fmt.Errorf("unable to change perm of file: \"%s\", %v", destZip, err)
		}
	}

	if prfl.Destination.Keep > 0 {
		// delete old backup files
		log.Info("Deleting older backups for profile", "name", prfl.Name)
		err = ExpurgeDir(prfl.Destination.Path, prfl.Destination.Keep, prfl.Name, log)
		if err != nil {
			return fmt.Errorf("error expurging old backup files: %w", err)
		}
	} else {
		log.Info("skipping deleting older backups because", "name", prfl.Name)
	}

	return nil
}

// exposed internally for testing purposes only
var ignoreHostKey = false

// backupRemote will open an ssh connection to a remote location and run copy of files and dbs
func backupRemote(prfl profile.Profile, dest string, log *slog.Logger) error {

	sshC, err := ssh.New(ssh.Cfg{
		Host:          prfl.Ssh.Host,
		Port:          prfl.Ssh.Port,
		Auth:          ssh.GetAuthType(prfl.Ssh.Type),
		User:          prfl.Ssh.User,
		Password:      prfl.Ssh.Password,
		PrivateKey:    prfl.Ssh.PrivateKey,
		PassPhrase:    prfl.Ssh.Passphrase,
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
		log.Info("backing up directory", "dir", bkpDir.Path)
		err := copyRemoteFiles(sshC, bkpDir, zipHandler)
		if err != nil {
			return err
		}
	}

	if len(prfl.Dbs) > 0 {
		for _, db := range prfl.Dbs {
			switch db.Type {
			case profile.DbMysql, profile.DbMaria:
				binPath, err := sshC.Which("mysqldump")
				if err != nil {
					return fmt.Errorf("error checking mysql binary: %v", err)
				}

				log.Info("backing up mysql database", "db", db.Name)
				err = copyRemoteMysql(sshC, binPath, db, zipHandler)
				if err != nil {
					return err
				}

			default:
				return fmt.Errorf("unknown db type: %s", db.Type)
			}
		}
	}

	// close the zip file at the end
	zipHandler.Close()
	return nil
}

// runSyncProfile takes a remote (sftp) location from the profile and downloads remote backups files
// to the local location
// the sources of backup MUST be a sftpSync profile
func runSyncProfile(prfl profile.Profile, log *slog.Logger) (err error) {

	// check if destination dir exists, or create
	err = prepareDestination(prfl.Destination.Path)
	if err != nil {
		return err
	}

	sshC, err := ssh.New(ssh.Cfg{
		Host:          prfl.Ssh.Host,
		Port:          prfl.Ssh.Port,
		Auth:          ssh.GetAuthType(prfl.Ssh.Type),
		User:          prfl.Ssh.User,
		Password:      prfl.Ssh.Password,
		PrivateKey:    prfl.Ssh.PrivateKey,
		PassPhrase:    prfl.Ssh.Passphrase,
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

	sftpc, err := sftp.NewClient(sshC.Connection())
	if err != nil {
		return fmt.Errorf("unable to create sftp client %v", err)
	}
	defer func() {
		cErr := sftpc.Close()
		if cErr != nil {
			err = errors.Join(err, cErr)
		}
	}()

	// copy remote dirs contents into local
	for _, syncDir := range prfl.Dirs {
		log.Info("synchronising remote directory", "dir", syncDir.Path)
		err = syncRemoteBackups(sftpc, syncDir.Path, syncDir.Name, prfl.Destination.Path, log)
		if err != nil {
			return err
		}

		if prfl.Destination.Keep > 0 {
			// delete old backup files
			log.Info("Deleting older backups for profile", "name", syncDir.Name)
			err = ExpurgeDir(prfl.Destination.Path, prfl.Destination.Keep, syncDir.Name, log)
			if err != nil {
				return fmt.Errorf("error expurging old backup files: %w", err)
			}
		} else {
			log.Info("skipping deleting older backups because", "name", prfl.Name)
		}
	}

	return nil
}

// prepareDestination will create the destination if it does not exist
//
//nolint:nestif // accepted error handling
func prepareDestination(dest string) error {
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

package goback

import (
	"fmt"
	"github.com/AndresBott/goback/internal/profile"
	"github.com/AndresBott/goback/lib/mysqldump"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/AndresBott/goback/lib/zip"
	"path/filepath"
)

// dumpDatabases iterates over multiple mysql profiles and tries to dump all the databases into the current zip file
func dumpDatabases(dbProfiles []profile.MysqlBackup, zip *zip.Handler) error {
	// check for mysqldump installed
	binPath, err := mysqldump.GetBinPath()
	if err != nil {
		return err
	}

	for _, dbPrfl := range dbProfiles {
		err := dumpDb(binPath, dbPrfl, zip)
		if err != nil {
			return err
		}
	}
	return nil
}

// dump a single database into a zip handler
func dumpDb(binPath string, dbPrfl profile.MysqlBackup, zip *zip.Handler) error {

	h, err := mysqldump.New(mysqldump.Cfg{
		BinPath: binPath,
		User:    dbPrfl.User,
		Pw:      dbPrfl.Pw,
		DbName:  dbPrfl.DbName,
	})

	if err != nil {
		return err
	}

	zlipWriter, err := zip.FileWriter(filepath.Join("_mysqldump", dbPrfl.DbName+".dump.slq"))
	if err != nil {
		return err
	}
	err = h.Exec(zlipWriter)
	if err != nil {
		return err
	}
	return nil
}

// dumpDatabases iterates over multiple mysql profiles and tries to dump all the databases into the current zip file
func dumpSshDatabases(sshc *ssh.Client, dbProfiles []profile.MysqlBackup, zip *zip.Handler) error {

	// rely on mysldump to be installed on $PATH
	for _, dbPrfl := range dbProfiles {
		err := dumpSshDb(sshc, "mysqldump", dbPrfl, zip)
		if err != nil {
			return err
		}
	}
	return nil
}

func dumpSshDb(sshc *ssh.Client, binPath string, dbPrfl profile.MysqlBackup, zip *zip.Handler) error {
	h, err := mysqldump.New(mysqldump.Cfg{
		BinPath: binPath,
		User:    dbPrfl.User,
		Pw:      dbPrfl.Pw,
		DbName:  dbPrfl.DbName,
	})
	if err != nil {
		return err
	}

	sess, err := sshc.Session()
	if err != nil {
		return fmt.Errorf("unable to create ssh session: %v", err)
	}

	outPipe, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to set stdout pipe, %v", err)
	}

	err = sess.Start(h.Cmd())
	if err != nil {
		return fmt.Errorf("unable to start ssh command: %v", err)
	}

	err = zip.WriteFile(outPipe, filepath.Join("_mysqldump", dbPrfl.DbName+".dump.slq"))
	if err != nil {
		return fmt.Errorf("unable write mysqloutout to zip file, %v", err)
	}

	err = sess.Wait()
	if err != nil {
		return fmt.Errorf("error with ssh command: %v", err)
	}

	return nil
}

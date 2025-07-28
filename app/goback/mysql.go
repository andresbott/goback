package goback

import (
	"errors"
	"fmt"
	"github.com/AndresBott/goback/internal/profile"
	"github.com/AndresBott/goback/lib/mysqldump"
	"github.com/AndresBott/goback/lib/ssh"
	"github.com/AndresBott/goback/lib/zip"
	"io"
	"path/filepath"
)

// StreamMysqlFile calls mysqldump on the remote machine and stream it's content
// to stdout, if any issue happens it will be streamed to stderr.
// this is used so that another process can take the content and store it in a backup file
func StreamMysqlFile(binPath string, dbPrfl profile.MysqlBackup, stdout, stderr io.Writer) error {

	return nil
}

// calls mysqldump on the local machine and stores it's contents on the zip file
func copyLocalMysql(binPath string, dbPrfl profile.MysqlBackup, zipHandler *zip.Handler) error {

	dbHandler, err := mysqldump.New(mysqldump.Cfg{
		BinPath: binPath,
		User:    dbPrfl.User,
		Pw:      dbPrfl.Pw,
		DbName:  dbPrfl.DbName,
	})

	if err != nil {
		return err
	}

	zipWriter, err := zipHandler.FileWriter(filepath.Join("_mysqldump", dbPrfl.DbName+".dump.sql"))
	if err != nil {
		return err
	}
	err = dbHandler.Exec(zipWriter)
	if err != nil {
		return err
	}
	return nil
}

// copyRemoteMysql uses an ssh connections to open a new session to the given remote, call mysqldump and store the
// streamed data on the local zip backup file.
func copyRemoteMysql(sshc *ssh.Client, binPath string, dbPrfl profile.MysqlBackup, zip *zip.Handler) (err error) {
	h, err := mysqldump.New(mysqldump.Cfg{
		BinPath: binPath,
		User:    dbPrfl.User,
		Pw:      dbPrfl.Pw,
		DbName:  dbPrfl.DbName,
	})
	if err != nil {
		return fmt.Errorf("nable to create mysqldump wrapper: %v", err)
	}

	sess, err := sshc.Session()
	if err != nil {
		return fmt.Errorf("unable to create ssh session: %v", err)
	}
	defer func() {
		// we ignore the EOF error on close since it is expected if session was closed by wait()
		if cErr := sess.Close(); cErr != nil && !errors.Is(cErr, io.EOF) {
			err = errors.Join(err, cErr)
		}
	}()

	outPipe, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to set stdout pipe, %v", err)
	}

	err = sess.Start(h.Cmd())
	if err != nil {
		return fmt.Errorf("unable to start ssh command: %v", err)
	}

	err = zip.WriteFile(outPipe, filepath.Join("_mysqldump", dbPrfl.DbName+".dump.sql"))
	if err != nil {
		return fmt.Errorf("unable write mysqloutout to zip file, %v", err)
	}

	err = sess.Wait()
	if err != nil {
		return fmt.Errorf("error with ssh command: %v", err)
	}

	return nil
}

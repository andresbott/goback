# GoBack

[![CircleCI](https://circleci.com/gh/AndresBott/goback/tree/main.svg?style=svg)](https://circleci.com/gh/AndresBott/goback/tree/main)

## Use

1.  Generate a profile 
```
goback -g > profile.backup.yaml
```
2. Modify the profile according to your needs

Details:
* _name_: the base name used when generating compressed files 
* _remote_: if defined, goback will backup remote locations instead of local ones
  * _type_ [ sshPassword | sshKey | sshAgent ]: how to login to the remote server.
  * _host_: ssh host  
  * _port_: ssh port
  * _user_: ssh user
  * _password_: plain text ssh password, used if type is sshPassword
  * _privateKey_: path to a private key, used if type is sshKey
  * _passPhrase_: plain text pass phrase to the private key

* _dirs_: is a list of directories to compress and download.
  * _root_: root of the path to backup.
  * _exclude_: a list of glob patterns of files to exclude from the backup.

example:
```
- root: /home/user/
  exclude:
    - "*.log"
- root: /var/log
```


* _mysql_: uses mysqldump ( needs to be installed ) to add databases to the backup. list of databases to backup.
  * _dbname_: database name
  * _user_: database user to login to the db. Leave empty to let the tool try to get access.
  * _pw_: database user to login to the db. Leave empty to let the tool try to get access.
  * Note: goback will try to get root credentials for mysql from common locations like /etc/my.cnf

example:
```
mysql:
    - dbname: dbname
      user: user
      pw: pw

```

* _syncBackups_: will connect to the remote location, and download existing backup files to the local path.
* _destination_: local path where backup files are created
* _keep_: how many older backups to keep for this profile
* _owner_: change the owner of the resulting backup file
* _mode_: change the mode of the resulting backup file

* _notify_: notify per email if a profile was successful or not
  * _to_: list of email addresses
  * host: email server host
  * port: email sever port
  * user: email server user to login
  * password: password for that user on the email server


## Roadmap

* backup git repositories
* backuo github orgs
* allow to use envs placeholders in profiles, e.g. for secrets
* write documentation
* exclude folders that contain .nobackup
* improve the expurge rules to keep N yearly, monthly etc

#### TODO
* use systemd timers instead of cron
* mysqldump should not depend on my.cnf
* add option to follow symlink instead of adding them to the backup file
* use tar.gz instead of zip to keep file permissions
* don't fail on broken symlinks

## Development

#### Requirements

* go
* make
* docker
* goreleaser
* golangci-lint
* git 

#### Release

make sure you have your gh token stored locally in ~/.goreleaser/gh_token

to release a new version:
```bash 
make release  version="v0.1.2"
```

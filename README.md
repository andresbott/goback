# GoBack

[![CircleCI](https://circleci.com/gh/andresbott/goback/tree/main.svg?style=svg)](https://circleci.com/gh/AndresBott/goback/tree/main)
 
Goback is a simple backup utility that generates zip files out of content.

## Getting started

Goback uses profile files to define the backup actions to perform, when invoking goback you can call a single
profile file or a folder with profiles; 

> Important!  all profiles withing a folder need to end with ".backup.yaml"
 
1. First we create a profile with
```
goback generate > ./profilesDir/my-profile.backup.yaml
```
2. Now we edit according our needs, see [profile details](Profile Details) below 

3. Now we can validate the configuration with 
```
goback validate ./profilesdir/my-profile.backup.yaml
# or
goback validate ./profilesdir/
```
4. To run the backup simply run 
```
goback backup ./profilesdir/my-profile.backup.yaml
# or
goback backup ./profilesdir/
```


## Profile Details

Currently, goback supports 3 **types** of profiles:

**Local**:
* intended to backup files on the same OS as the process runs
* Local backup is specified by the type `type: "Local"`
* mandatory fields: `dirs` or `dbs`, `destination`

**Remote**:
* opens an ssh connection to a target and runs the backup there, storing content locally
* remote backup is specified by the type `type: "remote"`
* mandatory fields: `dirs` or `dbs`, `ssh` and `destination`

**sftpSync**:
* connects to a remote target using SFTP, and pulls goback Backup files from remote to local.
This is useful if you have a remote machine running local backups, and you want a way to pull them into
another machine.
* specified by the type `type: "sftpsync"`
* mandatory fields: `dirs` or `dbs`, `ssh` and `destination`

### Details: V1

```
---
version: 1
name: "remote"
type: "remote"
```
* _name_: the base name used when generating compressed files and identifying log lines
* _type_: specify the type of profile

**dirs:**

* _dirs_: is a list of directories to backup.
  * _path_: root of the path to backup.
  * _exclude_: a list of glob patterns of files to exclude from the backup.
  * _name_: Only used in sftpsync, specify the name of the profile to pull

example:
```
dirs:
  - path: "relative/path"
    exclude:
      - "*.log"
  - path: "/backup/service2"
```

> NOTE: if connecting to a sftp jail (sftpsync) the path needs to account for the jail root,
> E.g. if your jail is in /var/backups/content /that makes your new root /backups hence you need to put in path
> /content

**dbs:**

* _dbs_: list of databases to backup.
  * _dbname_: database name
  * _type_: database type, at the moment only mysql is supported, either directly or connecting to a docker container.
  * _user_: database user to login to the db. Leave empty to let the tool try to get access.
  * _password_: database user to login to the db. Leave empty to let the tool try to get access.
  * _containerName_: the docker container name to run the db dump on

>Note: goback will try to get root credentials for mysql from common locations like /etc/my.cnf a d fallback 
> to socket login

> IMPORTANT: mysql/maraibd uses mysqldump to add databases to the backup, 
> this needs to be installed otherwise it will fail.

example:
```
dbs:
  - name: dbname
    type: mysql
    user: user
    password: pw
    containerName: container
```

**ssh:**

* _ssh_: details about ssh/sftp connection
  * _type_ [ password | sshkey | sshagent ]: how to login to the remote server.
  * _host_: ssh host
  * _port_: ssh port
  * _user_: ssh user
  * _password_: plain text ssh password, used if type is sshPassword
  * _privateKey_: path to a private key, used if type is sshKey
  * _passPhrase_: plain text pass phrase to the private key
    
example:
```
ssh:
    type: password
    host: bla.ble.com
    port: 22
    user: user
    password: pw
```

**destination:**

* _destination_: details about the backup files destination
  * _path_: local path where backup files are created
  * _keep_: how many older backups to keep for this profile, set to -1 to disable deletion.
  * _owner_: change the owner of the resulting backup file
  * _mode_: change the mode of the resulting backup file

example:
```
destination:
  path: /backups
  keep: 3
  owner: "ble"
  mode : "0600"

```

**notify:**

* _notify_: optional setting to send an email per profile
  * _to_: list of email addresses to notify
  * _host_: email server host
  * _port_: email sever port
  * _user_: email server user to login
  * _password_: password for that user on the email server
  * _from_: email From address

example:
```
notify:
  host: smtp.mail.com
  port: 587
  to:
    - mail1@mail.com
    - mail2@mail.com
  user: mail@mails.com
  password: 1234

```


## Roadmap

* backup git repositories
* backuo github orgs
* allow to use envs placeholders in profiles, e.g. for secrets
* exclude folders that contain .nobackup
* improve the expurge rules to keep N yearly, monthly etc

#### TODO
* use systemd timers instead of cron
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

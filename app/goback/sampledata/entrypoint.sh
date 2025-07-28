#!/bin/bash
set -e

# create a user called pwuser protected by password
if [[ ! -z "${PW_USER}" ]]; then
  echo "creating pwuser"
  useradd -m -p "${PW_USER}" -s /bin/bash pwuser
fi

# grant pwuser passwordless sudo access to mysqldump
if [[ ! -z "${PW_USER}" ]]; then
  echo "pwuser ALL=(ALL) NOPASSWD: /usr/local/bin/mysqldump" >> /etc/sudoers
  echo "pwuser ALL=(ALL) NOPASSWD: /usr/local/bin/goback" >> /etc/sudoers
fi


# create a user called privkey protected by ssh key
if [[ ! -z "${SHH_KEY_USER}" ]]; then
  echo "creating privkey"
  useradd -m -s /bin/bash privkey
  mkdir -p /home/privkey/.ssh/
  echo "${SHH_KEY_USER}" > /home/privkey/.ssh/authorized_keys
  chmod 0700 /home/privkey/.ssh/authorized_keys
  chown privkey:privkey -R /home/privkey/.ssh/
fi

# starting ssh-server
exec /usr/sbin/sshd -D -e
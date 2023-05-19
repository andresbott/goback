#!/bin/bash
# post install  script


# Declare an array of string with type
declare -a exec=(
"/usr/bin/goback"
"/etc/cron.weekly/run-goback"
"/etc/cron.monthly/run-goback"
)

# Iterate the string array using for loop
for item in ${exec[@]}; do
   chown root:root "$item"
   chmod 755  "$item"
done

# clean old files
# Declare an array of string with type
declare -a del=(
"/etc/cron.weekly/goback.sh"
"/etc/cron.monthly/goback.sh"
)

# Iterate the string array using for loop
for item in ${del[@]}; do
  rm -f "$item"
done
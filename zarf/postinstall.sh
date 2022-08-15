#!/bin/bash
# post install  script


# Declare an array of string with type
declare -a exec=(
"/usr/bin/goback"
)

# Iterate the string array using for loop
for item in ${exec[@]}; do
   chown root:root "$item"
   chmod 755  "$item"
done


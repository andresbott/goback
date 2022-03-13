#!/bin/bash

echo "mysqldump mock binary, params: $@"
# exit with failure if the user (second argument) equals fail
if [ "$2" = "fail" ]; then
   exit 1
fi
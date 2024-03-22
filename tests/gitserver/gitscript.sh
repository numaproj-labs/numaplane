#!/bin/bash

#This script is a cgi script that creates the repo using http api
#curl -X POST -d "name=my_new" http://localhost:8080/create {"status": "success", "message": "Repository 'my_new' created successfully."}


read -r repo_name
repo_name="${repo_name#*=}"
repo_name=$(echo -e $(echo "$repo_name" | sed "s/+/ /g;s/%\(..\)/\\\\x\1/g;"))
repo_dir="/var/www/git/${repo_name}.git"
if  mkdir -p "$repo_dir" && git init --bare "$repo_dir"  > /dev/null 2>&1; then
    chown -R www-data:www-data "$repo_dir" /dev/null 2>&1
    echo "Content-Type: application/json"
        echo ""
        echo "{\"status\": \"success\", \"message\": \"Repository '${repo_name}' created successfully.\"}"
    else
        echo "Content-Type: application/json"
        echo ""
        echo "{\"status\": \"error\", \"message\": \"Error: Failed to create repository.\"}"
fi
exit 0

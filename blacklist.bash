#!/bin/bash

# if url is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <url>"
    exit 1
fi

URL=$1

# sqlite database file
DB_FILE="db.db"

# blacklist the URL
SQL="UPDATE cache SET blacklist = 1 WHERE url = '$URL';"

# execute command 
sqlite3 $DB_FILE "$SQL"

echo "URL '$URL' has been blacklisted."
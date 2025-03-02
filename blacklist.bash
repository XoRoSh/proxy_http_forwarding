#!/bin/bash

# Check if URL is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <url>"
    exit 1
fi

URL=$1

# SQLite database file
DB_FILE="db.db"

# SQL command to blacklist the URL
SQL="UPDATE cache SET blacklist = 1 WHERE url = '$URL';"

# Execute the SQL command
sqlite3 $DB_FILE "$SQL"

echo "URL '$URL' has been blacklisted."
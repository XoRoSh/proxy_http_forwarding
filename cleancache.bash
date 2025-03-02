#!/bin/bash

# Database file path
DB_FILE="/Users/levganzha/Golang/go_proxy_http_forwarding/db.db"

# TTL in seconds 
TTL=5

# SQLite command to delete cache entries older than the TTL
sqlite3 $DB_FILE <<EOF
DELETE FROM cache WHERE timestamp <= datetime('now', '-$TTL seconds');
EOF

echo "Cache cleaned successfully."
#!/bin/bash

# Run proxy.go in the background
go run proxy.go &

# Run cleancache.bash every 5 seconds
while true; do
    ./cleancache.bash
    sleep 5
done
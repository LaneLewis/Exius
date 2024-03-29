#!/bin/sh

echo "Running Webdav Bash"
nohup rclone serve webdav $CONFIGNAME:/ --addr :8081 --dir-cache-time 1m0s --poll-interval 30s&
nohup rclone rcd --rc-web-gui --rc-baseurl admin --rc-user admin --rc-pass $ADMINKEY --rc-addr :8082 --rc-web-gui-no-open-browser & 
/rclone-proxy

#!/bin/bash
touch /var/log/auth.log
touch /var/log/apache2/access.log
touch /var/log/apache2/error.log
# Starting SSH service
service ssh start
tail -F /var/log/apache2/* /var/log/auth.log &

# Run Apache2 in the foreground is important
apache2ctl -D FOREGROUND

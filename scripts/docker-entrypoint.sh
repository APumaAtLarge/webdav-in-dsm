#!/bin/sh
set -eu

chmod -R 777 /var/www/data /var/log/nginx
nginx -t

exec "$@"

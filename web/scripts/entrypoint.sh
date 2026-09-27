#!/bin/sh
# Drop to nobody after the push-subscription volume is writable.
set -eu
if [ -d /data ]; then
  chown nobody:nobody /data 2>/dev/null || true
fi
if [ "$(id -u)" = "0" ]; then
  exec su -m nobody -s /bin/sh -c 'exec /usr/local/bin/lcc-live'
fi
exec /usr/local/bin/lcc-live

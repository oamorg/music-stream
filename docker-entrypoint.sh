#!/bin/sh
set -eu

mkdir -p /srv/media
chown -R appuser:appuser /srv/media

exec su-exec appuser /usr/local/bin/app "$@"

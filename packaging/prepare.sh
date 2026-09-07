#!/bin/sh
set -eu

project_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
asset_dir="$project_dir/packaging/.generated"

mkdir -p "$asset_dir"
gzip -9cn "$project_dir/packaging/external-dns-dnsmasq-webhook.8" \
	>"$asset_dir/external-dns-dnsmasq-webhook.8.gz"

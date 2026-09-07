#!/bin/sh
set -eu

version="${1:?release version is required}"
project_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
asset_dir="$project_dir/packaging/debian/.generated"

mkdir -p "$asset_dir"
sed "s/@VERSION@/$version/g" "$project_dir/packaging/debian/changelog" \
	>"$asset_dir/changelog.Debian"
gzip -9nf "$asset_dir/changelog.Debian"
sed "s/@VERSION@/$version/g" "$project_dir/packaging/debian/external-dns-dnsmasq-webhook.8" \
	>"$asset_dir/external-dns-dnsmasq-webhook.8"
gzip -9nf "$asset_dir/external-dns-dnsmasq-webhook.8"

#!/bin/sh
# First installation only; an existing maolv is never replaced.
set -eu
fail() { printf '%s\n' "$*" >&2; exit 1; }
if command -v maolv >/dev/null 2>&1; then exec maolv version --json; fi
target="$HOME/.local/bin/maolv"
if [ -e "$target" ] || [ -L "$target" ]; then exec "$target" version --json; fi
version=${MAOLV_VERSION:-v0.1.0}
case "$version" in *[!v0-9.]*) fail 'MAOLV_VERSION must be a release tag such as v0.1.0';; esac
printf '%s\n' "$version" | LC_ALL=C grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' || fail 'MAOLV_VERSION must be a release tag such as v0.1.0'
case "$(uname -s)" in Darwin) platform=darwin;; Linux) platform=linux;; *) fail 'Unsupported system';; esac
case "$(uname -m)" in x86_64|amd64) architecture=amd64;; arm64|aarch64) architecture=arm64;; *) fail 'Unsupported architecture';; esac
command -v curl >/dev/null 2>&1 || fail 'curl is required'
if command -v sha256sum >/dev/null 2>&1; then hash=sha256sum; elif command -v shasum >/dev/null 2>&1; then hash=shasum; else fail 'sha256sum or shasum is required'; fi
asset="maolv_${version}_${platform}_${architecture}"
base="https://github.com/jicheng1014/maolv-cli/releases/download/$version"
work=$(mktemp -d)
staged=
trap 'rm -rf "$work"; [ -z "$staged" ] || rm -f "$staged"' EXIT HUP INT TERM
download() {
  curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fLsS --max-redirs 5 --connect-timeout 15 --max-time 180 --max-filesize "$3" "$base/$1" -o "$2"
  [ "$(wc -c < "$2" | tr -d ' ')" -le "$3" ] || fail 'Download exceeds size limit'
}
download checksums.txt "$work/checksums.txt" 65536
expected=$(awk -v name="$asset" '$2 == name { count++; value=$1 } END { if (count == 1) print value; else exit 1 }' "$work/checksums.txt") || fail 'Missing or duplicate checksum'
printf '%s\n' "$expected" | LC_ALL=C grep -Eq '^[0-9a-f]{64}$' || fail 'Invalid SHA256'
download "$asset" "$work/maolv" 67108864
if [ "$hash" = sha256sum ]; then actual=$(sha256sum "$work/maolv" | cut -d ' ' -f 1); else actual=$(shasum -a 256 "$work/maolv" | cut -d ' ' -f 1); fi
[ "$actual" = "$expected" ] || fail 'Checksum mismatch; installation stopped'
chmod 755 "$work/maolv"
"$work/maolv" version --json > "$work/version.json"
reported=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$work/version.json")
[ "$reported" = "$version" ] || fail 'Downloaded binary version does not match release tag'
mkdir -p "$(dirname "$target")"
staged=$(mktemp "$(dirname "$target")/.maolv-install.XXXXXX")
cp "$work/maolv" "$staged"
chmod 755 "$staged"
# Atomic no-clobber install, including when two installers run concurrently.
ln "$staged" "$target"
cat "$work/version.json"
printf 'Installed %s at %s\n' "$version" "$target"
printf '%s\n' 'For this terminal: export PATH="$HOME/.local/bin:$PATH"'

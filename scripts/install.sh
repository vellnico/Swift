#!/usr/bin/env bash
# Swift installer.
#   bash <(curl -fsSL https://raw.githubusercontent.com/vellnico/Swift/main/scripts/install.sh)
# Flags: --update, --uninstall [--purge], --version <tag>, --domain <d>,
#        --address <ip>, --non-interactive, --no-bbr, --from-dir <dir>
# Running it again on an installed server changes nothing and keeps the password.
set -euo pipefail

REPO="${SWIFT_REPO:-vellnico/Swift}"
BIN=/usr/local/bin/swift
LIB=/usr/local/lib/swift
ETC=/etc/swift
DATA=/var/lib/swift
UNIT=/etc/systemd/system/swift.service
SYSCTL=/etc/sysctl.d/99-swift.conf

MODE=install
VERSION=latest
DOMAIN=""
ADDRESS=""
INTERACTIVE=1
BBR=1
PURGE=0
FROM_DIR=""
TMP=""

say() { printf '%s\n' "$*"; }
warn() { printf 'swift: %s\n' "$*" >&2; }
die() {
	warn "$*"
	exit 1
}
cleanup() { if [ -n "$TMP" ]; then rm -rf "$TMP"; fi; }
trap cleanup EXIT

usage() {
	sed -n '2,6p' "$0" | sed 's/^# \{0,1\}//'
	exit "${1:-0}"
}

parse_args() {
	while [ $# -gt 0 ]; do
		case "$1" in
		--update) MODE=update ;;
		--uninstall) MODE=uninstall ;;
		--purge) PURGE=1 ;;
		--version) VERSION="${2:?--version needs a tag}"; shift ;;
		--domain) DOMAIN="${2:?--domain needs a name}"; shift ;;
		--address) ADDRESS="${2:?--address needs an IP}"; shift ;;
		--non-interactive) INTERACTIVE=0 ;;
		--no-bbr) BBR=0 ;;
		--from-dir) FROM_DIR="${2:?--from-dir needs a directory}"; shift ;;
		-h | --help) usage 0 ;;
		*) warn "unknown flag $1"; usage 2 ;;
		esac
		shift
	done
}

check_system() {
	[ "$(id -u)" -eq 0 ] || die "run as root (sudo -i, then the command again)"
	[ -r /etc/os-release ] || die "cannot detect the OS: /etc/os-release is missing"
	# shellcheck disable=SC1091
	. /etc/os-release
	local major="${VERSION_ID%%.*}"
	case "${ID:-}" in
	debian) [ "${major:-0}" -ge 11 ] || die "Debian 11 or newer is required" ;;
	ubuntu) [ "${major:-0}" -ge 20 ] || die "Ubuntu 20.04 or newer is required" ;;
	*) die "only Debian 11+ and Ubuntu 20.04+ are supported (found ${ID:-unknown})" ;;
	esac
	case "$(uname -m)" in
	x86_64 | amd64) ARCH=amd64 XRAY_ARCH=64 ;;
	aarch64 | arm64) ARCH=arm64 XRAY_ARCH=arm64-v8a ;;
	*) die "unsupported architecture $(uname -m); amd64 and arm64 are supported" ;;
	esac
	[ -d /run/systemd/system ] || die "systemd is required"
}

ensure_tools() {
	local missing=()
	for t in curl unzip tar sha256sum; do
		command -v "$t" >/dev/null 2>&1 || missing+=("$t")
	done
	if [ ${#missing[@]} -gt 0 ]; then
		say "Installing ${missing[*]}..."
		DEBIAN_FRONTEND=noninteractive apt-get update -qq
		DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl unzip tar coreutils ca-certificates >/dev/null
	fi
}

fetch() { curl -fsSL --retry 3 -o "$2" "$1"; }

# download_swift leaves the verified binary in $TMP/swift.
download_swift() {
	if [ -n "$FROM_DIR" ]; then
		install -m 0755 "$FROM_DIR/swift" "$TMP/swift"
		return
	fi
	local tag="$VERSION"
	if [ "$tag" = latest ]; then
		tag="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")"
		tag="${tag##*/}"
		if [ -z "$tag" ] || [ "$tag" = latest ]; then die "cannot find the latest release of $REPO"; fi
	fi
	local file="swift_${tag#v}_linux_${ARCH}.tar.gz"
	local base="https://github.com/$REPO/releases/download/$tag"
	say "Downloading Swift $tag..."
	fetch "$base/$file" "$TMP/$file"
	fetch "$base/checksums.txt" "$TMP/checksums.txt"
	(cd "$TMP" && grep " ${file}\$" checksums.txt | sha256sum -c --status) || die "SHA256 of $file does not match checksums.txt"
	tar -xzf "$TMP/$file" -C "$TMP" swift
}

# download_xray leaves xray and geo files in $TMP/xray/, pinned to Swift's version.
download_xray() {
	mkdir -p "$TMP/xray"
	if [ -n "$FROM_DIR" ]; then
		cp "$FROM_DIR/xray/xray" "$FROM_DIR/xray/geoip.dat" "$FROM_DIR/xray/geosite.dat" "$TMP/xray/"
		return
	fi
	local ver zip want got
	ver="$("$TMP/swift" version --xray)"
	zip="Xray-linux-${XRAY_ARCH}.zip"
	say "Downloading Xray $ver..."
	fetch "https://github.com/XTLS/Xray-core/releases/download/v$ver/$zip" "$TMP/$zip"
	fetch "https://github.com/XTLS/Xray-core/releases/download/v$ver/$zip.dgst" "$TMP/$zip.dgst"
	want="$(sed -n 's/^SHA2-256= *//p' "$TMP/$zip.dgst" | tr -d '[:space:]' | tr 'A-F' 'a-f')"
	got="$(sha256sum "$TMP/$zip" | cut -d' ' -f1)"
	if [ -z "$want" ] || [ "$want" != "$got" ]; then die "SHA256 of $zip does not match its digest"; fi
	unzip -oq "$TMP/$zip" xray geoip.dat geosite.dat -d "$TMP/xray"
}

install_binaries() {
	install -d -m 0755 "$LIB"
	install -m 0755 "$TMP/swift" "$BIN"
	install -m 0755 "$TMP/xray/xray" "$LIB/xray"
	install -m 0644 "$TMP/xray/geoip.dat" "$TMP/xray/geosite.dat" "$LIB/"
}

write_config() {
	install -d -m 0755 "$ETC"
	install -d -m 0700 "$DATA"
	if [ ! -f "$ETC/swift.toml" ]; then
		cat >"$ETC/swift.toml" <<EOF
# Swift configuration. Product settings live in the panel; these are paths.
data_dir = "$DATA"
lib_dir = "$LIB"
EOF
		chmod 0644 "$ETC/swift.toml"
	fi
}

detect_address() {
	[ -n "$ADDRESS" ] && return
	ADDRESS="$(curl -fsS4 --max-time 5 https://api.ipify.org 2>/dev/null || true)"
	if [ -z "$ADDRESS" ]; then
		ADDRESS="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") print $(i + 1)}')"
	fi
	[ -n "$ADDRESS" ] || die "cannot detect the server IP; pass --address <ip>"
}

ask_domain() {
	[ -n "$DOMAIN" ] && return
	[ "$INTERACTIVE" -eq 1 ] || return 0
	local answer=""
	read -r -p "Есть домен, указывающий на этот сервер? [y/N] " answer </dev/tty || true
	case "$answer" in
	y | Y | yes | д | Д | да) read -r -p "Домен: " DOMAIN </dev/tty || true ;;
	esac
}

check_domain() {
	[ -n "$DOMAIN" ] || return 0
	local ips
	ips="$(getent ahostsv4 "$DOMAIN" | awk '{print $1}' | sort -u | tr '\n' ' ')"
	case " $ips " in
	*" $ADDRESS "*) ;;
	*)
		warn "the A record of $DOMAIN points to '${ips:-nothing}', not to $ADDRESS"
		if [ "$INTERACTIVE" -eq 1 ]; then
			die "fix the DNS record and run the installer again"
		fi
		warn "continuing without a domain"
		DOMAIN=""
		;;
	esac
}

enable_bbr() {
	[ "$BBR" -eq 1 ] || return 0
	modprobe tcp_bbr 2>/dev/null || true
	printf 'net.core.default_qdisc = fq\nnet.ipv4.tcp_congestion_control = bbr\n' >"$SYSCTL"
	sysctl --system >/dev/null 2>&1 || warn "could not apply BBR settings"
}

open_ports() {
	local proto port
	if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
		while read -r proto port; do
			ufw allow "$port/$proto" >/dev/null
		done < <("$BIN" ports)
		say "Opened ports in ufw."
	elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
		while read -r proto port; do
			firewall-cmd --permanent --add-port="$port/$proto" >/dev/null
		done < <("$BIN" ports)
		firewall-cmd --reload >/dev/null
		say "Opened ports in firewalld."
	fi
}

write_unit() {
	cat >"$UNIT" <<EOF
[Unit]
Description=Swift VPN panel
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=$BIN serve -config $ETC/swift.toml
Restart=always
RestartSec=3
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
	systemctl daemon-reload
}

print_credentials() {
	local out url login pass
	out="$1"
	url="$(printf '%s\n' "$out" | sed -n 's/^ *URL: *//p')"
	login="$(printf '%s\n' "$out" | sed -n 's/^ *Login: *//p')"
	pass="$(printf '%s\n' "$out" | sed -n 's/^ *Password: *//p')"
	say ""
	say "  ┌──────────────────────────────────────────────────────────────┐"
	say "  │  Swift установлен                                            │"
	say "  └──────────────────────────────────────────────────────────────┘"
	say ""
	say "    Панель:  $url"
	say "    Логин:   $login"
	say "    Пароль:  $pass"
	say ""
	say "    Сохраните пароль: он показан один раз."
	say "    Новый пароль: swift admin reset"
	say ""
}

do_install() {
	check_system
	ensure_tools
	TMP="$(mktemp -d)"
	local fresh=1
	[ -x "$BIN" ] && "$BIN" admin show -config "$ETC/swift.toml" >/dev/null 2>&1 && fresh=0
	download_swift
	download_xray
	if [ -f "$UNIT" ]; then systemctl stop swift 2>/dev/null || true; fi
	install_binaries
	write_config
	detect_address
	ask_domain
	check_domain
	local args=(-config "$ETC/swift.toml" -address "$ADDRESS")
	[ -n "$DOMAIN" ] && args+=(-domain "$DOMAIN")
	"$BIN" setup "${args[@]}"
	enable_bbr
	write_unit
	open_ports
	systemctl enable --now swift >/dev/null 2>&1
	systemctl restart swift
	if [ "$fresh" -eq 1 ] && ! "$BIN" admin show -config "$ETC/swift.toml" | grep -q '^Login: [a-z0-9]\{8\}$'; then
		print_credentials "$("$BIN" admin reset -config "$ETC/swift.toml")"
	else
		say "Swift is up to date and running."
		"$BIN" admin show -config "$ETC/swift.toml"
	fi
}

do_update() {
	check_system
	ensure_tools
	[ -x "$BIN" ] || die "Swift is not installed; run without --update"
	TMP="$(mktemp -d)"
	local backup
	backup="$DATA/backup-$(date +%Y%m%d-%H%M%S).db"
	"$BIN" db backup "$backup" -config "$ETC/swift.toml" >/dev/null
	say "Database backed up to $backup"
	download_swift
	download_xray
	systemctl stop swift
	install_binaries
	write_unit
	systemctl start swift
	say "Updated: $("$BIN" version)"
}

do_uninstall() {
	[ "$(id -u)" -eq 0 ] || die "run as root"
	if [ -f "$UNIT" ]; then
		systemctl disable --now swift >/dev/null 2>&1 || true
		rm -f "$UNIT"
		systemctl daemon-reload
	fi
	rm -f "$BIN" "$SYSCTL"
	rm -rf "$LIB"
	if [ "$PURGE" -eq 1 ]; then
		rm -rf "$ETC" "$DATA"
		say "Swift and all its data were removed."
	else
		say "Swift was removed. Data is kept in $DATA and $ETC (use --uninstall --purge to delete it)."
	fi
}

main() {
	parse_args "$@"
	case "$MODE" in
	install) do_install ;;
	update) do_update ;;
	uninstall) do_uninstall ;;
	esac
}

main "$@"

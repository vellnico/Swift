#!/usr/bin/env bash
# Swift installer.
#   bash <(curl -fsSL https://raw.githubusercontent.com/vellnico/Swift/main/scripts/install.sh)
# Flags: --update, --uninstall [--purge], --version <tag>, --domain <d>,
#        --address <ip>, --non-interactive, --no-bbr, --from-dir <dir>
# Running it again on an installed server changes nothing and keeps the password.
set -Eeuo pipefail

REPO="${SWIFT_REPO:-vellnico/Swift}"
BIN=/usr/local/bin/swift
LIB=/usr/local/lib/swift
ETC=/etc/swift
DATA=/var/lib/swift
UNIT=/etc/systemd/system/swift.service
SYSCTL=/etc/sysctl.d/99-swift.conf
LOG=/var/log/swift-install.log

MODE=install
VERSION=latest
DOMAIN=""
ADDRESS=""
INTERACTIVE=1
BBR=1
PURGE=0
FROM_DIR=""
TMP=""

# Character counts below must be in characters, not bytes.
if locale -a 2>/dev/null | grep -qiE '^c\.utf-?8$'; then export LC_ALL=C.UTF-8; fi

# --- interface ---------------------------------------------------------------
# The screen gets fd 3; everything else goes to $LOG once the run starts.
exec 3>&1
TTY=0
[ -t 1 ] && TTY=1
B="" D="" R=""
if [ "$TTY" -eq 1 ] && [ -z "${NO_COLOR:-}" ]; then
	B=$'\e[1m' D=$'\e[2m' R=$'\e[0m'
fi
PCT=0
LABEL=""
STATE=""
TICKER=""

ui() { printf '%s\n' "$*" >&3; }

header() {
	ui ""
	ui "  ${B}Swift${R}"
	ui "  ${D}$1${R}"
	ui ""
}

bar() {
	local width=30 fill
	fill=$(($1 * width / 100))
	printf '%s' "$B"
	[ "$fill" -gt 0 ] && printf '━%.0s' $(seq 1 "$fill")
	printf '%s' "$R$D"
	[ "$fill" -lt "$width" ] && printf '─%.0s' $(seq 1 $((width - fill)))
	printf '%s' "$R"
}

draw() { # percent label mark
	printf '\r\e[K  %s %s  %s%3d%%%s  %s' "$3" "$(bar "$1")" "$B" "$1" "$R" "$2" >&3
}

ticker() {
	trap - ERR
	set +e
	local frames=(⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏) i=0 pct label
	while :; do
		IFS='|' read -r pct label <"$STATE" 2>/dev/null
		draw "${pct:-0}" "${label:-}" "${frames[i % 10]}"
		i=$((i + 1))
		sleep 0.12
	done
}

progress_start() {
	if [ "$TTY" -eq 1 ]; then
		STATE="$(mktemp)"
		printf '0|\n' >"$STATE"
		printf '\e[?25l' >&3
		ticker &
		TICKER=$!
	fi
}

step() { # percent label
	PCT=$1 LABEL=$2
	if [ -n "$STATE" ]; then
		printf '%s|%s\n' "$PCT" "$LABEL" >"$STATE.next" && mv "$STATE.next" "$STATE"
	else
		printf '  %3d%%  %s\n' "$PCT" "$LABEL" >&3
	fi
	printf '\n== %s%% %s\n' "$PCT" "$LABEL"
}

progress_stop() { # final mark
	if [ -n "$TICKER" ]; then
		kill "$TICKER" 2>/dev/null || true
		wait "$TICKER" 2>/dev/null || true
		TICKER=""
		draw "$PCT" "$LABEL" "${1:-✓}"
		printf '\n\e[?25h' >&3
		rm -f "$STATE" "$STATE.next"
		STATE=""
	fi
}

die() {
	progress_stop "✕"
	ui ""
	ui "  ${B}✕  $1${R}"
	[ $# -gt 1 ] && ui "     $2"
	[ -s "$LOG" ] && ui "     ${D}Подробности: $LOG${R}"
	ui ""
	exit 1
}

on_error() {
	local code=$?
	# a failing $(...) reports in its parent
	[ "$BASHPID" = "$$" ] || exit "$code"
	progress_stop "✕"
	ui ""
	ui "  ${B}✕  Не выполнен шаг «${LABEL:-подготовка}»${R}"
	if [ -s "$LOG" ]; then
		ui ""
		tail -n 8 "$LOG" | sed "s/^/     ${D}/; s/\$/${R}/" >&3
		ui ""
		ui "     ${D}Весь журнал: $LOG${R}"
	fi
	ui ""
	exit "$code"
}
trap on_error ERR

cleanup() {
	local code=$?
	progress_stop "✕"
	if [ -n "$TMP" ]; then rm -rf "$TMP"; fi
	exit "$code"
}
trap cleanup EXIT

# box prints a framed block: a bold title, then "label|value" rows ("" is a blank row).
box() {
	local title=$1 width=${#1} row label value
	shift
	for row in "$@"; do
		label=${row%%|*} value=${row#*|}
		[ $((9 + ${#value})) -gt "$width" ] && width=$((9 + ${#value}))
	done
	width=$((width + 6))
	local hr pad
	hr=$(printf '─%.0s' $(seq 1 "$width"))
	ui "  ┌${hr}┐"
	ui "  │$(printf '%*s' "$width" '')│"
	pad=$(printf '%*s' $((width - 3 - ${#title})) '')
	ui "  │   ${B}${title}${R}${pad}│"
	ui "  │$(printf '%*s' "$width" '')│"
	for row in "$@"; do
		if [ -z "$row" ]; then
			ui "  │$(printf '%*s' "$width" '')│"
			continue
		fi
		label=${row%%|*} value=${row#*|}
		pad=$(printf '%*s' $((width - 12 - ${#value})) '')
		ui "  │   ${D}${label}$(printf '%*s' $((9 - ${#label})) '')${R}${value}${pad}│"
	done
	ui "  │$(printf '%*s' "$width" '')│"
	ui "  └${hr}┘"
}

start_log() {
	[ "$(id -u)" -eq 0 ] || die "Нужны права root" "Выполните sudo -i и запустите команду снова."
	umask 077
	: >"$LOG"
	umask 022
	exec >>"$LOG" 2>&1
	printf 'swift installer, %s, mode %s\n' "$(date -u '+%F %T UTC')" "$MODE"
}

# --- arguments ---------------------------------------------------------------

usage() {
	sed -n '2,6p' "$0" | sed 's/^# \{0,1\}//' >&3
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
		*) ui "Неизвестный флаг: $1"; usage 2 ;;
		esac
		shift
	done
}

# --- steps -------------------------------------------------------------------

check_system() {
	[ -r /etc/os-release ] || die "Не удалось определить систему" "Нет файла /etc/os-release."
	# shellcheck disable=SC1091
	. /etc/os-release
	local major="${VERSION_ID%%.*}"
	case "${ID:-}" in
	debian) [ "${major:-0}" -ge 11 ] || die "Нужен Debian 11 или новее" ;;
	ubuntu) [ "${major:-0}" -ge 20 ] || die "Нужна Ubuntu 20.04 или новее" ;;
	*) die "Поддерживаются Debian 11+ и Ubuntu 20.04+" "Сейчас: ${PRETTY_NAME:-${ID:-неизвестно}}." ;;
	esac
	case "$(uname -m)" in
	x86_64 | amd64) ARCH=amd64 XRAY_ARCH=64 ;;
	aarch64 | arm64) ARCH=arm64 XRAY_ARCH=arm64-v8a ;;
	*) die "Архитектура $(uname -m) не поддерживается" "Подойдут amd64 и arm64." ;;
	esac
	[ -d /run/systemd/system ] || die "Нужен systemd"
}

ensure_tools() {
	local missing=()
	for t in curl unzip tar sha256sum; do
		command -v "$t" >/dev/null 2>&1 || missing+=("$t")
	done
	if [ ${#missing[@]} -gt 0 ]; then
		DEBIAN_FRONTEND=noninteractive apt-get update -q
		DEBIAN_FRONTEND=noninteractive apt-get install -y -q curl unzip tar coreutils ca-certificates
	fi
}

fetch() {
	curl -fsSL --retry 3 -o "$2" "$1" ||
		die "Не удалось скачать ${1##*/}" "Проверьте, что сервер видит github.com, и запустите установку снова."
}

# download_swift leaves the verified binary in $TMP/swift.
download_swift() {
	if [ -n "$FROM_DIR" ]; then
		install -m 0755 "$FROM_DIR/swift" "$TMP/swift"
		return
	fi
	local tag="$VERSION"
	if [ "$tag" = latest ]; then
		tag="$(curl -fsSLI --retry 3 -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" || true)"
		tag="${tag##*/}"
		if [ -z "$tag" ] || [ "$tag" = latest ]; then
			die "Не найден релиз Swift" "Проверьте, что сервер видит github.com."
		fi
	fi
	local file="swift_${tag#v}_linux_${ARCH}.tar.gz"
	local base="https://github.com/$REPO/releases/download/$tag"
	fetch "$base/$file" "$TMP/$file"
	fetch "$base/checksums.txt" "$TMP/checksums.txt"
	if ! (cd "$TMP" && grep " ${file}\$" checksums.txt | sha256sum -c); then
		die "Контрольная сумма Swift не совпала" "Файл повреждён при загрузке. Запустите установку снова."
	fi
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
	fetch "https://github.com/XTLS/Xray-core/releases/download/v$ver/$zip" "$TMP/$zip"
	fetch "https://github.com/XTLS/Xray-core/releases/download/v$ver/$zip.dgst" "$TMP/$zip.dgst"
	want="$(sed -n 's/^SHA2-256= *//p' "$TMP/$zip.dgst" | tr -d '[:space:]' | tr 'A-F' 'a-f')"
	got="$(sha256sum "$TMP/$zip" | cut -d' ' -f1)"
	if [ -z "$want" ] || [ "$want" != "$got" ]; then
		die "Контрольная сумма Xray не совпала" "Файл повреждён при загрузке. Запустите установку снова."
	fi
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
	if [ -n "$ADDRESS" ]; then return; fi
	ADDRESS="$(curl -fsS4 --max-time 5 https://api.ipify.org 2>/dev/null || true)"
	if [ -z "$ADDRESS" ]; then
		ADDRESS="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") print $(i + 1)}' || true)"
	fi
	[ -n "$ADDRESS" ] || die "Не удалось определить IP сервера" "Укажите его флагом --address <ip>."
}

ask_domain() {
	if [ -n "$DOMAIN" ] || [ "$INTERACTIVE" -eq 0 ]; then return; fi
	local answer=""
	printf '  Есть домен, который указывает на этот сервер? %s[y/N]%s ' "$D" "$R" >&3
	read -r answer </dev/tty || true
	case "$answer" in
	y | Y | yes | д | Д | да | Да)
		printf '  Домен: ' >&3
		read -r DOMAIN </dev/tty || true
		;;
	esac
	ui ""
}

check_domain() {
	[ -n "$DOMAIN" ] || return 0
	local ips
	ips="$(getent ahostsv4 "$DOMAIN" | awk '{print $1}' | sort -u | tr '\n' ' ' || true)"
	case " $ips " in
	*" $ADDRESS "*) ;;
	*)
		if [ "$INTERACTIVE" -eq 1 ]; then
			die "Домен $DOMAIN не указывает на этот сервер" "A-запись: ${ips:-нет}, а IP сервера $ADDRESS. Исправьте DNS и запустите установку снова."
		fi
		printf 'domain %s points to %s, not %s; continuing without it\n' "$DOMAIN" "${ips:-nothing}" "$ADDRESS"
		DOMAIN=""
		;;
	esac
}

enable_bbr() {
	[ "$BBR" -eq 1 ] || return 0
	modprobe tcp_bbr || true
	printf 'net.core.default_qdisc = fq\nnet.ipv4.tcp_congestion_control = bbr\n' >"$SYSCTL"
	sysctl --system || printf 'could not apply BBR settings\n'
}

open_ports() {
	local proto port
	if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
		while read -r proto port; do
			ufw allow "$port/$proto"
		done < <("$BIN" ports -config "$ETC/swift.toml")
	elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
		while read -r proto port; do
			firewall-cmd --permanent --add-port="$port/$proto"
		done < <("$BIN" ports -config "$ETC/swift.toml")
		firewall-cmd --reload
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

start_service() {
	systemctl enable swift
	systemctl restart swift
	local i
	for i in $(seq 1 20); do
		systemctl is-active --quiet swift && return 0
		sleep 0.5
	done
	journalctl -u swift --no-pager -n 30 || true
	die "Панель не запустилась" "Причину покажет journalctl -u swift."
}

port_list() {
	"$BIN" ports -config "$ETC/swift.toml" | awk '{printf "%s%s/%s", (NR > 1 ? "  " : ""), $2, $1}'
}

field() { printf '%s\n' "$1" | sed -n "s/^ *$2: *//p"; }

# --- modes -------------------------------------------------------------------

do_install() {
	header "Установка панели"
	start_log
	check_system
	ask_domain
	detect_address
	check_domain

	progress_start
	step 4 "Подготовка системы"
	ensure_tools
	TMP="$(mktemp -d)"
	local fresh=1
	if [ -x "$BIN" ] && "$BIN" admin show -config "$ETC/swift.toml" | grep -q '^Login: [a-z0-9]\{8\}$'; then fresh=0; fi
	step 12 "Загрузка Swift"
	download_swift
	step 34 "Загрузка Xray"
	download_xray
	step 58 "Установка файлов"
	if [ -f "$UNIT" ]; then systemctl stop swift || true; fi
	install_binaries
	write_config
	if [ -n "$DOMAIN" ]; then
		step 64 "Профили и сертификат Let's Encrypt"
	else
		step 64 "Профили и сертификат"
	fi
	local args=(-config "$ETC/swift.toml" -address "$ADDRESS")
	if [ -n "$DOMAIN" ]; then args+=(-domain "$DOMAIN"); fi
	"$BIN" setup "${args[@]}"
	step 84 "Настройка сети"
	enable_bbr
	write_unit
	open_ports
	step 92 "Запуск панели"
	start_service
	local creds=""
	[ "$fresh" -eq 1 ] && creds="$("$BIN" admin reset -config "$ETC/swift.toml")"
	step 100 "Готово"
	progress_stop
	ui ""

	if [ -n "$creds" ]; then
		box "Swift установлен" \
			"Панель|$(field "$creds" URL)" \
			"Логин|$(field "$creds" Login)" \
			"Пароль|$(field "$creds" Password)"
		ui ""
		ui "  Сохраните пароль: он показан один раз."
		ui "  ${D}Новый пароль выдаст команда swift admin reset.${R}"
	else
		local show
		show="$("$BIN" admin show -config "$ETC/swift.toml")"
		box "Swift уже установлен" \
			"Панель|$(field "$show" URL)" \
			"Логин|$(field "$show" Login)"
		ui ""
		ui "  Пароль прежний, данные и профили сохранены."
		ui "  ${D}Новый пароль выдаст команда swift admin reset.${R}"
	fi
	if [ -z "$DOMAIN" ]; then
		ui "  ${D}Браузер предупредит о самоподписанном сертификате: это ожидаемо.${R}"
	fi
	ui ""
	ui "  Если у хостера есть свой файрвол, откройте в нём порты:"
	ui "  ${B}$(port_list)${R}"
	ui ""
}

do_update() {
	header "Обновление"
	start_log
	check_system
	[ -x "$BIN" ] || die "Swift не установлен" "Запустите установщик без --update."
	progress_start
	step 5 "Резервная копия базы"
	ensure_tools
	TMP="$(mktemp -d)"
	local backup
	backup="$DATA/backup-$(date +%Y%m%d-%H%M%S).db"
	"$BIN" db backup "$backup" -config "$ETC/swift.toml"
	step 15 "Загрузка Swift"
	download_swift
	step 45 "Загрузка Xray"
	download_xray
	step 75 "Установка файлов"
	systemctl stop swift
	install_binaries
	write_unit
	step 90 "Запуск панели"
	start_service
	step 100 "Готово"
	progress_stop
	ui ""
	local v
	v="$("$BIN" version)"
	box "Swift обновлён" \
		"Swift|$(printf '%s\n' "$v" | awk '{print $2}')" \
		"Xray|$(printf '%s\n' "$v" | awk '{print $4}')" \
		"Копия БД|$backup"
	ui ""
}

do_uninstall() {
	header "Удаление"
	start_log
	progress_start
	step 30 "Остановка панели"
	if [ -f "$UNIT" ]; then
		systemctl disable --now swift || true
		rm -f "$UNIT"
		systemctl daemon-reload
	fi
	step 70 "Удаление файлов"
	rm -f "$BIN" "$SYSCTL"
	rm -rf "$LIB"
	if [ "$PURGE" -eq 1 ]; then rm -rf "$ETC" "$DATA"; fi
	step 100 "Готово"
	progress_stop
	ui ""
	if [ "$PURGE" -eq 1 ]; then
		ui "  ${B}Swift удалён вместе с данными.${R}"
	else
		ui "  ${B}Swift удалён.${R}"
		ui "  ${D}База и настройки остались в $DATA и $ETC.${R}"
		ui "  ${D}Удалить и их: флаги --uninstall --purge.${R}"
	fi
	ui ""
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

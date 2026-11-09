#!/usr/bin/env bash
# Swift installer.
#   bash <(curl -fsSL https://raw.githubusercontent.com/vellnico/Swift/main/scripts/install.sh)
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

# Names must not clash with /etc/os-release (VERSION, ID, NAME...), which check_system sources.
MODE=install
SWIFT_VERSION=latest
UI_LANG=""
DOMAIN=""
ADDRESS=""
INTERACTIVE=1
BBR=1
PURGE=0
FROM_DIR=""
TMP=""
TAG=""

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

# L prints the English or the Russian text depending on the chosen language.
L() {
	if [ "$UI_LANG" = ru ]; then printf '%s' "$2"; else printf '%s' "$1"; fi
}

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
	[ -s "$LOG" ] && ui "     ${D}$(L "Details" "Подробности"): $LOG${R}"
	ui ""
	exit 1
}

on_error() {
	local code=$?
	# a failing $(...) reports in its parent
	[ "$BASHPID" = "$$" ] || exit "$code"
	progress_stop "✕"
	ui ""
	ui "  ${B}✕  $(L "Step failed" "Не выполнен шаг"): ${LABEL:-$(L "preparation" "подготовка")}${R}"
	if [ -s "$LOG" ]; then
		ui ""
		tail -n 8 "$LOG" | sed "s/^/     ${D}/; s/\$/${R}/" >&3
		ui ""
		ui "     ${D}$(L "Full log" "Весь журнал"): $LOG${R}"
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
		[ $((11 + ${#value})) -gt "$width" ] && width=$((11 + ${#value}))
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
		pad=$(printf '%*s' $((width - 14 - ${#value})) '')
		ui "  │   ${D}${label}$(printf '%*s' $((11 - ${#label})) '')${R}${value}${pad}│"
	done
	ui "  │$(printf '%*s' "$width" '')│"
	ui "  └${hr}┘"
}

start_log() {
	[ "$(id -u)" -eq 0 ] || die "$(L "Root is required" "Нужны права root")" "$(L "Run sudo -i and start the command again." "Выполните sudo -i и запустите команду снова.")"
	umask 077
	: >"$LOG"
	umask 022
	exec >>"$LOG" 2>&1
	printf 'swift installer, %s, mode %s, language %s\n' "$(date -u '+%F %T UTC')" "$MODE" "$UI_LANG"
}

# --- arguments ---------------------------------------------------------------

# The text is inline: with bash <(curl ...) the script file cannot be read again.
usage() {
	cat >&3 <<'EOF'
Swift installer
  bash <(curl -fsSL https://raw.githubusercontent.com/vellnico/Swift/main/scripts/install.sh) [flags]

  --lang en|ru        installer and panel language (asked when omitted)
  --domain <name>     issue a Let's Encrypt certificate for the domain
  --address <ip>      public IP when it is not detected
  --version <tag>     install a specific release
  --update            update Swift and Xray, backing up the database first
  --uninstall         remove Swift; add --purge to remove data and settings too
  --no-bbr            leave TCP settings alone
  --non-interactive   ask nothing
EOF
	exit "${1:-0}"
}

parse_args() {
	while [ $# -gt 0 ]; do
		case "$1" in
		--update) MODE=update ;;
		--uninstall) MODE=uninstall ;;
		--purge) PURGE=1 ;;
		--version) SWIFT_VERSION="${2:?--version needs a tag}"; shift ;;
		--lang) UI_LANG="$(lang_code "${2:?--lang needs en or ru}")"; shift ;;
		--domain) DOMAIN="${2:?--domain needs a name}"; shift ;;
		--address) ADDRESS="${2:?--address needs an IP}"; shift ;;
		--non-interactive) INTERACTIVE=0 ;;
		--no-bbr) BBR=0 ;;
		--from-dir) FROM_DIR="${2:?--from-dir needs a directory}"; shift ;;
		-h | --help) usage 0 ;;
		*) ui "Unknown flag: $1"; usage 2 ;;
		esac
		shift
	done
}

# lang_code maps an answer to en or ru; anything unknown is English.
lang_code() {
	case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
	2 | ru | rus | russian | рус | русский | Русский) printf 'ru' ;;
	*) printf 'en' ;;
	esac
}

# choose_lang asks for the installer and panel language before anything else.
choose_lang() {
	[ -z "$UI_LANG" ] || return 0
	UI_LANG=en
	[ "$INTERACTIVE" -eq 1 ] || return 0
	local answer=""
	ui ""
	ui "  ${B}Language / Язык${R}"
	ui "  ${B}1${R}  English"
	ui "  ${B}2${R}  Русский"
	printf '  %s›%s ' "$D" "$R" >&3
	read -r answer </dev/tty || true
	UI_LANG="$(lang_code "$answer")"
}

# installed_lang reads the language chosen at install time (update and uninstall).
installed_lang() {
	[ -z "$UI_LANG" ] || return 0
	UI_LANG=en
	if [ -x "$BIN" ]; then
		local v
		v="$("$BIN" admin show -config "$ETC/swift.toml" 2>/dev/null | sed -n 's/^Language: *//p' || true)"
		if [ "$v" = ru ]; then UI_LANG=ru; fi
	fi
}

# --- steps -------------------------------------------------------------------

check_system() {
	[ -r /etc/os-release ] || die "$(L "Cannot detect the system" "Не удалось определить систему")" "$(L "/etc/os-release is missing." "Нет файла /etc/os-release.")"
	local id ver_id pretty
	# a subshell keeps os-release variables (VERSION among them) out of the installer
	# shellcheck disable=SC1091
	id="$(. /etc/os-release && printf '%s' "${ID:-}")"
	# shellcheck disable=SC1091
	ver_id="$(. /etc/os-release && printf '%s' "${VERSION_ID:-}")"
	# shellcheck disable=SC1091
	pretty="$(. /etc/os-release && printf '%s' "${PRETTY_NAME:-${ID:-unknown}}")"
	local major="${ver_id%%.*}"
	case "$major" in '' | *[!0-9]*) major=0 ;; esac
	case "$id" in
	debian) [ "$major" -ge 11 ] || die "$(L "Debian 11 or newer is required" "Нужен Debian 11 или новее")" ;;
	ubuntu) [ "$major" -ge 20 ] || die "$(L "Ubuntu 20.04 or newer is required" "Нужна Ubuntu 20.04 или новее")" ;;
	*) die "$(L "Supported systems: Debian 11+ and Ubuntu 20.04+" "Поддерживаются Debian 11+ и Ubuntu 20.04+")" "$(L "This one: $pretty." "Сейчас: $pretty.")" ;;
	esac
	case "$(uname -m)" in
	x86_64 | amd64) ARCH=amd64 XRAY_ARCH=64 ;;
	aarch64 | arm64) ARCH=arm64 XRAY_ARCH=arm64-v8a ;;
	*) die "$(L "Architecture $(uname -m) is not supported" "Архитектура $(uname -m) не поддерживается")" "$(L "amd64 and arm64 are supported." "Подойдут amd64 и arm64.")" ;;
	esac
	[ -d /run/systemd/system ] || die "$(L "systemd is required" "Нужен systemd")"
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
	curl -fsSL --retry 5 --retry-delay 3 -o "$2" "$1" ||
		die "$(L "Could not download ${1##*/}" "Не удалось скачать ${1##*/}")" "$(L "Make sure the server can reach github.com and run the installer again." "Проверьте, что сервер видит github.com, и запустите установку снова.")"
}

# release_tag sets TAG: --version as given (with a leading v), or the latest release.
release_tag() {
	TAG="$SWIFT_VERSION"
	if [ "$TAG" != latest ]; then
		case "$TAG" in v*) ;; *) TAG="v$TAG" ;; esac
		return
	fi
	TAG="$(curl -fsSLI --retry 3 -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" || true)"
	TAG="${TAG##*/}"
	case "$TAG" in
	v[0-9]*) ;;
	*) die "$(L "No Swift release found" "Не найден релиз Swift")" "$(L "Make sure the server can reach github.com." "Проверьте, что сервер видит github.com.")" ;;
	esac
}

# download_swift leaves the verified binary in $TMP/swift.
download_swift() {
	if [ -n "$FROM_DIR" ]; then
		install -m 0755 "$FROM_DIR/swift" "$TMP/swift"
		return
	fi
	release_tag
	local tag="$TAG"
	local file="swift_${tag#v}_linux_${ARCH}.tar.gz"
	local base="https://github.com/$REPO/releases/download/$tag"
	fetch "$base/$file" "$TMP/$file"
	fetch "$base/checksums.txt" "$TMP/checksums.txt"
	if ! (cd "$TMP" && grep " ${file}\$" checksums.txt | sha256sum -c); then
		die "$(L "Swift checksum mismatch" "Контрольная сумма Swift не совпала")" "$(L "The download is corrupted. Run the installer again." "Файл повреждён при загрузке. Запустите установку снова.")"
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
		die "$(L "Xray checksum mismatch" "Контрольная сумма Xray не совпала")" "$(L "The download is corrupted. Run the installer again." "Файл повреждён при загрузке. Запустите установку снова.")"
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
	[ -n "$ADDRESS" ] || die "$(L "Cannot detect the server IP" "Не удалось определить IP сервера")" "$(L "Pass it with --address <ip>." "Укажите его флагом --address <ip>.")"
}

ask_domain() {
	if [ -n "$DOMAIN" ] || [ "$INTERACTIVE" -eq 0 ]; then return; fi
	local answer=""
	printf '  %s %s[y/N]%s ' "$(L "Do you have a domain pointing at this server?" "Есть домен, который указывает на этот сервер?")" "$D" "$R" >&3
	read -r answer </dev/tty || true
	case "$answer" in
	y | Y | yes | Yes | YES | д | Д | да | Да | ДА)
		printf '  %s: ' "$(L "Domain" "Домен")" >&3
		read -r DOMAIN </dev/tty || true
		DOMAIN="$(printf '%s' "$DOMAIN" | tr -d '[:space:]')"
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
			die "$(L "Domain $DOMAIN does not point at this server" "Домен $DOMAIN не указывает на этот сервер")" "$(L "A record: ${ips:-none}, server IP: $ADDRESS. Fix DNS and run the installer again." "A-запись: ${ips:-нет}, а IP сервера $ADDRESS. Исправьте DNS и запустите установку снова.")"
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
	die "$(L "The panel did not start" "Панель не запустилась")" "$(L "journalctl -u swift shows the reason." "Причину покажет journalctl -u swift.")"
}

port_list() {
	"$BIN" ports -config "$ETC/swift.toml" | awk '{printf "%s%s/%s", (NR > 1 ? "  " : ""), $2, $1}'
}

field() { printf '%s\n' "$1" | sed -n "s/^ *$2: *//p"; }

# --- modes -------------------------------------------------------------------

do_install() {
	choose_lang
	header "$(L "Panel installation" "Установка панели")"
	start_log
	check_system
	ask_domain
	detect_address
	check_domain

	progress_start
	step 4 "$(L "Preparing the system" "Подготовка системы")"
	ensure_tools
	TMP="$(mktemp -d)"
	local fresh=1
	if [ -x "$BIN" ] && "$BIN" admin show -config "$ETC/swift.toml" | grep -q '^Login: [a-z0-9]\{8\}$'; then fresh=0; fi
	step 12 "$(L "Downloading Swift" "Загрузка Swift")"
	download_swift
	step 34 "$(L "Downloading Xray" "Загрузка Xray")"
	download_xray
	step 58 "$(L "Installing files" "Установка файлов")"
	if [ -f "$UNIT" ]; then systemctl stop swift || true; fi
	install_binaries
	write_config
	if [ -n "$DOMAIN" ]; then
		step 64 "$(L "Profiles and a Let's Encrypt certificate" "Профили и сертификат Let's Encrypt")"
	else
		step 64 "$(L "Profiles and certificate" "Профили и сертификат")"
	fi
	local args=(-config "$ETC/swift.toml" -address "$ADDRESS")
	# releases before v0.2.0 (--version) have no -lang; -h exits non-zero
	local help
	help="$("$BIN" setup -h 2>&1 || true)"
	case "$help" in *-lang*) args+=(-lang "$UI_LANG") ;; esac
	if [ -n "$DOMAIN" ]; then args+=(-domain "$DOMAIN"); fi
	"$BIN" setup "${args[@]}"
	step 84 "$(L "Network settings" "Настройка сети")"
	enable_bbr
	write_unit
	open_ports
	step 92 "$(L "Starting the panel" "Запуск панели")"
	start_service
	local creds=""
	if [ "$fresh" -eq 1 ]; then creds="$("$BIN" admin reset -config "$ETC/swift.toml")"; fi
	step 100 "$(L "Done" "Готово")"
	progress_stop
	ui ""

	if [ -n "$creds" ]; then
		box "$(L "Swift is installed" "Swift установлен")" \
			"$(L "Panel" "Панель")|$(field "$creds" URL)" \
			"$(L "Login" "Логин")|$(field "$creds" Login)" \
			"$(L "Password" "Пароль")|$(field "$creds" Password)"
		ui ""
		ui "  $(L "Save the password: it is shown only once." "Сохраните пароль: он показан один раз.")"
		ui "  ${D}$(L "swift admin reset issues a new one." "Новый пароль выдаст команда swift admin reset.")${R}"
	else
		local show
		show="$("$BIN" admin show -config "$ETC/swift.toml")"
		box "$(L "Swift is already installed" "Swift уже установлен")" \
			"$(L "Panel" "Панель")|$(field "$show" URL)" \
			"$(L "Login" "Логин")|$(field "$show" Login)"
		ui ""
		ui "  $(L "The password is unchanged; data and profiles are kept." "Пароль прежний, данные и профили сохранены.")"
		ui "  ${D}$(L "swift admin reset issues a new one." "Новый пароль выдаст команда swift admin reset.")${R}"
	fi
	if [ -z "$DOMAIN" ]; then
		ui "  ${D}$(L "The browser will warn about a self-signed certificate: this is expected." "Браузер предупредит о самоподписанном сертификате: это ожидаемо.")${R}"
	fi
	ui ""
	ui "  $(L "If your host has its own firewall, open these ports there:" "Если у хостера есть свой файрвол, откройте в нём порты:")"
	ui "  ${B}$(port_list)${R}"
	ui ""
}

do_update() {
	installed_lang
	header "$(L "Update" "Обновление")"
	start_log
	check_system
	[ -x "$BIN" ] || die "$(L "Swift is not installed" "Swift не установлен")" "$(L "Run the installer without --update." "Запустите установщик без --update.")"
	progress_start
	step 5 "$(L "Backing up the database" "Резервная копия базы")"
	ensure_tools
	TMP="$(mktemp -d)"
	local backup
	backup="$DATA/backup-$(date +%Y%m%d-%H%M%S).db"
	"$BIN" db backup "$backup" -config "$ETC/swift.toml"
	step 15 "$(L "Downloading Swift" "Загрузка Swift")"
	download_swift
	step 45 "$(L "Downloading Xray" "Загрузка Xray")"
	download_xray
	step 75 "$(L "Installing files" "Установка файлов")"
	systemctl stop swift
	install_binaries
	write_unit
	step 90 "$(L "Starting the panel" "Запуск панели")"
	start_service
	step 100 "$(L "Done" "Готово")"
	progress_stop
	ui ""
	local v
	v="$("$BIN" version)"
	box "$(L "Swift is updated" "Swift обновлён")" \
		"Swift|$(printf '%s\n' "$v" | awk '{print $2}')" \
		"Xray|$(printf '%s\n' "$v" | awk '{print $4}')" \
		"$(L "DB backup" "Копия БД")|$backup"
	ui ""
}

do_uninstall() {
	installed_lang
	header "$(L "Removal" "Удаление")"
	start_log
	progress_start
	step 30 "$(L "Stopping the panel" "Остановка панели")"
	if [ -f "$UNIT" ]; then
		systemctl disable --now swift || true
		rm -f "$UNIT"
		systemctl daemon-reload
	fi
	step 70 "$(L "Removing files" "Удаление файлов")"
	rm -f "$BIN" "$SYSCTL"
	rm -rf "$LIB"
	if [ "$PURGE" -eq 1 ]; then rm -rf "$ETC" "$DATA"; fi
	step 100 "$(L "Done" "Готово")"
	progress_stop
	ui ""
	if [ "$PURGE" -eq 1 ]; then
		ui "  ${B}$(L "Swift is removed together with its data." "Swift удалён вместе с данными.")${R}"
	else
		ui "  ${B}$(L "Swift is removed." "Swift удалён.")${R}"
		ui "  ${D}$(L "The database and settings remain in $DATA and $ETC." "База и настройки остались в $DATA и $ETC.")${R}"
		ui "  ${D}$(L "To remove them too: --uninstall --purge." "Удалить и их: флаги --uninstall --purge.")${R}"
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

# Sourcing the script (tests) defines the functions without running anything.
if [ "${BASH_SOURCE[0]:-$0}" = "$0" ]; then
	main "$@"
fi

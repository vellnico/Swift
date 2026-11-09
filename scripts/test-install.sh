#!/usr/bin/env bash
# Runs install.sh in a systemd container: clean install, repeated install,
# --update, --uninstall and --uninstall --purge.
#   scripts/test-install.sh debian:12
# Needs docker, bin/swift built for linux and .cache/xray from `make xray`.
set -euo pipefail

BASE="${1:-debian:12}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NAME="swift-install-test-$$"
IMAGE="swift-install-test:${BASE//[:\/]/-}"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	docker exec "$NAME" journalctl -u swift --no-pager -n 100 >&2 || true
	exit 1
}
ct() { docker exec "$NAME" "$@"; }
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT

[ -x "$ROOT/bin/swift" ] || fail "bin/swift is missing: run make build"
[ -x "$ROOT/.cache/xray/xray" ] || fail ".cache/xray/xray is missing: run make xray"

docker build -q --build-arg "BASE=$BASE" -t "$IMAGE" "$ROOT/scripts/testdata/install" >/dev/null
docker run -d --name "$NAME" --privileged --cgroupns=host \
	-v /sys/fs/cgroup:/sys/fs/cgroup:rw --tmpfs /run --tmpfs /run/lock "$IMAGE" >/dev/null
for _ in $(seq 60); do
	state="$(ct systemctl is-system-running 2>/dev/null || true)"
	case "$state" in running | degraded) break ;; esac
	sleep 1
done

ct mkdir -p /opt/dist/xray
docker cp "$ROOT/bin/swift" "$NAME:/opt/dist/swift"
for f in xray geoip.dat geosite.dat; do docker cp "$ROOT/.cache/xray/$f" "$NAME:/opt/dist/xray/$f"; done
docker cp "$ROOT/scripts/install.sh" "$NAME:/opt/install.sh"

run_installer() { ct bash /opt/install.sh --non-interactive --no-bbr --from-dir /opt/dist --address 127.0.0.1 "$@"; }

panel_url() { ct swift admin show -config /etc/swift/swift.toml | sed -n 's/^URL: *//p'; }

wait_panel() {
	local url
	url="$(panel_url)"
	for _ in $(seq 30); do
		if ct curl -ksf -o /dev/null "$url"; then return 0; fi
		sleep 1
	done
	fail "panel does not answer at $url"
}

panel_lang() {
	ct curl -ksf "$(panel_url)" | grep -q "<html lang=\"$1\">" || fail "panel is not served in $1"
}

echo "== os-release does not leak into the installer on $BASE"
# VERSION from /etc/os-release once replaced the release tag ("swift_12 (bookworm)_linux_amd64.tar.gz")
# shellcheck disable=SC2016 # expanded inside the container
ct bash -c 'source /opt/install.sh; check_system; [ "$SWIFT_VERSION" = latest ]' || fail "check_system changed SWIFT_VERSION"
# shellcheck disable=SC2016
ct bash -c 'source /opt/install.sh; release_tag; case "$TAG" in v[0-9]*) ;; *) exit 1 ;; esac' || fail "latest release tag not resolved"

echo "== clean install on $BASE"
out="$(run_installer)" || fail "install exited with an error: $out"
printf '%s\n' "$out"
grep -q 'Password *[^ ]\{20\} *│$' <<<"$out" || fail "credentials box missing"
grep -q '100%  Done' <<<"$out" || fail "progress did not reach 100%"
grep -qE 'apt|Reading|Selecting|Profile ' <<<"$out" && fail "installer leaked command output to the screen"
ct systemctl is-active --quiet swift || fail "swift.service is not active"
wait_panel
panel_lang en
ct pgrep -f /usr/local/lib/swift/xray >/dev/null || fail "xray is not running"
ports_before="$(ct swift ports -config /etc/swift/swift.toml)"
grep -q '^tcp 443$' <<<"$ports_before" || fail "no VLESS Reality on 443/tcp"
grep -q '^udp 443$' <<<"$ports_before" || fail "no Hysteria2 on 443/udp"
login="$(ct swift admin show -config /etc/swift/swift.toml | sed -n 's/^Login: *//p')"

echo "== repeated install in Russian"
out="$(run_installer --lang ru)" || fail "second install failed: $out"
grep -q 'Swift уже установлен' <<<"$out" || fail "second install did not report an existing setup in Russian"
grep -q 'Пароль *[^ ]\{20\} *│$' <<<"$out" && fail "second install printed a new password"
[ "$(ct swift ports -config /etc/swift/swift.toml)" = "$ports_before" ] || fail "second install changed ports"
show="$(ct swift admin show -config /etc/swift/swift.toml)"
grep -q "^Login: $login$" <<<"$show" || fail "second install changed the login"
wait_panel
panel_lang ru

echo "== update keeps the installed language"
out="$(run_installer --update)" || fail "--update failed: $out"
grep -q 'Swift обновлён' <<<"$out" || fail "--update did not use the installed language"
ct sh -c 'ls /var/lib/swift/backup-*.db' >/dev/null || fail "--update made no backup"
ct systemctl is-active --quiet swift || fail "swift.service is not active after --update"
wait_panel

echo "== a broken update rolls back by itself"
# what the panel leaves behind right before restarting into a new version
ct swift db backup /var/lib/swift/pre-update.db -config /etc/swift/swift.toml >/dev/null
ct cp /usr/local/bin/swift /usr/local/bin/swift.bak
# the running binary cannot be overwritten in place; the panel renames it too
ct sh -c 'printf "#!/bin/sh\nexit 1\n" > /usr/local/bin/swift.new && chmod 755 /usr/local/bin/swift.new && mv -f /usr/local/bin/swift.new /usr/local/bin/swift'
ct sqlite3 /var/lib/swift/swift.db "INSERT OR REPLACE INTO settings(key, value) VALUES ('swift_update_pending', '{\"from\":\"0.0.1\",\"to\":\"9.9.9\",\"backup\":\"/var/lib/swift/pre-update.db\"}')"
ct systemd-run --unit swift-update-guard-test --collect /usr/local/bin/swift.bak update-guard -config /etc/swift/swift.toml
ct systemctl restart swift || true
rolled=0
for _ in $(seq 90); do
	if ct test -f /usr/local/bin/swift.failed && ct /usr/local/bin/swift version >/dev/null 2>&1; then
		rolled=1
		break
	fi
	sleep 1
done
if [ "$rolled" != 1 ]; then
	ct journalctl -u swift-update-guard-test --no-pager -n 30 >&2 || true
	fail "the broken update was not rolled back"
fi
for _ in $(seq 30); do
	if ct systemctl is-active --quiet swift; then break; fi
	sleep 1
done
ct systemctl is-active --quiet swift || fail "swift.service is not active after the rollback"
wait_panel
[ "$(ct sqlite3 /var/lib/swift/swift.db "SELECT value FROM settings WHERE key = 'swift_skip_version'")" = "9.9.9" ] || fail "the rolled back version is not skipped"

echo "== uninstall"
run_installer --uninstall || fail "--uninstall failed"
ct test ! -e /usr/local/bin/swift || fail "binary left after --uninstall"
ct test ! -e /etc/systemd/system/swift.service || fail "unit left after --uninstall"
ct test -f /var/lib/swift/swift.db || fail "--uninstall removed the database"
run_installer --uninstall --purge || fail "--uninstall --purge failed"
ct test ! -e /var/lib/swift || fail "--purge left /var/lib/swift"

echo "OK: $BASE"

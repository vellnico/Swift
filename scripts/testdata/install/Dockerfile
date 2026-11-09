# Minimal systemd-booting image for the installer test (scripts/test-install.sh).
ARG BASE=debian:12
FROM ${BASE}
ENV DEBIAN_FRONTEND=noninteractive container=docker
RUN apt-get update -qq \
	&& apt-get install -y -qq --no-install-recommends systemd systemd-sysv curl ca-certificates unzip iproute2 procps kmod sqlite3 \
	&& rm -rf /var/lib/apt/lists/* \
	&& systemctl mask systemd-logind.service getty.target console-getty.service
STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]

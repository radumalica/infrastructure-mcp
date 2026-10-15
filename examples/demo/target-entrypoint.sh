#!/bin/sh
# Install the demo public key with the ownership/permissions sshd requires,
# then run sshd in the foreground.
set -eu
install -d -m 700 -o hermes -g hermes /home/hermes/.ssh
install -m 600 -o hermes -g hermes /keys/id_ed25519.pub /home/hermes/.ssh/authorized_keys
exec /usr/sbin/sshd -D -e

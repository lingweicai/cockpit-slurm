#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"

make clean
make
sudo make install
sudo systemctl daemon-reload
sudo systemctl restart cockpit-slurm
sudo systemctl --no-pager --full status cockpit-slurm
#!/usr/bin/env bash
# Runs the test suite with a timeout the simulation package actually fits in.
#
# Go's default is 10 minutes per package. The simulation package measured
# 576-700 seconds across runs, with TestEverySpecialistIsReachable alone at
# 284s and 400s on two of them, so a bare `go test ./...` fails on the alarm
# some of the time and passes the rest. Both of its slowest tests are
# long-horizon guards that are supposed to take that long.
#
# Any extra arguments are passed through: ./test.sh ./manager/ -run Kill
set -u
cd "$(dirname "$0")"
exec go test -timeout 20m "${@:-./...}"

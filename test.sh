#!/usr/bin/env bash
# Runs the test suite with a timeout the simulation package actually fits in.
#
# Go's default is 10 minutes per package. The simulation package measured
# 576-700 seconds across runs, with TestEverySpecialistIsReachable alone at
# 284s and 400s on two of them, so a bare `go test ./...` fails on the alarm
# some of the time and passes the rest. Both of its slowest tests are
# long-horizon guards that are supposed to take that long.
#
# It is 1,299s now, measured on an idle machine with everything passing, and
# TestEverySpecialistIsReachable alone is 525s. Worlds that used to collapse
# to ~150 organisms by cycle 4,000 hold 5,000-7,000 for the whole run under
# the current food economy, so every cycle costs more. 40m rather than 25m
# because the figure above moved by that much once already.
#
# Any extra arguments are passed through: ./test.sh ./manager/ -run Kill
set -u
cd "$(dirname "$0")"
exec go test -timeout 40m "${@:-./...}"

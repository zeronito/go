#!/usr/bin/env bash
# Copyright 2015 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Usage: buildall.bash [-e] [pattern]
#
# buildall.bash builds the standard library for all Go-supported
# architectures. It is used by the "misc-compile" trybot builders,
# as a smoke test to quickly flag portability issues.
#
# Options:
#   -e: stop at first failure

if [ ! -f run.bash ]; then
	echo 'buildall.bash must be run from $GOROOT/src' 1>&2
	exit 1
fi

sete=false
if [ "$1" = "-e" ]; then
	sete=true
	shift
fi

if [ "$sete" = true ]; then
	set -e
fi

pattern="$1"
if [ "$pattern" = "" ]; then
	pattern=.
fi

./make.bash || exit 1
GOROOT="$(cd .. && pwd)"

gettargets() {
	../bin/go tool dist list | sed -e 's|/|-|'
	echo linux-386-387
	echo linux-arm-arm5
}

selectedtargets() {
	gettargets | egrep "$pattern"
}

# put linux first in the target list to get all the architectures up front.
linux_targets() {
	selectedtargets | grep 'linux' | sort
}

non_linux_targets() {
	selectedtargets | grep -v 'linux' | sort
}

# Note words in $targets are separated by both newlines and spaces.
targets="$(linux_targets) $(non_linux_targets)"

failed=false
for target in $targets
do
	echo ""
	echo "### Building $target"
	export GOOS=$(echo $target | sed 's/-.*//')
	export GOARCH=$(echo $target | sed 's/.*-//')
	unset GO386 GOARM
	if [ "$GOARCH" = "arm5" ]; then
		export GOARCH=arm
		export GOARM=5
	fi
	if [ "$GOARCH" = "387" ]; then
		export GOARCH=386
		export GO386=387
	fi

	# Build and vet everything.
	# cmd/go/internal/work/exec.go enables the same vet flags during go test of std cmd
	# and should be kept in sync with any vet flag changes here.
	if ! "$GOROOT/bin/go" build std cmd || ! "$GOROOT/bin/go" vet -unsafeptr=false std cmd; then
		failed=true
		if $sete; then
			exit 1
		fi
	fi
done

if [ "$failed" = "true" ]; then
	echo "" 1>&2
	echo "Build(s) failed." 1>&2
	exit 1
fi

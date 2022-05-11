#!/usr/bin/env bash

usage() {
    echo "test_new_version.sh REPO_PATH BIN_PATH1 BIN_PATH2"
    echo
    echo "test 2 different versions of agnosticV CLI against an agnosticv repository."
    echo
    echo "EXAMPLE"
    echo "cd agnosticv"
    echo "./test_new_version.sh ~/agnosticv ~/bin/agnosticv.v0.3.2 ~/bin/agnosticv.GPTEINFRA-3125"

    exit 2

}

[ -z "${1}" ] && usage
[ ! -d "${1}" ] && usage
[ -z "${2}" ] && usage
[ -z "${3}" ] && usage

cd ${1}
cli1="${2}"
cli2="${3}"

diff -u <($cli1 --list) <($cli2 --list)
if [ $? != 0 ]; then
	echo >&2 "Listing is not the same"
	exit 2
fi

diff -u <($cli1 --list --has __meta__.catalog) <($cli2 --list --has __meta__.catalog)
if [ $? != 0 ]; then
	echo >&2 "Listing using JMSEPath is not the same"
	exit 2
fi
for ci in $($cli1 --list); do
	echo -n "testing merge $ci ......................."

	diff -u <($cli1 --merge $ci) <($cli2 --merge $ci) > /dev/null
	if [ $? != 0 ]; then
		echo "NO"
		diff -y --color=always <($cli1 --merge $ci) <($cli2 --merge $ci)
		#exit 2
	fi

	echo YES
done

for fil in $(find -name common.yaml) $(find -name account.yaml) $(find includes -type f); do
	echo -n "testing related files $fil ......................."
	diff -u <($cli1 --list --related $fil) <($cli2 --list --related $fil) > /dev/null
	if [ $? != 0 ]; then
		echo "NO"
		diff -u <($cli1 --list --related $fil) <($cli2 --list --related $fil)
		exit 2
	fi
	echo YES
done

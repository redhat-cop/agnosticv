#!/usr/bin/env bash

usage() {
    echo "$0 REPO_PATH CLI REV1 REV2"
    echo
    echo "Compare 2 revisions of an agnosticV repo with agnosticv CLI."
    echo
    echo "EXAMPLE"
    echo "cd agnosticv"
    echo "$0 ~/agnosticv ~/bin/agnosticv.v0.7.1 master GPTEINFRA-3125"

    exit 2

}

[ -z "${1}" ] && usage
[ ! -d "${1}" ] && usage
[ -z "${2}" ] && usage
[ -z "${3}" ] && usage
[ -z "${4}" ] && usage

maindir=${1}
cli=${2}
rev1=${3}
rev2=${4}

cd ${maindir}

echo "testing revisions"
git checkout ${rev1}
git checkout ${rev2}

echo -n "listing ......................."

git checkout ${rev1} &> /dev/null
$cli --list > /tmp/list1

git checkout ${rev2} &> /dev/null
$cli --list > /tmp/list2

if ! diff -u /tmp/list1 /tmp/list2; then
	echo >&2 "Listing is not the same"
	exit 2
fi
echo OK

for dir in *; do
    if [ -d $dir ]; then
        printf "%-80s" "listing in ${dir}"
        cd "${dir}"
        git checkout ${rev1} &> /dev/null
        $cli --list > /tmp/list1

        git checkout ${rev2} &> /dev/null
        $cli --list > /tmp/list2

        if ! diff -u /tmp/list1 /tmp/list2; then
            echo >&2 "Listing is not the same"
            exit 2
        fi
        echo OK

        cd ${maindir}
    fi
done

git checkout ${rev1} &> /dev/null
$cli --list --has __meta__.catalog > /tmp/list1
git checkout ${rev2} &> /dev/null
$cli --list --has __meta__.catalog > /tmp/list2
if ! diff -u /tmp/list1 /tmp/list2; then
    echo >&2 "Listing using JMSEPath is not the same"
    exit 2
fi

for ci in $($cli --list); do
	printf "%-80s" "merge $ci"

    git checkout ${rev1} &> /dev/null
    $cli --merge $ci |sed '/^#/d' > /tmp/merge1
    git checkout ${rev2} &> /dev/null
    $cli --merge $ci |sed '/^#/d' > /tmp/merge2
	if ! diff -u /tmp/merge1 /tmp/merge2; then
		echo "NO"
		#exit 2
	fi

	echo YES
done

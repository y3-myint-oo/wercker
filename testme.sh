#!/bin/sh

function integration_tests() {
    ./wercker-sentcli --projectDir=tests/projects build pass || return 1
    ./wercker-sentcli --projectDir=tests/projects build fail && return 1
    return 0
}

if [ -z "$1" ]; then
  ./buildme.sh && integration_tests
else
  ./buildme.sh && ./wercker-sentcli build $1
fi

passfail=$?

case $passfail in
    0)
        echo "♡ PASS"
        ;;
    1)
        echo "✘ FAIL"
        ;;
esac

exit $passfail

# ./sentcli run "while true; do date; sleep 1; done"

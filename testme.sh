#!/bin/bash

function integration_tests() {
    ./sentcli build tests/projects/fail && return 1
    ./sentcli build tests/projects/pass || return 1
    return 0
}

if [ -z "$1" ]; then
  ./buildme.sh && integration_tests
else
  ./buildme.sh && ./sentcli build $1
fi

passfail=$?

echo -n "Tests "
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

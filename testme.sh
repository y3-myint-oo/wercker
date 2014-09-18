#!/bin/sh

function integration_tests() {
    ./sentcli --projectDir=tests/projects build pass || return 1
    ./sentcli --projectDir=tests/projects build fail && return 1
    return 0
}

./buildme.sh && ./sentcli build $1 && integration_tests

passfail=$?

case $passfail in
    0)
        echo "tests pass"
        ;;
    1)
        echo "tests fail"
        ;;
esac

exit $passfail

# ./sentcli run "while true; do date; sleep 1; done"

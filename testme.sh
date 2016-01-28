#!/bin/bash

function integration_tests() {
  local keenProjectWriteKey="**REMOVED**"
  local keenProjectId="**REMOVED**"

  # ./sentcli \
  #   build tests/projects/fail \
  #   --keen-metrics \
  #   --keen-project-write-key $keenProjectWriteKey \
  #   --keen-project-id $keenProjectId \
  #   && return 1

  ./sentcli \
    build tests/projects/pass \
    --keen-metrics \
    --keen-project-write-key $keenProjectWriteKey \
    --keen-project-id $keenProjectId \
    || return 1

  return 0
}

if [ -z "$1" ]; then
  ./buildme.sh && integration_tests
else
  ./buildme.sh && ./sentcli $@
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

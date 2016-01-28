#!/bin/bash

function integration_tests() {
  local keenProjectWriteKey="**REMOVED**"
  local keenProjectId="**REMOVED**"

  # ./wercker \
  #   build tests/projects/fail \
  #   --keen-metrics \
  #   --keen-project-write-key $keenProjectWriteKey \
  #   --keen-project-id $keenProjectId \
  #   && return 1

  ./wercker \
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
  ./buildme.sh && ./wercker $@
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

# ./wercker run "while true; do date; sleep 1; done"

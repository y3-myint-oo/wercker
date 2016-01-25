#!/bin/bash

sentcli=../../sentcli
workingDir=./.tests

basicTest() {
  testName=$1
  shift
  printf "testing %s... " "$testName"
  $sentcli $@ --working-dir $workingDir > "${workingDir}/${testName}.log"
  if [ $? -ne 0 ]; then
    printf "failed\n"
    cat "${workingDir}/${testName}.log"
    return 1
  else
    printf "passed\n"
  fi
  return 0
}

basicTest "gitignore" build gitignore
basicTest "source-path" build source-path

testDirectMount() {
  echo -n "testing direct-mount..."
  testDir="./direct-mount"
  testFile=${testDir}/testfile
  > $testFile
  echo "hello" > $testFile
  logFile="${workingDir}/direct-mount.log"
  $sentcli build $testDir --direct-mount --working-dir $workingDir > $logFile
  contents=`cat ${testFile}`
  if [ $contents == 'world' ]
      then echo "passed"
      return 0
  else
      echo 'failed'
      cat $logFile
      return 1
  fi
}

testDirectMount

#!/bin/bash

# This is a shell script to run a bunch of regression tests that require
# running sentcli in a fully docker-enabled environment. They'll eventually
# be moved into a golang test package.
wercker=./wercker
workingDir=./.werckertests
testsDir=./tests/projects

# Make sure we have a working directory
mkdir -p $workingDir
if [ ! -e "$wercker" ]; then
  go build
fi


basicTest() {
  testName=$1
  shift
  printf "testing %s... " "$testName"
  $wercker $@ --working-dir $workingDir > "${workingDir}/${testName}.log"
  if [ $? -ne 0 ]; then
    printf "failed\n"
    cat "${workingDir}/${testName}.log"
    return 1
  else
    printf "passed\n"
  fi
  return 0
}

testDirectMount() {
  echo -n "testing direct-mount..."
  testDir=$testsDir/direct-mount
  testFile=${testDir}/testfile
  > $testFile
  echo "hello" > $testFile
  logFile="${workingDir}/direct-mount.log"
  $wercker build $testDir --direct-mount --working-dir $workingDir > $logFile
  contents=$(cat ${testFile})
  if [ "$contents" == 'world' ]
      then echo "passed"
      return 0
  else
      echo 'failed'
      cat $logFile
      return 1
  fi
}


runTests() {
  basicTest "source-path" build $testsDir/source-path || return 1
  basicTest "test local services" build $testsDir/local-service/service-consumer || return 1
  basicTest "test deploy" deploy $testsDir/deploy-no-targets || return 1
  basicTest "test deploy target" deploy --deploy-target test $testsDir/deploy-targets || return 1
  testDirectMount || return 1
}

runTests
rm -rf $workingDir

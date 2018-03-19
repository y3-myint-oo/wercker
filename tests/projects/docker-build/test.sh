#!/bin/bash

# This is intended to be called from wercker/test-all.sh, which sets the required environment variables
# if you run this file directly, you need to set $wercker, $workingDir and $testDir
# as a convenience, if these are not set then assume we're running from the local directory 
if [ -z ${wercker} ]; then wercker=$PWD/../../../wercker; fi
if [ -z ${workingDir} ]; then workingDir=$PWD/../../../.werckertests; mkdir -p "$workingDir"; fi
if [ -z ${testsDir} ]; then testsDir=$PWD/..; fi

testDockerBuild () {
  testName=docker-build
  testDir=$testsDir/docker-build
  printf "testing %s... " "$testName"
  # this test will create an image with the following tag
  tag=docker-build-test-image
  # stop any existing container started by a previous run
  docker kill ${testName}-container > /dev/null 2>&1
  # delete any existing image built by a previous run
  docker images | grep $tag | awk '{print $3}' | xargs -n1 docker rmi -f > /dev/null 2>&1
  # check no existing image with the specified tag
  docker images | grep -q "$tag"
  if [ $? -eq 0 ]; then
    echo "An image with tag $tag already exists"
    return 1
  fi
  # now run the build pipeline - this creates an image with the specified tag
  $wercker build "$testDir" --docker-local --working-dir "$workingDir" &> "${workingDir}/${testName}.log"
  if [ $? -ne 0 ]; then
    printf "failed\n"
    if [ "${workingDir}/${testName}.log" ]; then
      cat "${workingDir}/${testName}.log"
    fi
    return 1
  fi
  # verify that an image was created with expected tag
  if [ $? -ne 0 ]; then
    echo "An image with tag $tag was not found"
    return 1
  fi
  # start the image using the docker CLI
  docker run --name ${testName}-container --rm -d -p 5000:5000 $tag > "${workingDir}/${testName}.log" 2>&1
  # test the image
  curlOutput=`curl -s localhost:5000`
  # stop the container
  docker kill ${testName}-container > "${workingDir}/${testName}.log" 2>&1
  # delete the image we've just created
  docker images | grep $tag | awk '{print $3}' | xargs -n1 docker rmi -f > "${workingDir}/${testName}.log" 2>&1
  # now the container and image have been cleaned up, check whether the test worked
  if [ "$curlOutput" != "Hello World!" ]; then
    echo "Unexpected response from test container: " $curlOutput
    return 1
  fi
  # test passed
  printf "passed\n"
  return 0
}

testDockerBuildAll () {
  testDockerBuild || return 1 
}

testDockerBuildAll

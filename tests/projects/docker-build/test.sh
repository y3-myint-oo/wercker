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
  
  ##########################################################
  # Test1: Test docker-build by running the build pipeline #
  ##########################################################

  # this test will create an image whose repository setting has the following suffix - should match the image-name setting in wercker.yml
  imagename=docker-build-image-name-1
  # stop any existing container started by a previous run
  docker kill ${testName}-container > /dev/null 2>&1
  # delete any existing image built by a previous run
  docker images | grep $imagename | awk '{print $3}' | xargs -n1 docker rmi -f > /dev/null 2>&1
  # check no existing image with the specified repository (column 1 is the repository)
  docker images | awk '{print $1}' | grep -q "$imagename"
  if [ $? -eq 0 ]; then
    echo "An image with repository $imagename already exists"
    return 1
  fi
  # now run the build pipeline - this creates an image with the specified repository setting
  $wercker build "$testDir" --docker-local --working-dir "$workingDir" &> "${workingDir}/${testName}.log"
  if [ $? -ne 0 ]; then
    printf "failed\n"
    if [ "${workingDir}/${testName}.log" ]; then
      cat "${workingDir}/${testName}.log"
    fi
    return 1
  fi
  # verify that an image was created with the expected repository setting (column 1 is the repository)
  docker images | awk '{print $1}' | grep -q "$imagename"
  if [ $? -ne 0 ]; then
    echo "An image with repository $imagename was not found"
    return 1
  fi
  # find the imageid of this image (we don't know the complete repository name as it is prefixed with build-id)
  imageid=`docker images | grep "$imagename" | awk '{print $3}'`
  # verify that the image has NOT been squashed by doing "docker history" and checking that the most recent layer was NOT a merge
  docker history $imageid  --format "{{.Comment}}" | head -1 | grep -q merge
  if [ $? -eq 0 ]; then
    echo "Image was unexpectedly merged"
    return 1
  fi

  # start the image using the docker CLI
  docker run --name ${testName}-container --rm -d -p 5000:5000 ${imageid} >> "${workingDir}/${testName}.log" 2>&1
  # test the image
  curlOutput1=`curl -s localhost:5000`         # should return "Hello World!"
  curlOutput2=`curl -s localhost:5000/env/foo` # should return value of build-arg foo (set in wercker.yml)"
  curlOutput3=`curl -s localhost:5000/env/bar` # should return value of build-arg bar (set in wercker.yml)"
  # stop the container
  docker kill ${testName}-container >> "${workingDir}/${testName}.log" 2>&1
  # delete the image we've just created
  docker images | grep $imagename | awk '{print $3}' | xargs -n1 docker rmi -f >> "${workingDir}/${testName}.log" 2>&1
  # now the container and image have been cleaned up, check whether the test worked
  if [ "$curlOutput1" != "Hello World!" ]; then
    cat "${workingDir}/${testName}.log"
    echo "Unexpected response from test container for localhost:5000 " $curlOutput1
    return 1
  fi
  if [ "$curlOutput2" != "val1" ]; then
    cat "${workingDir}/${testName}.log"
    echo "Unexpected response from test container for localhost:5000/env/foo " $curlOutput2
    return 1
  fi
  if [ "$curlOutput3" != "val2" ]; then
    cat "${workingDir}/${testName}.log"
    echo "Unexpected response from test container for localhost:5000/env/bar " $curlOutput3
    return 1
  fi    

  ##############################################################################
  # Test2: test the "squash" property by running the build-and-squash pipeline #
  ##############################################################################

  # this test will create an image whose repository setting has the following suffix - should match the image-name setting in wercker.yml
  imagename2=docker-build-image-name-2
  # delete any existing image built by a previous run
  docker images | grep $imagename2 | awk '{print $3}' | xargs -n1 docker rmi -f > /dev/null 2>&1
  # check no existing image with the specified repository (column 1 is the repository)
  docker images | awk '{print $1}' | grep -q "$imagename2"
  if [ $? -eq 0 ]; then
    echo "An image with repository $imagename2 already exists"
    return 1
  fi
  # now run the build-and-squash pipeline to build a squashed image
  $wercker build "$testDir" --pipeline build-and-squash --docker-local --working-dir "$workingDir" &> "${workingDir}/${testName}.log"
  if [ $? -ne 0 ]; then
    printf "failed\n"
    if [ "${workingDir}/${testName}.log" ]; then
      cat "${workingDir}/${testName}.log"
    fi
    return 1
  fi
  # find the imageid of this image (we don't know the complete repository name as it is prefixed with build-id)
  imageid2=`docker images | grep $imagename2 | awk '{print $3}'`
  # verify that the image HAS been squashed by doing "docker history" and checking that the most recent layer WAS a merge
  docker history $imageid2 --format "{{.Comment}}" | head -1 | grep -q merge
  if [ $? -ne 0 ]; then
    echo "Image was not merged"
    return 1
  fi

  # test passed
  #cat "${workingDir}/${testName}.log"
  printf "passed\n"
  return 0
}

testDockerBuildAll () {
  testDockerBuild || return 1 
}

testDockerBuildAll

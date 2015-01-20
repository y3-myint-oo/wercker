#!/bin/bash

## Detect platform
PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')

found_python=$(which python)

function python_parse_docker_version {
  local docker_host
  docker_host=$1
  curl -s ${docker_host}/version | python -c "import json,sys;obj=json.load(sys.stdin);print '%s %s' % (obj['Version'], obj['ApiVersion'])" 2>/dev/null
}


function check_darwin {
  found_docker=$(which docker)
  found_env=$DOCKER_HOST
  found_b2d=$(which boot2docker)

  found_versions=0
  # If we found docker installed, try to check the server version with it
  if [[ -n "$found_docker" ]]; then
    version_response=$(${found_docker} version 2>/dev/null)
    status=$?
    if [[ $status -eq 0 ]]; then
      server_version=$(${found_docker} version | grep "Server version" | cut -f 3 -d " ")
      api_version=$(${found_docker} version | grep "Server API version" | cut -f 4 -d " ")
      found_versions=1
    fi
  fi

  # If docker wasn't there, that's not the end of the world, let's check for
  # the DOCKER_HOST envvar
  if [[ $found_versions -eq 0 ]] && [[ -n "$found_env" ]]; then
    version_response=$(python_parse_docker_version "$found_env")
    status=$?
    if [[ $status -eq 0 ]]; then
      server_version=$(echo ${version_response} | cut -d " " -f 1)
      api_version=$(echo ${version_response} | cut -d " " -f 2)
      found_versions=1
    fi
  fi

  # If neither of those were around but boot2docker was maybe they have
  # boot2docker installed and just haven't really set it up correctly
  if [[ $found_versions -eq 0 ]] && [[ -n "$found_b2d" ]]; then
    echo "You seem to have boot2docker installed but are not currently"
    echo "running it (or at least don't have any environment variables"
    echo "set in your shell.)"
    echo ""
    echo "You probably already know this, but the way to do that is:"
    echo ""
    echo "  boot2docker up"
    echo "  \$(boot2docker shellinit)"
  fi

  # If we found other things, but they failed to connect, give some nice
  # error messages about them before suggesting an install

  # We had an envvar but no boot2docker and no docker
  if [[ $found_versions -eq 0 ]] && [[ -n "$found_env" ]] && [[ -z "$found_docker" ]] && [[ -z "$found_b2d" ]]; then
    echo "You seem to have a DOCKER_HOST environment variable set but"
    echo "no server running (at least we can't connect to it.)"
    echo ""
    echo "You seem to know what you are doing, so just keep in mind"
    echo "that you will need a docker server running at that location"
    echo "for the wercker command-line tool to work."
    echo ""
  fi

  # We had an docker client but no boot2docker
  if [[ $found_versions -eq 0 ]] && [[ -n "$found_docker" ]] && [[ -z "$found_b2d" ]]; then
    echo "You seem to have a docker client installed but no server running"
    echo "(at least we can't connect to it.)"
    echo ""
    echo "You probably know what you are doing, so just keep in mind"
    echo "that you will need a docker server running and probably also"
    echo "a DOCKER_HOST environment variable set with its location"
    echo "for the wercker command-line tool to work."
    echo ""
  fi


  if [[ $found_versions -ne 0 ]]; then
    echo "docker server version: ${server_version}"
    echo "docker api version: ${api_version}"
  else
    echo "We weren't able to get any version information about docker."
  fi

  #if [ -n "$found_docker" ]; then
  #  echo "checking for docker: ${found_docker}"
  #else
  #  echo "checking for docker: ${found_docker}"
  #fi

  #if
  #  echo "checking for boot2docker: ${found_b2d}
}

if [[ "darwin" = "$PLATFORM" ]]; then
  check_darwin
elif [[ "linux" = "$PLATFORM" ]]; then
  check_darwin
fi

#!/bin/sh

cat << "EOF"
                              __        
 _      __ ___   _____ _____ / /__ ___   _____
| | /| / // _ \ / ___// ___// //_// _ \ / ___/
| |/ |/ //  __// /   / /__ / ,<  /  __// /    
|__/|__/ \___//_/    \___//_/|_| \___//_/     

EOF
echo "-----> Installing the wercker CLI"



## Detect platform
PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')

found_python=$(which python)

python_parse_docker_version() {
  local docker_host=$1
  curl -s ${docker_host}/version | python -c "import json,sys;obj=json.load(sys.stdin);print '%s %s' % (obj['Version'], obj['ApiVersion'])" 2>/dev/null
}


check_darwin() {
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
    echo
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
    echo "for the wercker command-line interface to work."
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
    echo "Found the following docker versions:"
    echo "docker server version: ${server_version}"
    echo "docker api version: ${api_version}"
    echo
    download_cli darwin
  else
    echo "Unable to determine docker version"
    echo
    echo "You can install boot2docker via homebrew by running:"
    echo "brew install boot2docker"
    echo
    echo "Alternatively you can download the boot2docker installer from: https://github.com/boot2docker/osx-installer/releases"
    echo
    echo "Installation of the wercker CLI failed. Visit http://devcenter.wercker.com for more information."
  fi
}

download_cli() {
    platform=$1
    url="http://downloads.wercker.com/cli/stable/${platform}_amd64/wercker"
    
    if [[ :$PATH: == *:"/usr/local/bin":* ]] ; then
        # is /usr/local/bin available in the PATH
        location="/usr/local/bin/wercker"
        echo "The directory $location is available in the PATH"
        echo "We will install the wercker command inside this directory"
        echo
    elif [[ :$PATH: == *:"/usr/bin":* ]] ; then
        # is /usr/bin available in the PATH
        location="/usr/bin/wercker"
        echo "The directory /usr/bin is available in the PATH"
        echo "We will install the wercker command inside this directory"
    else
        echo "None of the preferred paths available, we're downloading wercker in the current folder"
        curl -w '%{http_code}' $url -o wercker
    fi
    
    status=$(curl -w '%{http_code}' $url -o $location)
    if [[ "$status" != "200" ]]; then
        echo "Unable to download CLI from http://downloads.wercker.com"
        exit 1
    else
        make_executable $location
    fi
}


make_executable() {
    # TODO: probably check if this command is succesful or not
    if chmod +x $1; then
        echo
        echo "Succesfully made wercker command executable"
    else
        echo "Unable to make wercker command executable."
        echo "Try to run chmod +x $1 ."
    fi
}

check_linux() {
  found_docker=$(which docker)
  found_env=$DOCKER_HOST

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
  download_cli linux
}

if [[ "darwin" = "$PLATFORM" ]]; then
  check_darwin
elif [[ "linux" = "$PLATFORM" ]]; then
  check_linux
else
  echo "We were unable to detect your platform."
  echo "Currently we only support Mac OSX and Linux"
fi

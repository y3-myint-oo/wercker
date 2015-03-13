#!/bin/sh

set -e

main() {
  cat << "EOF"
                       _
__      _____ _ __ ___| | _____ _ __
\ \ /\ / / _ \ '__/ __| |/ / _ \ '__|
 \ V  V /  __/ | | (__|   <  __/ |
  \_/\_/ \___|_|  \___|_|\_\___|_|

EOF

  local platform
  platform=$(uname -s | tr '[:upper:]' '[:lower:]')

  if [ "$platform" != "darwin" ] && [ "$platform" != "linux" ]; then
    echo 'We were unable to detect your platform.'
    echo 'Currently we only support Mac OSX and Linux.'
    exit 1
  fi

  echo 'Installing the latest version of the wercker CLI'
  install "$platform"
  echo
  echo 'Succesfully installed the wercker CLI:'
  /usr/local/bin/wercker version --no-update-check
  echo
  echo 'Checking for docker'
  check "$platform"
}

# install $platform - Install the wercker CLI for $platform (linux or darwin)
install() {
  local platform="$1"
  local url="https://s3.amazonaws.com/downloads.wercker.com/cli/stable/${platform}_amd64/wercker"
  local location="/usr/local/bin/wercker"

  local install_script="
echo \"Downloading the CLI...\"
status=\$(curl -# -w '%{http_code}' \"$url\" -o \"$location\");
if [ \"\$status\" != \"200\" ]; then
  echo \"Unable to download CLI from http://downloads.wercker.com\";
  exit 2;
fi

if chmod +x \"$location\"; then
  echo \"done.\";
else
  echo \"Unable to make wercker command executable.\";
  echo \"Try to run chmod +x $location .\";
fi"

  if [ "$platform" = "darwin" ]; then
    echo "$install_script" | /bin/sh
  elif [ "$platform" = "linux" ]; then
    echo
    echo "This script requires superuser access to install software."
    echo "You will be prompted for your password by sudo."
    sudo -k
    echo "$install_script" | sudo /bin/sh
  fi

  # remind the user to add to $PATH if they don't already have it
  case "$PATH" in
    */usr/local/bin*)
      ;;
    *)
      echo "Add the wercker CLI to your PATH using:"
      echo "$ echo 'PATH=\"/usr/local/bin:\$PATH\"' >> ~/.profile"
      ;;
  esac
}

# check $platform - check for Docker on $platform (linux or darwin)
# TODO(bvdberg): Ideally we want to move this check to the wercker CLI.
check() {
  local platform="$1"

  if [ "$platform" = "darwin" ]; then
    check_darwin
  elif [ "$platform" = "linux" ]; then
    check_linux
  else
    echo "Unable to check for your current platform for Docker"
  fi
}

# check_darwin - check for Docker on darwin
check_darwin() {
  local found_docker
  local found_b2d
  local version_response
  local status
  local server_version
  local api_version

  local found_env="$DOCKER_HOST"
  found_docker=$(which docker)
  found_b2d=$(which boot2docker)

  local found_versions=0
  # If we found docker installed, try to check the server version with it
  if [ -n "$found_docker" ]; then
    version_response=$(${found_docker} version 2>/dev/null)
    status=$?
    if [ "$status" -eq 0 ]; then
      server_version=$(${found_docker} version | grep "Server version" | cut -f 3 -d " ")
      api_version=$(${found_docker} version | grep "Server API version" | cut -f 4 -d " ")
      found_versions=1
    fi
  fi

  # If docker wasn't there, that's not the end of the world, let's check for
  # the DOCKER_HOST envvar
  if [ $found_versions -eq 0 ] && [ -n "$found_env" ]; then
    version_response=$(parse_docker_version "$found_env")
    status=$?
    if [ "$status" -eq 0 ]; then
      server_version=$(echo "${version_response}" | cut -d " " -f 1)
      api_version=$(echo "${version_response}" | cut -d " " -f 2)
      found_versions=1
    fi
  fi

  # If neither of those were around but boot2docker was maybe they have
  # boot2docker installed and just haven't really set it up correctly
  if [ $found_versions -eq 0 ] && [ -n "$found_b2d" ]; then
    echo "You seem to have boot2docker installed but are not currently"
    echo "running it (or at least don't have any environment variables"
    echo "set in your shell.)"
    echo ""
    echo "You probably already know this, but the way to do that is:"
    echo ""
    echo "  boot2docker up"
    echo "  \$(boot2docker shellinit)"
    echo ""
  fi

  # If we found other things, but they failed to connect, give some nice
  # error messages about them before suggesting an install

  # We had an envvar but no boot2docker and no docker
  if [ $found_versions -eq 0 ] && [ -n "$found_env" ] && [ -z "$found_docker" ] && [ -z "$found_b2d" ]; then
    echo "You seem to have a DOCKER_HOST environment variable set but"
    echo "no server running (at least we can't connect to it.)"
    echo ""
    echo "You seem to know what you are doing, so just keep in mind"
    echo "that you will need a docker server running at that location"
    echo "for the wercker command-line interface to work."
    echo ""
  fi

  # We had an docker client but no boot2docker
  if [ $found_versions -eq 0 ] && [ -n "$found_docker" ] && [ -z "$found_b2d" ]; then
    echo "You seem to have a docker client installed but no server running"
    echo "(at least we can't connect to it.)"
    echo ""
    echo "You probably know what you are doing, so just keep in mind"
    echo "that you will need a docker server running and probably also"
    echo "a DOCKER_HOST environment variable set with its location"
    echo "for the wercker command-line tool to work."
    echo ""
  fi

  if [ $found_versions -ne 0 ]; then
    echo "Found the following docker versions:"
    echo "docker server version: ${server_version}"
    echo "docker api version: ${api_version}"
  else
    echo "Unable to determine docker version"
  fi
}

# check_linux - check for Docker on linux
check_linux() {
  local found_docker
  local version_response
  local status
  local server_version
  local api_version

  local found_env=
  found_docker=$(which docker)

  found_versions=0
  # If we found docker installed, try to check the server version with it
  if [ -n "$found_docker" ]; then
    version_response=$(${found_docker} version 2>/dev/null)
    status=$?
    if [ "$status" -eq 0 ]; then
      server_version=$(${found_docker} version | grep "Server version" | cut -f 3 -d " ")
      api_version=$(${found_docker} version | grep "Server API version" | cut -f 4 -d " ")
      found_versions=1
    fi
  fi

  # If docker wasn't there, that's not the end of the world, let's check for
  # the DOCKER_HOST envvar
  if [ $found_versions -eq 0 ] && [ -n "$found_env" ]; then
    version_response=$(parse_docker_version "$found_env")
    status=$?
    if [ "$status" -eq 0 ]; then
      server_version=$(echo "${version_response}" | cut -d " " -f 1)
      api_version=$(echo "${version_response}" | cut -d " " -f 2)
      found_versions=1
    fi
  fi

  # We had an envvar but no docker
  if [ $found_versions -eq 0 ] && [ -n "$found_env" ] && [ -z "$found_docker" ]; then
    echo "You seem to have a DOCKER_HOST environment variable set but"
    echo "no server running (at least we can't connect to it.)"
    echo ""
    echo "You seem to know what you are doing, so just keep in mind"
    echo "that you will need a docker server running at that location"
    echo "for the wercker command-line interface to work."
    echo ""
  fi

  # We had an docker client
  if [ $found_versions -eq 0 ] && [ -n "$found_docker" ]; then
    echo "You seem to have a docker client installed but no server running"
    echo "(at least we can't connect to it.)"
    echo ""
    echo "You probably know what you are doing, so just keep in mind"
    echo "that you will need a docker server running and probably also"
    echo "a DOCKER_HOST environment variable set with its location"
    echo "for the wercker command-line tool to work."
    echo ""
  fi

  if [ $found_versions -ne 0 ]; then
    echo "Found the following docker versions:"
    echo "docker server version: ${server_version}"
    echo "docker api version: ${api_version}"
  else
    echo "Unable to determine docker version"
  fi
}

parse_docker_version() {
  local host="$1"
  curl -s "${host}/version" | python -c "import json,sys;obj=json.load(sys.stdin);print '%s %s' % (obj['Version'], obj['ApiVersion'])" 2>/dev/null
}

main

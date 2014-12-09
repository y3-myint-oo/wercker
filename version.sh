#
# Write the git `HEAD` commit to `version.go`.
# Enables us to log the version of `sentcli`
# we're currently on. Only used in `wercker.yml`,
# don't run this locally.
#

GITHASH=$1

cat >./version.go <<EOL
package main

const (
  GitVersion = "$GITHASH"
  Version    = "1.0.0"
)
EOL

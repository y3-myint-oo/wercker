# wercker
==============================

`wercker` is a CLI designed to increase developer velocity by enabling users to run their tests and build projects locally by leveraging the power of Docker containers.

Note: The `master` branch may be in a broken or unstable state during development. Therefore, it is recommended that you download `wercker` through [the downloads section](http://wercker.com/cli/)
on our website, if you're not contributing to the code base.

## Building wercker

`wercker` is built using Go version 1.5 or greater. If you don't have it already, you can get it from the [official download page](https://golang.org/dl/). Once you go installed, set up your go environment by [using this guide](https://golang.org/doc/code.html#Organization)

Next, you'll need `glide` to install `wercker`'s dependencies. You can do this by running `go get github.com/Masterminds/glide`

Set `GO15VENDOREXPERIMENT=1` in your shell (see [Go 1.5 Vendor Experiment](https://docs.google.com/document/d/1Bz5-UB7g2uPBdOx-rw5t9MxJwkfpx90cqG9AFL0JAYo/edit))

Run `go get github.com/wercker/wercker` in order to fetch repository.

Next, run `glide install --quick`. This command should download the appropiate dependencies wercker needs.

Once all that is setup, you should be able to run `go build` and get a working executable called `wercker`.

Note: this is the bare minimum to build and contribute to the code base. It is also reccomended you install docker locally in order to be able to run `wercker` properly. You can follow [this guide](https://docs.docker.com/engine/installation/) to install it on your machine.

## Reporting Bugs

If you are experiencing bugs running wercker locally, please create an issue
containing the following:

- Which OS are you using?
- Which Docker environment are you using? (Boot2docker, custom, etc)
- Create a gist containing the following information:
  - The entire log when running wercker with the `--debug` log. (ie. `wercker --debug build`)
  - The wercker.yml file that causes the issues.

Please don't file any issue dealing with the usage of steps or unexpected behavior on hosted wercker.

If you are experiencing issues running builds or deploys on hosted wercker,
please do the following:

Try running the build again to see if the error keeps occurring. If it does, turn
on support for the application, and create an issue with the following
information:

- The application owner and application name.
- The ID of the build or deploy that failed.

## Contact

Join us in our slack room: [![Slack Status](http://werckerpublicslack.herokuapp.com/badge.svg)](http://slack.wercker.com)

## License

`wercker` is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.

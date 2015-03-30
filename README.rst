sentcli
=======

Parse your wercker.yml, do the things wercker would do.

 .. image:: https://app.wercker.com/status/b9328d1816a2c82df512ca359cd934aa/m
    :alt: "Wercker status" 
    :target: https://app.wercker.com/status/b9328d1816a2c82df512ca359cd934aa/


Getting Started
---------------

Usage
-----
::

  NAME:
  ./sentcli - A new cli application

  USAGE:
  ./sentcli [global options] command [command options] [arguments...]

  VERSION:
  0.0.0

  COMMANDS:
  build, b	build a project
  help, h	Shows a list of commands or help for one command

  GLOBAL OPTIONS:
  --project-dir './_projects'				path where downloaded projects live
  --step-dir './_steps'				path where downloaded steps live
  --build-dir './_builds'				path where created builds live
  --docker-host 'tcp://127.0.0.1:2375'			docker api host [$DOCKER_HOST]
  --wercker-endpoint 'https://app.wercker.com/api/v2'	wercker api endpoint
  --base-url 'https://app.wercker.com/'		base url for the web app
  --registry '127.0.0.1:3000'				registry endpoint to push images to
  --mnt-root '/mnt'					directory on the guest where volumes are mounted
  --guest-root '/pipeline'				directory on the guest where work is done
  --report-root '/report'				directory on the guest where reports will be written
  --build-id 						build id [$WERCKER_BUILD_ID]
  --application-id 					application id [$WERCKER_APPLICATION_ID]
  --application-name 					application id [$WERCKER_APPLICATION_NAME]
  --application-owner-name 				application id [$WERCKER_APPLICATION_OWNER_NAME]
  --application-started-by-name 			application started by [$WERCKER_APPLICATION_STARTED_BY_NAME]
  --push						push the build result to registry
  --commit						commit the build result locally
  --tag 						tag for this build [$WERCKER_GIT_BRANCH]
  --message 						message for this build
  --aws-secret-key 					secret access key
  --aws-access-key 					access key id
  --s3-bucket 'wercker-development'			bucket for artifacts
  --aws-region 'us-east-1'				region
  --keen-metrics					report metrics to keen.io
  --keen-project-write-key 				keen write key
  --keen-project-id 					keen project id
  --source-dir 					source path relative to checkout root
  --no-response-timeout '5'				timeout if no script output is received in this many minutes
  --command-timeout '10'				timeout if command does not complete in this many minutes
  --help, -h						show help
  --version, -v					print the version

We're still in the demo-ware phase so there are a couple steps to getting
this working locally:

  1. You need a working Docker environment. I do this on OS X by setting up
     a CoreOS environment and exposing the docker port in config.rb
     `$expose_docker_tcp=4243`
  2. You need to manually check out the project you are going to build into
     the `projects` directory. Something like::
       $ mkdir projects/wercker
       $ git clone git@github.com:wercker/wercker-sentinel \
         projects/wercker/wercker-sentinel

  3. You need a working Go environment and the root of the checkout has to
     be on your `GOPATH`. Mine looks like::

       GOPATH=/Users/termie/.venv/test-sentcli/gopath:/Users/termie/p/wercker/test-sentcli

  4. If you're using the Vagrant method, you need to mount your local
     directories in the box so that Docker can mount them. If you are
     running inside a VM or on Linux already you don't need to do this.
     Anyway, this is what I added to my Vagrantfile::

       config.vm.synced_folder "/Users/termie/dev/wercker", "/Users/termie/dev/wercker", id: "wercker", :nfs => true, :mount_options => ['nolock,vers=3,udp']

  5. You need to import the appropriate base boxes. They don't all work
     perfectly, but these two do, at least::

       # for the wercker/python box
       sudo ./convert_lxc.sh wercker/python "https://s3.amazonaws.com/wercker-production-optimi/1c84b4ce-2c0a-42d5-931a-9f07721de53e"

     In the Vagrant version of Docker/CoreOS I do this stuff on the actual box
     because I don't actually have Docker installed locally.


Now that you have all that stuff working, let's do the fun stuffs::

  $ glide in
  $ glide install
  $ mkdir -p projects/termie
  $ git clone http://github.com/termie/farmboy projects/termie/farmboy
  $ ./testme.sh termie/farmboy







Basic Process
-------------

  1. Download boxes  (requires new box api?)
  2. Download steps (steps api?)
  3. EXECUTE
    a. Build steps into scripts
    b. Run docker containers locally
    c. Execute scripts in docker containers

See https://github.com/wercker/sentcli/blob/master/docs/design.rst for more.


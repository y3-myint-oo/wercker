wercker-sentcli
===============

Parse your wercker.yml, do the things wercker would do.


Getting Started
---------------

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

  $ ./install_dependencies.sh
  $ ./testme.sh wercker/wercker-sentinel







Basic Process
-------------

  1. Download boxes  (requires new box api?)
  2. Download steps (steps api?)
  3. EXECUTE
    a. Build steps into scripts
    b. Run docker containers locally
    c. Execute scripts in docker containers

See https://github.com/wercker/wercker-sentcli/blob/master/docs/design.rst for more.

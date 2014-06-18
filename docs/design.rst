Sentinel as a CLI
=================

Overview overview overview



User Stories
------------

Such users,

So wow


API (Usage?)
------------

command line interface, args, options



Architecture
------------

-----------
wercker.yml
-----------

This is the main file for describing how to build a project, it lives in the
project's code repository and looks like this::


  # This is the base Image to use, this one is provided by wercker
  # but any valid public Docker image should be acceptable.
  box: wercker/ruby

  # Services are Images that we instantiate to be interacted with by the
  # build Containers. This accepts the same Image types as above, but
  # will probably also have access to some private Images that will not be
  # downloadable by users (so that we can sell access to services).
  services:
      - mies/rethinkdb

  # Build is a conceptual Job (see Jobs below) and a physical job in CoreOS,
  # it will instantiate a new Container from the base Image ("box" above) and
  # run the steps defined inside of it, save the output, commit the changes
  # to a new image and push it to our repository.
  build:
      # Each step is provided to the Container as a Volume, the overall build
      # Job will execute each of these steps in the order given.
      steps:
          # Execute the `bundle-install` step provided by wercker.
          - bundle-install
          # Execute a custom script.
          - script:
              # This name will be displayed in the wercker UI and logs.
              name: middleman build
              # This is the actual code to execute
              code: |
                echo "hello world"
                bundle exec middleman build --verbose

  # Deploy is also a conceptual Job and a physical job in CoreOS, it differs
  # from Build in that semantically we aren't usually running Deploy jobs
  # automatically: they are triggered manually from a successful build.
  # As with all Jobs, this has access to the outputs and images from the Jobs
  # before it in the pipeline (in this case the Build job).
  deploy:
      # By default (?), the deploy Container is based on the committed Image
      # from the build that it has been triggered on, but that can be
      # overriden. Some common overrides would be to use a fresh instance
      # of the base Image (the default in the current system) or to use
      # a premade production snapshot.
      #box: $WERCKER_LAST_BUILD_BOX
      steps:
          # Notable here is that we include additional environment variable
          # definitions for the steps. Step creators may need specific
          # settings, this is where they can be set.
          - s3sync:
              # These settings get exposed to the step under a namespace
              # and prefix, for example this will result in the execution of:
              #   export WERCKER_S3SYNC_KEY_ID="$AWS_ACCESS_KEY_ID"
              # Since this is being executed inside the container, it will
              # use the environment to fill the $AWS_ACCESS_KEY_ID at run time.
              # That environment variable was probably set in the wercker
              # UI and passed into the container prior to this step.
              key_id: $AWS_ACCESS_KEY_ID
              key_secret: $AWS_ACCESS_FOO
              # This would expand to:
              #   export WERCKER_S3SYNC_SOURCE_DIR="build/"
              source_dir: build/
      # After-steps are executed regardless of whether the previous steps have
      # succeeded. They are usually used to notify other systems about build
      # success or failure.
      after-steps:
          - hipchat-notify
              token: $HIPCHAT_TOKEN
              room_id: id
              from-name: name

The `wercker.yml` is expected to be in the root of the project directory.


------
Images
------

Images will now be standardized as Docker images.

We will manage our own repository for pushing user images to as the result of
Jobs.

We will keep images for some amount of time before deleting them.


--------
Services
--------

Services are containers instantiated and linked to our build containers so
that they can be accessed via their public interfaces.

For now, Services are just regular boxes like any other, but we expect to have
private services at some point that are paid upgrades.


----
Jobs
----

Jobs are groupings of Steps executed within the same Container.

The beginnings of a Job are:

 - Environment variables associated with the project and Job provided to the
   job by the user via the wercker UI (or locally if dev).
 - Environment variables provided to the Container:
   - Pass-through the variables provided to the Job by the user.
   - Information about Services that have been linked to the Container.
   - Information about the source code repository.
   - Information about each Step as they are executed.
 - The Image to be used, downloaded if necessary by `sentcli`.
 - The source code fetched by `codefetcher`.
 - The step code, downloaded by `sentcli`.
 - Read-only Volumes attached to the Container containing source and steps.

The results of a Job are:

 - (Production) Entries in the database about metrics (start, stop, usage, etc).
 - (Production) Logs pushed to log storage.
 - (Production) Event notifications about build results.
 - Any files output to $WERCKER_OUTPUT_DIR within the Container.
   These are usually the tarballs of the things that were built.
 - A new image based on committing the container at the end.
 - (Production) The new image pushed to our repository.

In order to communicate with the appropriate APIs in Production, the proper
command-line flags should be set to enable logging, event notifications, and
so forth, with the keys needed to access those resources.


-------------
Jobs (CoreOS)
-------------

Each Job is an individual execution of `sentcli` with all of the information
needed by it passed into the environment via the systemd file.

TODO(termie): Add a template for the systemd job file.


Database Impact
---------------

Shouldn't need to interact with the database directly, all data will be
passed at invocation time. No new data needs to be provided.

When running in production, it needs to report back to the logging and
notification services.

Will attempt to upload artifacts and boxes to S3/otherstorage when keys
are provided.



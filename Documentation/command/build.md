## build

### NAME:
   build - build a project

### USAGE:
   command `build [command options] [arguments...]`

### OPTIONS:
```
   --project-dir "./_projects"                   path where downloaded projects live
   --step-dir "./_steps"                         path where downloaded steps live
   --build-dir "./_builds"                       path where created builds live
   --container-dir "./_containers"               path where exported containers live
   --build-id                                    build id [$WERCKER_BUILD_ID]
   --deploy-id                                   deploy id [$WERCKER_DEPLOY_ID]
   --deploy-target                               deploy target name [$WERCKER_DEPLOYTARGET_NAME]
   --application-id                              application id [$WERCKER_APPLICATION_ID]
   --application-name                            application id [$WERCKER_APPLICATION_NAME]
   --application-owner-name                      application id [$WERCKER_APPLICATION_OWNER_NAME]
   --application-started-by-name                 application started by [$WERCKER_APPLICATION_STARTED_BY_NAME]
   --pipeline                                    alternate pipeline name to execute [$WERCKER_PIPELINE]
   --docker-host "tcp://127.0.0.1:2375"          docker api host [$DOCKER_HOST]
   --docker-tls-verify "0"                       docker api tls verify [$DOCKER_TLS_VERIFY]
   --docker-cert-path                            docker api cert path [$DOCKER_CERT_PATH]
   --direct-mount                                mount our binds read-write to the pipeline path
   --publish [--publish option --publish option] publish a port from the main container, same format as docker --publish
   --attach                                      Attach shell to container if a step fails.
   --git-domain                                  git domain [$WERCKER_GIT_DOMAIN]
   --git-owner                                   git owner [$WERCKER_GIT_OWNER]
   --git-repository                              git repository [$WERCKER_GIT_REPOSITORY]
   --git-branch                                  git branch [$WERCKER_GIT_BRANCH]
   --git-commit                                  git commit [$WERCKER_GIT_COMMIT]
   --commit                                      commit the build result locally
   --tag                                         tag for this build [$WERCKER_GIT_BRANCH]
   --message                                     message for this build
   --artifacts                                   store artifacts
   --no-remove                                   don't remove the containers
   --store-local                                 store artifacts and containers locally
   --store-s3                                    store artifacts and containers on s3
   --aws-secret-key                              secret access key
   --aws-access-key                              access key id
   --s3-bucket "wercker-development"             bucket for artifacts
   --aws-region "us-east-1"                      region
   --source-dir                                  source path relative to checkout root
   --no-response-timeout "5"                     timeout if no script output is received in this many minutes
   --command-timeout "25"                        timeout if command does not complete in this many minutes
   --wercker-yml                                 specify a specific yaml file
   --mnt-root "/mnt"                             directory on the guest where volumes are mounted
   --guest-root "/pipeline"                      directory on the guest where work is done
   --report-root "/report"                       directory on the guest where reports will be written
   --keen-metrics                                report metrics to keen.io
   --keen-project-write-key                      keen write key
   --keen-project-id                             keen project id
   --report                                      Report logs back to wercker (requires build-id, wercker-host, wercker-token)
   --wercker-host                                Wercker host to use for wercker reporter
   --wercker-token                               Wercker token to use for wercker reporter

```

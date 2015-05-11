## pull

### NAME:
   pull - pull <build id>

### USAGE:
   command `pull [command options] [arguments...]`

### DESCRIPTION:
   download a Docker repository, and load it into Docker

### OPTIONS:
```
   --docker-host "tcp://127.0.0.1:2375" docker api host [$DOCKER_HOST]
   --docker-tls-verify "0"              docker api tls verify [$DOCKER_TLS_VERIFY]
   --docker-cert-path                   docker api cert path [$DOCKER_CERT_PATH]
   --branch                             filter on this branch
   --result                             filter on this result (passed or failed)
   --output "./repository.tar"          path to repository
   --load                               load the container into docker after downloading
   -f, --force                          override output if it already exists

```

import docker

c = docker.Client(base_url='tcp://127.0.0.1:4243')

#cid = "dd7a63ea37641905fbb64abb4deaabaa7edd1e871d968d16c1ecdc990f1cd2af"
cid = "b700e1597df7"
cid = "20e3ba64f8fb0d70d9b3fb1903f35be308da687051e77f69bea8473284e853ad"


stream = c.attach_socket(cid, params={'stdin': 1, 'stdout': 1, 'stream': 1},
                         ws=True)

#longrunner = "while true; do eco hello world; sleep 1; done\n"
doom = "echo doom\n"
exports = "export WERCKER_BUILD_ID=14124234534534"
blala = "echo $WERCKER_BUILD_ID"


stream.send(doom)
stream.send(exports)
stream.send(blala)
print stream.recv()


while True:
  print stream.recv()


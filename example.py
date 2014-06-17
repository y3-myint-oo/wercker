import eventlet
eventlet.monkey_patch()

import argparse
import os
import pprint
import shutil
import uuid

import docker
import yaml


class WerckerConfig(dict):
  """Top-level parsed wercker.yml file.

  Contains defaults and validation, services, steps and boxes are
  rich objects as well.
  """

  def __init__(self, data):
    self.data = data
    self.box = WerckerBox(self.data.get('box'))
    self.services = WerckerServices(self.data.get('services'))
    self.build = WerckerBuild(self.data.get('build'))
    self.deploy = WerckerDeploy(self.data.get('deploy'))


class WerckerBox(dict):
  """Metadata about a box."""
  def __init__(self, data):
    self['name'] = data
    self.name = data


class WerckerServices(list):
  """Aggregate of wercker services."""
  pass


class WerckerService(dict):
  """Metadata about a service.

  Will include things like download url.
  """
  pass


class WerckerSteps(list):
  """Collection of steps."""

  def __init__(self, data):
    self.steps = [wercker_step_factory(x) for x in data]


def wercker_step_factory(step):
  """Some steps have special internal implementations, maybe look them up."""
  return step
  if stepname in _SPECIAL_STEPS:
    return _SPECIAL_STEPS[step.name](step)
  return WerckerStep(step)


class WerckerStep(dict):
  """Metadata about a step.

  Will include things like, download url.
  """
  pass


class WerckerBuild(WerckerSteps):
  """Collection of build steps."""

  def get_env(self, options):
    return {'ci': 'True',
            '_build_id': options.buildId,
            '_build_url': settings.build_url(options.buildId)}


class WerckerDeploy(WerckerSteps):
  """Collection of deploy steps."""

  # similar to above
  # ...
  pass



def get_logger():
  """Grab a connection to the logging/notification service."""
  return conn




def load_config(path):
  # ...
  parsed_yaml = yaml.load(open(path))
  return WerckerConfig(parsed_yaml)


def build_environment(config, step):
  """Build up the environment objects."""
  pass




def maybe_pull_image(box, d=None):
  images = d.images()
  #pprint.pprint(images)

  # If we already have the image, don't pull
  for image in images:
    if box.name in image['RepoTags']:
      return image

  # Didn't find the image, pull it down
  image = d.pull(box.name)
  return image


def create_container(image, box=None, volumes=None, d=None):
  default_params = {
    'stdin_open': True,
    'tty': False,
    'command': '/bin/bash',
    'volumes': volumes and volumes or [],
    'name': 'wercker-build-' + uuid.uuid4().hex
  }
  container = d.create_container(image['Id'], **default_params)
  return container


class Session(object):
  """Run commands in a container and keep track of output."""
  def __init__(self, container_id, d=None):
    self.container_id = container_id
    self.d = d
    self._sent = []
    self._send_idx = 0

  def attach(self):
    ws = self.d.attach_socket(self.container_id,
                              params={'stdin': 1,
                                      'stdout': 1,
                                      'stream': 1},
                              ws=True)
    self.ws = ws

    self._recv_queue = eventlet.Queue(0)
    self._recv_thr = eventlet.spawn(self._start_recv)

  def _start_recv(self):
    while True:
      data = self.ws.recv()
      print "recv", data
      self._recv_queue.put(data)

  def send(self, commands):
    if type(commands) is type(""):
      commands = [commands]
    self._sent.append(commands)
    for cmd in commands:
      self.ws.send(cmd + '\n')

  def recv(self):
    while True:
      yield self._recv_queue.get()

# dev only
PROJECT_DIR = './projects'
BUILD_DIR = './builds'

# TODO(termie): actually deal with getting code :p
# TODO(termie): and branches and tags and commits
def checkout_project(project, branch=None, tag=None, commit=None):
  owner, project = project.split('/')
  path = os.path.abspath(os.path.join(PROJECT_DIR, owner, project))

  # NOTE: just shortcircuiting right now for testing
  return path


def cli_build(args):
  project = args.project

  # Set up our build directory, all the things we checkout or
  # supply to the build will come from here
  build_id = uuid.uuid4().hex
  build_path = os.path.abspath(os.path.join(BUILD_DIR, build_id))
  os.makedirs(build_path)

  # Get the code and link it in
  project_path = checkout_project(project)
  checkout_path = os.path.join(build_path, 'source')
  shutil.copytree(project_path, checkout_path)

  # Parse the yaml file
  config = load_config(os.path.join(checkout_path, 'wercker.yml'))
  #pprint.pprint(config.data)


  # Download any steps we need
  # TODO(termie): download some steps and link them into build dir


  # Build list of volumes to mount
  volume_paths = dict(
      (os.path.join('/tmp', x), os.path.join(build_path, x))
      for x in os.listdir(build_path))
  binds = dict((v, {'bind': k, 'ro': True})
               for k, v in volume_paths.iteritems())

  pprint.pprint(volume_paths.keys())
  pprint.pprint(binds)

  #pprint.pprint(config.box)

  # Execute the build
  d = docker.Client(base_url='tcp://127.0.0.1:4243')
  image = maybe_pull_image(config.box, d=d)

  #pprint.pprint(image)

  c = create_container(image, config.box, volumes=volume_paths.keys(), d=d)
  print build_id
  print c

  d.start(c['Id'], binds=binds)

  session = Session(c['Id'], d=d)
  session.attach()
  #session.send(['echo hello world %s' % x for x in range(10)])
  #session.send('echo test')

  #thr = eventlet.spawn(session._start_recv)

  #i = 0
  #for data in session.recv():
  #  print i, data
  #  i += 1




def get_cli():
  parser = argparse.ArgumentParser(description="sentcli")
  subparsers = parser.add_subparsers()
  parser_build = subparsers.add_parser('build')
  parser_build.set_defaults(func=cli_build)
  parser_build.add_argument('project')
  return parser

if __name__ == '__main__':
  cli = get_cli()
  args = cli.parse_args()

  args.func(args)

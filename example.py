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
    self.services = WerckerServices(self.data.get('services', []))
    self.build = WerckerBuild(self.data.get('build', {}))
    self.deploy = WerckerDeploy(self.data.get('deploy', {}))


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


class WerckerBuild(dict):
  """Collection of build steps."""

  def __init__(self, data):
    self.data = data
    self.steps = self._convert_steps(data.get('steps', []))

  def _convert_steps(self, steps):
    """Convert [{step_id: step_content}] to [(step_id, step_content)]."""
    steps_out = [(d.items()[0][0], d.items()[0][1]) for d in steps]
    steps_out.insert(0, ('wercker-init', {}))
    return steps_out

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
    if (box.name in image['RepoTags']
        or '%s:latest' % box.name in image['RepoTags']):
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
                                      'stderr': 1,
                                      'stream': 1},
                              ws=True)
    self.ws = ws

    self._recv_queue = eventlet.Queue(0)
    self._recv_thr = eventlet.spawn(self._start_recv)

  def _start_recv(self):
    line = ''
    while True:
      data = self.ws.recv()
      line += data
      print 'raw', repr(data)
      if '\n' in line:
        parts = line.split('\n')
        line = parts[-1]
        for part in parts[:-1]:
          print "recv", part
          self._recv_queue.put(part)

  def send(self, commands):
    if type(commands) is type(""):
      commands = [commands]
    self._sent.append(commands)
    for cmd in commands:
      print 'send', cmd
      self.ws.send(cmd + '\n')


  def send_checked(self, commands):
    rand_id = uuid.uuid4().hex
    self.send(commands)
    self.send('echo %s $?' % rand_id)

    check = False
    exit_code = None
    recv = []
    while not check:
      line = self.recv().next()
      if not line:
        continue
      line = line.strip()
      if line.startswith('%s ' % rand_id):
        check = True
        exit_code = line[len('%s ' % rand_id):]
      else:
        recv.append(line)
    return (exit_code, recv)


  def recv(self):
    while True:
      yield self._recv_queue.get()

# dev only
PROJECT_DIR = './projects'
BUILD_DIR = './builds'
STEP_DIR = './steps'
CONTAINER_MNT = '/mnt'
CONTAINER_TMP = '/tmp'

# TODO(termie): actually deal with getting code :p
# TODO(termie): and branches and tags and commits
def checkout_project(project, branch=None, tag=None, commit=None):
  owner, project = project.split('/')
  path = os.path.abspath(os.path.join(PROJECT_DIR, owner, project))

  # NOTE: just shortcircuiting right now for testing
  return path


# TODO(termie): actually check out the code :p
def checkout_step(step, branch=None, tag=None, commit=None):
  path = os.path.abspath(os.path.join(STEP_DIR, step))
  # NOTE: just shortcircuits for now
  return path


def build_script_step(build_path, step):
  new_id = uuid.uuid4().hex[:8]
  step_path = os.path.join(build_path, new_id)
  script_path = os.path.join(step_path, 'run.sh')
  os.makedirs(step_path)

  content = _normalize_code(step['code'])
  with open(script_path, 'w') as fp:
    fp.write(content)

  return new_id


def _normalize_step_id(s):
  return s.replace('/', '_')


def _normalize_code(s):
  code = s.split('\n')
  if not code[0].startswith('#!'):
    code.insert(0, '#!/bin/bash -xe')
  return '\n'.join(code)


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
  for step_id, step in config.build.steps:
    #pprint.pprint(step)
    if step_id == 'script':
      new_step_id = build_script_step(build_path, step)
      # Add an ID so that we can find this folder later.
      step['id'] = new_step_id
    else:
      step_path = checkout_step(step_id)
      step_yml_path = os.path.join(step_path, 'wercker-step.yml')
      step_config = None
      if os.path.exists(step_yml_path):
        step_config = yaml.load(open(step_yml_path))

      if step_config and 'properties' in step_config:
        step['_properties'] = step_config['properties']

      shutil.copytree(step_path,
                      os.path.join(build_path, _normalize_step_id(step_id)))


  # Build list of volumes to mount
  volume_paths = dict(
      (os.path.join(CONTAINER_MNT, x), os.path.join(build_path, x))
      for x in os.listdir(build_path))
  binds = dict((v, {'bind': k, 'ro': True})
               for k, v in volume_paths.iteritems())

  pprint.pprint(volume_paths.keys())
  pprint.pprint(binds)

  #pprint.pprint(config.box)

  # Execute the build
  d = docker.Client(base_url='tcp://127.0.0.1:4243')
  image = maybe_pull_image(config.box, d=d)

  pprint.pprint(image)

  c = create_container(image, config.box, volumes=volume_paths.keys(), d=d)
  print build_id
  print c

  d.start(c['Id'], binds=binds)

  session = Session(c['Id'], d=d)
  session.attach()

  #session.send(['echo hello world %s' % x for x in range(10)])
  #session.send('echo test')

  source_mnt = os.path.join(CONTAINER_MNT, 'source')
  source_path = os.path.join(CONTAINER_TMP, 'source')

  session.send(['export TERM=xterm-256color'])
  session.send(['cp -r %s %s' % (source_mnt, source_path)])

  # Copy all the steps
  for step_id, step in config.build.steps:
    if step_id == 'script':
      step_id = step['id']

    mnt_path = os.path.join(CONTAINER_MNT, step_id)
    container_path = os.path.join(CONTAINER_TMP, step_id)
    (exit_code, recv) = session.send_checked('cp -r %s %s' % (mnt_path, container_path))
    print '%s : %s' % (exit_code, recv)


  # Execute the steps
  for step_id, step in config.build.steps:
    if step_id == 'script':
      step_id = step['id']

    container_path = os.path.join(CONTAINER_TMP, step_id)
    init_path = os.path.join(container_path, 'init.sh')
    run_path = os.path.join(container_path, 'run.sh')

    env_template = 'export WERCKER_%(step_id)s_%(key)s="%(value)s"'
    commands = []
    commands.append('export WERCKER_STEP_ROOT="%s"' % container_path)
    for k, v in step.get('_properties', {}).iteritems():
      if k in step:
        value = step.get(k)
      else:
        value = v.get('default', '')

      commands.append(env_template % {'step_id': step_id.upper(),
                                      'key': k.upper(),
                                      'value': value})
    commands.append('cd "%s"' % source_path)
    if os.path.exists(os.path.join(build_path, step_id, 'init.sh')):
      commands.append('source "%s"' % init_path)

    if os.path.exists(os.path.join(build_path, step_id, 'run.sh')):
      commands.append('chmod +x "%s"' % run_path)
      commands.append('source "%s"' % run_path)

    (exit_code, recv) = session.send_checked(commands)
    print '%s : %s' % (exit_code, recv)



  i = 0
  for data in session.recv():
    print i, data
    i += 1




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

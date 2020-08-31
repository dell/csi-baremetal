#!/usr/bin/env python3
# -*- coding: utf-8 -*
import yaml
import time
import argparse
import sys
from os.path import isfile, dirname, basename
from os import makedirs
from shutil import copy
from signal import signal, SIGINT, SIGTERM
from filecmp import cmp, clear_cache
import logging

log = logging.getLogger('patcher')


def run():

    parser = argparse.ArgumentParser(
        description='Patcher script for csi-baremetal kube-extender')
    parser.add_argument(
        '--manifest', help='path to the scheduler manifest file', required=True)
    parser.add_argument(
        '--restore', help='restore manifest when on shutdown', action='store_true')
    parser.add_argument('--interval', type=int,
                        help='interval to check manifest config')
    parser.add_argument('--target-config-path',
                        help='target path for scheduler config file', required=True)
    parser.add_argument('--target-policy-path',
                        help='target path for scheduler policy file', required=True)
    parser.add_argument('--source-config-path',
                        help='source path for scheduler config file', required=True)
    parser.add_argument('--source-policy-path',
                        help='source path for scheduler policy file', required=True)
    parser.add_argument(
        '--loglevel', help="Set level for logging", dest="loglevel", default='info')
    parser.add_argument(
        '--backup-path', help="Set path for backup folder", default='/etc/kubernetes/scheduler')
    args = parser.parse_args()

    logging.basicConfig(level=logging.getLevelName(args.loglevel.upper()))

    log.info('patcher started')

    source_config = File(args.source_config_path)
    source_policy = File(args.source_policy_path)
    target_config = File(args.target_config_path)
    target_policy = File(args.target_policy_path)

    config_volume = Volume("scheduler-config", args.target_config_path)
    config_volume.compile_config()
    policy_volume = Volume("scheduler-policy", args.target_policy_path)
    policy_volume.compile_config()

    manifest = ManifestFile(
        args.manifest, [config_volume, policy_volume], args.target_config_path, args.backup_path)

    # add watcher on signals
    killer = GracefulKiller(args.restore, manifest)
    killer.watch(SIGINT)
    killer.watch(SIGTERM)

    while True:
        # check everything is in a right place
        _must_exist(manifest, source_config, source_policy)

        # copy config and policy if they don't exist or they have different content
        copy_not_equal(source_config, target_config)
        copy_not_equal(source_policy, target_policy)

        # work with content of manifest file
        manifest.load()
        manifest.patch()

        if manifest.changed:
            manifest.backup()
            manifest.flush()
            log.info('manifest file({}) was patched'.format(manifest.path))

        log.debug('sleeping {} seconds'.format(args.interval))
        time.sleep(args.interval)


class GracefulKiller:
    def __init__(self, restore, file):
        self.restore = restore
        self.file = file

    def exit_gracefully(self, signum, frame):
        if self.restore:
            self.file.restore()
            sys.exit(0)

    def watch(self, sig):
        signal(sig, self.exit_gracefully)


class File:
    def __init__(self, path):
        self.path = path

    def copy(self, target_file):
        makedirs(dirname(target_file.path), exist_ok=True)
        copy(self.path, target_file.path)

    def exists(self):
        return isfile(self.path)

    def equal(self, target_file):
        if target_file.exists():
            clear_cache()
            return cmp(self.path, target_file.path)
        return False


class Volume:
    def __init__(self, name, path):
        self.name = name
        self.path = path

    def compile_config(self):
        self.mount_path = {'name': self.name,
                           'mountPath': self.path, 'readOnly': True}
        self.container_volume = {
            'name': self.name,
            'hostPath': {
                'path': self.path,
                'type': 'File'}
        }


class ManifestFile(File):
    def __init__(self, path, volumes, config_path, backup_folder):
        self.path = path
        self.backup_folder = backup_folder
        self.volumes = volumes
        self.config_path = config_path

    def backup(self):
        makedirs(dirname(self.backup_folder), exist_ok=True)
        backup_path = self.backup_folder + basename(self.path)
        copy(self.path, backup_path)

    def restore(self):
        backup_path = self.backup_folder + basename(self.path)
        copy(backup_path, self.path)

    def need_patching(self):
        self.changed = True

    def load(self):
        with open(self.path, 'r') as f:
            content = f.read()
            self.content = yaml.load(content, Loader=yaml.FullLoader)
            log.debug('manifest {} loaded'.format(self.path))
            self.changed = False

    def flush(self):
        with open(self.path, 'w') as f:
            yaml.dump(self.content, f)
            log.debug('manifest {} dumped'.format(self.path))

    def patch_volumes(self):
        volumes = self.content['spec']['volumes']
        volumeMounts = self.content['spec']['containers'][0]['volumeMounts']
        for volume in self.volumes:
            if not _name_exists(volumes, volume.name):
                volumes.append(volume.container_volume)
                self.need_patching()
            if not _name_exists(volumeMounts, volume.name):
                volumeMounts.append(volume.mount_path)
                self.need_patching()

    def patch_commands(self):
        commands = self.content['spec']['containers'][0]['command']
        config_command = '--config={}'.format(self.config_path)
        if config_command not in commands:
            commands.append(config_command)
            self.need_patching()

    def patch(self):
        self.patch_commands()
        self.patch_volumes()


def _name_exists(items, name):
    for i in items:
        if i['name'] == name:
            return True
    return False


def _must_exist(*files):
    for f in files:
        if not f.exists():
            raise FileNotFoundError(
                'One of the required files is not there - {}'.format(f.path))


def copy_not_equal(src, dst):
    if not src.equal(dst):
        src.copy(dst)
        log.info('{} copied to {}'.format(src.path, dst.path))


if __name__ == "__main__":
    run()

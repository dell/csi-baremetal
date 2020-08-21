#!/usr/bin/env python3
# -*- coding: utf-8 -*
from os.path import isfile, dirname
from os import makedirs
from shutil import copy
import yaml
import time
config_path = '/patcher/config.yaml'
policy_path = '/patcher/policy.yaml'
files = {config_path: '/etc/kubernetes/scheduler/config.yaml',
         policy_path: '/etc/kubernetes/scheduler/policy.yaml'}

schedule_config_mount = {'name': 'scheduler-config',
                         'mountPath': '/etc/kubernetes/scheduler/config.yaml', 'readOnly': True}
schedule_config_volume = {
    'name': 'scheduler-config',
    'hostPath': {
            'path': '/etc/kubernetes/scheduler/config.yaml',
            'type': 'File'}
}

schedule_policy_mount = {'name': 'scheduler-policy',
                         'mountPath': '/etc/kubernetes/scheduler/policy.yaml', 'readOnly': True}
schedule_policy_volume = {
    'name': 'scheduler-policy',
    'hostPath': {
            'path': '/etc/kubernetes/scheduler/policy.yaml',
            'type': 'File'}
}
schedule_config = {'volume': schedule_config_volume,
                   'mountPath': schedule_config_mount}
schedule_policy = {'volume': schedule_policy_volume,
                   'mountPath': schedule_policy_mount}


def name_exists(items, name):
    for i in items:
        if i['name'] == name:
            return True
    return False


def patch(filename):
    with open(filename, 'r') as f:
        content = f.read()
        doc = yaml.load(content, Loader=yaml.FullLoader)
    need_patching = False
    print('starting')
    print(type(doc))
    print(doc)
    volumes = doc['spec']['volumes']
    commands = doc['spec']['containers'][0]['command']
    volumeMounts = doc['spec']['containers'][0]['volumeMounts']
    if '--config=/etc/kubernetes/scheduler/config.yaml' not in commands:
        commands.append('--config=/etc/kubernetes/scheduler/config.yaml')
        need_patching = True
    if not name_exists(volumes, schedule_config_volume['name']):
        volumes.append(schedule_config_volume)
        need_patching = True
    if not name_exists(volumeMounts, schedule_config_mount['name']):
        volumeMounts.append(
            schedule_config_mount)
        need_patching = True
    if not name_exists(volumes, schedule_policy_volume['name']):
        volumes.append(schedule_policy_volume)
        need_patching = True
    if not name_exists(volumeMounts, schedule_policy_mount['name']):
        volumeMounts.append(
            schedule_policy_mount)
        need_patching = True
    if need_patching:
        print('file need patching')
        print(doc)
        with open(filename, 'w') as fw:
            yaml.dump(doc, fw)


while(True):
    # first task
    for src, dest in files.items():
        if isfile(src):
            pass
        makedirs(dirname(dest), exist_ok=True)
        copy(src, dest)
    # second task
    patch('/etc/kubernetes/manifests/kube-scheduler.yaml')
    time.sleep(10)

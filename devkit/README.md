# Devkit for CSI-Baremetal
This directory contains source files to build docker image that should be used 
to build [csi-baremetal](https://github.com/dell/csi-baremetal).

#### Build and Installation
- Clone this repository with `git clone git@github.com:dell/csi-baremetal.git`.
- Go to `./csi-baremetal/devkit` directory.
- Build devkit docker image with `make`.
- Install wrapper script with `make install` (optional).
- Use in in accordance with [Usage](#Usage).
- If you do not need devkit anymore uninstall it with `make uninstall && make clean` (optional).

### Usage
- Set environment variables:
  - If you pulled the image from registry:
    - `export DEVKIT_DOCKER_IMAGE_REPO="ACTUAL_DOCKER_REGISTRY"`
  - `export DEVKIT_WORKSPACE_HOST_DIR_PATH=<path to your workspace on the host>`
- `./devkit` (if installed: `devkit`)  

To obtain a list of supported input options execute: `devkit --help`.
You might have a problem with getting input parameters via `devkit --help` in case you have an unusual
environment on the host. In this case execute `make help` to obtain a list of environment variables that
define devkit container's start parameters. If you do not understand what a particular environment variable 
is responsible for, look into the source code of [`devkit`](./devkit) script.

Devkit forwards stdin to bash directly or to the provided command. That is, it can be used within a UNIX pipeline.

**Devkit startup options for different projects:**

|  Project                                         | Startup option                                                                                                  |
|--------------------------------------------------|-----------------------------------------------------------------------------------------------------------------|
| [`csi-baremetal-devkit`](https://github.com/dell/csi-baremetal)                                 | `./devkit` (if installed: `devkit`)                    |

### Configuring IDEs  
Currently devkit supports IntelliJ IDEA and CLion. These IDEs can be started inside devkit. 
- You should set variables as described in `bashrc_templates` for your case.  
- Add `--idea yes` and/or `--clion yes` to startup options for start your IDE. You can also start both when you are inside the devkit by typing `idea/clion` correspondingly.
- By default, it is assumed that the sources of IDE installed to `/opt/<ide_name>`. But it can be configured with setting `DEVKIT_SOFTWARE_HOST_DIR_PATH` to your location. 
- It is also possible that you need to add permissions to this directory to avoid errors:`sudo chmod -R a+rw /opt/<ide_name>`.  

### Files Description
|  File                                            | Description                                                                                                                          |
|--------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| [`Dockerfile`](./Dockerfile)                     | One of the input arguments to `docker build` command (see [Dockerfile reference](https://docs.docker.com/engine/reference/builder)). |
| [`devkit`](./devkit)                             | Wrapper around `docker run` command that hides complex list of arguments from the developer.                                         |
| [`devkit-entrypoint.sh`](./devkit-entrypoint.sh) | Docker image entrypoint. Performs required preparations and starts required services.                                                |
| [`start_ide`](./start_ide)                       | Script to start IDE in a background.                                                                                       |

### Versioning
Devkit follows the semantic versioning approach ([semver.org](http://semver.org/)).
> Given a version number MAJOR.MINOR.PATCH, increment the:
> 1. MAJOR version when you make incompatible API changes,
> 2. MINOR version when you add functionality in a backwards-compatible manner, and
> 3. PATCH version when you make backwards-compatible bug fixes.


### Examples
*Note* that the following examples may be a bit outdated in terms of used environment variables and input options.

#### Developer Opens an Interactive Bash Session
```
user@emc:~> devkit
DEVKIT: docker daemon................[+]
DEVKIT: bash session.................[+]
user@devkit:~/Workspace> # Work hard! 
user@devkit:~/Workspace> exit
```

#### Networking
**Requirements:**  
When IP forwarding is disabled, we can get access to network only with *--net=host*. 
But there is a conflict with *--hostname* option in the Docker version less than 1.11.0 (in 2.1.3 version Devkit can avoid this problem)  
When IP forwarding is enabled, we can get access to network without any changes in Devkit.

```
export DEVKIT_WORKSPACE_HOST_DIR_PATH=/workspace
export DEVKIT_WORKSPACE_SHARED_BOOL=true
export DEVKIT_CACHE_HOST_DIR_PATH=/root/.devkit
export DEVKIT_CACHE_GRADLE_SHARED_BOOL=false
export DEVKIT_CACHE_VAR_LIB_DOCKER_SHARED_BOOL=true
export DEVKIT_SSH_HOST_DIR_PATH=/home/admin/.ssh
export DEVKIT_SSH_SHARED_BOOL=true
export DEVKIT_DOCKER_PSEUDO_TTY_BOOL=false
export DEVKIT_LOG_STDOUT_ENABLED_BOOL=false
...
```

#### To run Devkit with root user
```
export DEVKIT_USER_NAME=root
```

### Contacts
If any problems with this image occur, contact CSI team @dell/csi-baremetal


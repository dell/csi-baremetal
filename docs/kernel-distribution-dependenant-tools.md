Currently we are using specific system calls in the node daemonset, which in some times depends on kernel version.

For example, parted command at Ubuntu18 acts differently from such host OS as Ubuntu20 or SLES15SP3. Under the hood it uses, `udevadm settle` command two times, which has by default timeout in 120 seconds and these commands hangs:

strace results (from CONTAINER):
```
strace -tT udevadm settle
 
03:34:46 close(3)                       = 0 <0.000093>
03:34:46 openat(AT_FDCWD, "/proc/self/stat", O_RDONLY|O_CLOEXEC) = 3 <0.000185>
03:34:46 fstat(3, {st_mode=S_IFREG|0444, st_size=0, ...}) = 0 <0.000036>
03:34:46 read(3, "1187761 (udevadm) R 1187759 1187"..., 1024) = 326 <0.000049>
03:34:46 close(3)                       = 0 <0.000722>
03:34:46 getuid()                       = 0 <0.000472>
03:34:46 socket(AF_UNIX, SOCK_SEQPACKET|SOCK_CLOEXEC|SOCK_NONBLOCK, 0) = 3 <0.000047>
03:34:46 setsockopt(3, SOL_SOCKET, SO_PASSCRED, [1], 4) = 0 <0.000057>
03:34:46 connect(3, {sa_family=AF_UNIX, sun_path="/run/udev/control"}, 19) = 0 <0.000156>
03:34:46 sendto(3, "udev-237\0\0\0\0\0\0\0\0\352\35\255\336\7\0\0\0\0\0\0\0\0\0\0\0"..., 280, 0, NULL, 0) = 280 <0.000055>
03:34:46 poll([{fd=3, events=POLLIN}], 1, 120000   <--- hanging here
```

strace results for same command from host on which container are running:
```
strace -tT udevadm settle
 
00:36:35 getpid()                       = 2822353 <0.000026>
00:36:35 openat(AT_FDCWD, "/proc/self/stat", O_RDONLY|O_CLOEXEC) = 3 <0.000046>
00:36:35 fstat(3, {st_mode=S_IFREG|0444, st_size=0, ...}) = 0 <0.000040>
00:36:35 read(3, "2822353 (udevadm) R 2822350 2822"..., 1024) = 327 <0.000044>
00:36:35 ioctl(3, TCGETS, 0x7fff3bcc2370) = -1 ENOTTY (Inappropriate ioctl for device) <0.000028>
00:36:35 read(3, "", 1024)              = 0 <0.000021>
00:36:35 close(3)                       = 0 <0.000033>
00:36:35 newfstatat(AT_FDCWD, "/proc/1/root", {st_mode=S_IFDIR|0755, st_size=156, ...}, 0) = 0 <0.000039>
00:36:35 newfstatat(AT_FDCWD, "/", {st_mode=S_IFDIR|0755, st_size=156, ...}, 0) = 0 <0.000027>
00:36:35 getuid()                       = 0 <0.000025>
00:36:35 socket(AF_UNIX, SOCK_SEQPACKET|SOCK_CLOEXEC|SOCK_NONBLOCK, 0) = 3 <0.000036>
00:36:35 setsockopt(3, SOL_SOCKET, SO_PASSCRED, [1], 4) = 0 <0.000028>
00:36:35 connect(3, {sa_family=AF_UNIX, sun_path="/run/udev/control"}, 20) = 0 <0.000048>
00:36:35 sendto(3, "udev-246\0\0\0\0\0\0\0\0\352\35\255\336\7\0\0\0\0\0\0\0\0\0\0\0"..., 280, 0, NULL, 0) = 280 <0.000031>
00:36:35 sendto(3, "udev-246\0\0\0\0\0\0\0\0\352\35\255\336\0\0\0\0\0\0\0\0\0\0\0\0"..., 280, 0, NULL, 0) = 280 <0.000028>
00:36:35 epoll_create1(EPOLL_CLOEXEC)   = 4 <0.000028>
00:36:35 gettid()                       = 2822353 <0.000026>
00:36:35 epoll_ctl(4, EPOLL_CTL_ADD, 3, {EPOLLIN, {u32=73863808, u64=94055562678912}}) = 0 <0.000027>
...
```

And so, partprobe has the same udevadm commands executed as child processes.

Currently there is one workaround, which we are using to handle this situation: https://github.com/dell/csi-baremetal/blob/master/docs/proposals/specific-node-kernel-version.md. 
With specific-node-kernel-version we choose the image version, which we need for node daemonset depends on host os version. 
But as we discovered in case of SLES15SP3, we need not only node-kernel but distribution as well, which becomes cumbersome in perspective.

With issue https://github.com/dell/csi-baremetal/issues/656 we moved from udev based tools (parted, partprobe) to not based ones (sgdisk, blockdev).
This changes helps us to support cross kernel host/guest dependency up to now. And currently there is no need in specific-node-kernel-version:
https://github.com/dell/csi-baremetal/issues/660

While completing the issue https://github.com/dell/csi-baremetal/issues/656 we used following versions of udev:

udev version at SLES15SP2:

S  | Name                  | Type    | Version      | Arch   | Repository
---|-----------------------|---------|--------------|--------|------------------------------------
i  | libudev1              | package | 234-24.93.1  | x86_64 | SLE-Module-Basesystem15-SP2-Updates
i  | udev                  | package | 234-24.93.1  | x86_64 | SLE-Module-Basesystem15-SP2-Updates


udev version at SLES15SP3:

S  | Name                  | Type    | Version       | Arch   | Repository
---|-----------------------|---------|---------------|--------|------------------------------------
i  | libudev1              | package | 246.16-7.21.1 | x86_64 | SLE-Module-Basesystem15-SP3-Updates
i  | udev                  | package | 246.16-7.21.1 | x86_64 | SLE-Module-Basesystem15-SP3-Updates

udev based tool worked fine with host OS - SLES15SP2 and guest - Ubuntu18. But with host os SLES15SP3, guest's udev based tools start to hang.

In basic there are some differences which was done to udev package at SLES15SP3. As shown at strace output below, epoll reactor is used insted of poll mechanism at udev settle command.  

In future it's worth to consider completely move away from tools to use directly sysfs, /dev catalogue and /run/udev/data for discovery as it is done in https://github.com/minio/direct-csi
Issue for this proposal: https://github.com/dell/csi-baremetal/issues/661

# ProxyFS Release Notes

## 0.55.0 (October 30, 2017)

### Features:

* Caching of metadata in RAM now configurable
* Samba parameters now specified via the standard /etc/samba/smb.conf mechanism

### Bug Fixes:

* Fixed memory leaks in readdir() APIs issued via SMB
* Fixed metadata on objects set via Swift/S3 API

### Known Issues:

* Named Streams are disabled in SMB (enabling this is TBD)
* Upgrading metadata checkpointing from V1 to V2 experiences process hangs in some cases

## 0.54.1 (October 10, 2017)

### Features:

* Updates to HTTP COALESCE Method
* Improved flushing of affected Swift connections during SIGHUP (reload)
* Improved dataflow during high number of unflushed open file traffic

### Bug Fixes:

* Resolved memory leaks in Samba processes during heavy Robocopy activity
* Resolved potential deadlock for unflushed files that are removed
* Hardened error handling between Samba & ProxyFS processes

### Known Issues:

* Named Streams are disabled in SMB (enabling this is TBD)
* Upgrading metadata checkpointing from V1 to V2 experiences process hangs in some cases

## 0.54.0 (October 3, 2017)

### Features:

* Improved Object ETag MD5 handling
* Object SLO uploads converted to COALESCE'd Objects/Files

### Bug Fixes:

* Non BiModal Accounts remain accessible even when no ProxyFS nodes are available

### Known Issues:

* Named Streams are disabled in SMB (enabling this is TBD)
* Upgrading metadata checkpointing from V1 to V2 experiences process hangs in some cases

## 0.53.0.3 (September 29, 2017)

### Features:

* Added statistics logging

### Bug Fixes:

* Fixed BiModal IP Address reporting following SIGHUP reload
* Fixed issue with large transfers causing Swift API errors

### Known Issues:

* Named Streams are disabled in SMB (enabling this is TBD)
* Upgrading metadata checkpointing from V1 to V2 experiences process hangs in some cases

## 0.53.0.2 (September 19, 2017)

Note: This was just a re-tagging of 0.53.0.1

### Known Issues:

* Named Streams are disabled in SMB (enabling this is TBD)
* Upgrading metadata checkpointing from V1 to V2 experiences process hangs in some cases

## 0.53.0.1 (September 15, 2017)

### Features:

* Added support for Samba version 4.6

### Bug Fixes:

* Fixed memory leak in smbd resulting from a closed TCP connection to proxyfsd

### Known Issues:

* Named Streams are disabled in SMB (enabling this is TBD)
* Upgrading metadata checkpointing from V1 to V2 experiences process hangs in some cases

## 0.53.0 (September 11, 2017)

### Features:

* Added avaibility improvements for ProxyFS Swift clusters to continue when a ProxyFS node is down
* Significantly improved logging during startup and shutdown
* New `mkproxyfs` tool now available to format Volumes (Swift Accounts)

### Bug Fixes:

* Embedded HTTP Server now reports current configuration once startup/restart (SIGHUP) completes
* HTTP Head on ProxyFS-hosted Objects now returns proper HTTPStatus
* Resolved incomplete file locking semantics for SMB
* Resolved issue where a file being written is deleted before its data has been flushed
* Corrected behavior of readdir() enabling callers to bound the size of the returned list
* Corrected permissions checking & metadata updating
* Resolved NFS (FUSE) issue where the underlying file system state failed to reset during restart
* Resolved SMB (smbd) memory leak resulting from unmount/remount sequence

### Known Issues:

* SMB (smbd) memory leaks resulting from restarting the ProxyFS process (proxyfsd) underneath it
* Named Streams are disabled in SMB (enabling this is TBD)
* Upgrading metadata checkpointing from V1 to V2 experiences process hangs in some cases

## 0.52.0 (August 21, 2017)

### Features:

* Support for disabling volumes added

### Bug Fixes:

* Fixed missing flushing of modified files leading to zero-lengthed files

### Known Issues:

* Named Streams are disabled in SMB (enabling this is TBD)
* Upgrading metadata checkpointing from V1 to V2 experiences process hangs in some cases

## 0.51.2 (August 15, 2017)

### Features:

* Improved metadata checkpointing mechanism (V2) performs optimized garbage collection

### Bug Fixes:

* Fixed clean-up of FUSE (and NFS) mount point upon restart after failure
* Fixed memory leaks in SMBd for readdir(), getxattr(), chdir() and list xattr
* Fixed race condition between time-based flushes and on-going write traffic
* Fixed multi-threaded socket management code in resolving DNS names
* Fixed missing support for file names containing special characters

### Known Issues:

* Named Streams are disabled in SMB (enabling this is TBD)
* Upgrading metadata checkpointing from V1 to V2 experiences process hangs in some cases

## 0.51.1 (August 3, 2017)

### Features:

* Enhanced tolerance for intermittent Swift errors
* Read Cache now consumes a configurable percentage of available memory
* Flow Controls now get a weighted fraction of total Read Cache memory
* Configuration reload now supported via SIGHUP signal

### Bug Fixes:

* Fixed embedded HTTP Server handling of "empty" URLs
* Removed memory leaks in SMB handling
* Resolved potential corruption when actively written files are flushed

### Known Issues:

* Memory leak in SMB directory reading and extended attribute reading
* Process restart may leave NFS mount point in a hung state
* Named Streams are disabled in SMB (enabling this is TBD)

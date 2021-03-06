// Package fs, sitting on top of the inode manager, defines the filesystem exposed by ProxyFS.
package fs

// #include <limits.h>
import "C"

import (
	"github.com/swiftstack/ProxyFS/inode"
	"github.com/swiftstack/ProxyFS/stats"
	"github.com/swiftstack/ProxyFS/utils"
)

type MountID uint64

// ReadRangeIn is the ReadPlan range requested
//
// Either Offset or Len can be omitted, but not both. Those correspond
// to HTTP byteranges "bytes=N-" (no Len; asks for byte N to the end
// of the file) and "bytes=-N" (no Offset; asks for the last N bytes
// of the file).
type ReadRangeIn struct {
	Offset *uint64
	Len    *uint64
}

// Returned by MiddlewareGetAccount
type AccountEntry struct {
	Basename string
}

// Returned by MiddlewareGetContainer
//
type ContainerEntry struct {
	Basename         string
	FileSize         uint64
	ModificationTime uint64 // nanoseconds since epoch
	IsDir            bool
	NumWrites        uint64
	InodeNumber      uint64
	Metadata         []byte
}

type HeadResponse struct {
	Metadata         []byte
	FileSize         uint64
	ModificationTime uint64 // nanoseconds since epoch
	IsDir            bool
	InodeNumber      inode.InodeNumber
	NumWrites        uint64
}

// The following constants are used to ensure that the length of file fullpath and basenames are POSIX-compliant
const (
	FilePathMax = C.PATH_MAX
	FileNameMax = C.NAME_MAX
)

// The maximum number of symlinks we will follow
const MaxSymlinks = 8 // same as Linux; see include/linux/namei.h in Linux's Git repository

// Constant defining the name of the alternate data stream used by Swift Middleware
const MiddlewareStream = "middleware"

// Byte prefix constants
const (
	KiloByte = 1024
	MegaByte = KiloByte * 1024
	GigaByte = MegaByte * 1024
	TeraByte = GigaByte * 1024
)

// The following constants are used when responding to StatVfs calls
const (
	FsBlockSize           = 64 * KiloByte
	FsOptimalTransferSize = 64 * KiloByte
)

// XXX TBD: For now we are cooking up these numbers...
const (
	VolFakeTotalBlocks = TeraByte / FsBlockSize
	VolFakeFreeBlocks  = TeraByte / FsBlockSize
	VolFakeAvailBlocks = TeraByte / FsBlockSize

	VolFakeTotalInodes = TeraByte
	VolFakeFreeInodes  = TeraByte
	VolFakeAvailInodes = TeraByte
)

type FlockStruct struct {
	Type   int32
	Whence int32
	Start  uint64
	Len    uint64
	Pid    uint64
}

type MountOptions uint64

const (
	MountReadOnly MountOptions = 1 << iota
)

type StatKey uint64

const (
	StatCTime     StatKey = iota + 1 // time of last inode attribute change (ctime in posix stat)
	StatCRTime                       // time of inode creation              (crtime in posix stat)
	StatMTime                        // time of last data modification      (mtime in posix stat)
	StatATime                        // time of last data access            (atime in posix stat)
	StatSize                         // inode data size in bytes
	StatNLink                        // Number of hard links to the inode
	StatFType                        // file type of inode
	StatINum                         // inode number
	StatMode                         // file mode
	StatUserID                       // file userid
	StatGroupID                      // file groupid
	StatNumWrites                    // number of writes to inode
)

// XXX TODO: StatMode, StatUserID, and StatGroupID are really
//           uint32, not uint64. How to expose a stat map with
//           values of different types?
type Stat map[StatKey]uint64 // key is one of StatKey consts

// Whole-filesystem stats for StatVfs calls
//
type StatVFSKey uint64

const (
	StatVFSBlockSize      StatVFSKey = iota + 1 // statvfs.f_bsize - Filesystem block size
	StatVFSFragmentSize                         // statvfs.f_frsize - Filesystem fragment size, smallest addressable data size in the filesystem
	StatVFSTotalBlocks                          // statvfs.f_blocks - Filesystem size in StatVFSFragmentSize units
	StatVFSFreeBlocks                           // statvfs.f_bfree - number of free blocks
	StatVFSAvailBlocks                          // statvfs.f_bavail - number of free blocks for unprivileged users
	StatVFSTotalInodes                          // statvfs.f_files - number of inodes in the filesystem
	StatVFSFreeInodes                           // statvfs.f_ffree - number of free inodes in the filesystem
	StatVFSAvailInodes                          // statvfs.f_favail - number of free inodes for unprivileged users
	StatVFSFilesystemID                         // statvfs.f_fsid  - Our filesystem ID
	StatVFSMountFlags                           // statvfs.f_flag  - mount flags
	StatVFSMaxFilenameLen                       // statvfs.f_namemax - maximum filename length
)

type StatVFS map[StatVFSKey]uint64 // key is one of StatVFSKey consts

// Mount handle interface

func Mount(volumeName string, mountOptions MountOptions) (mountHandle MountHandle, err error) {
	mountHandle, err = mount(volumeName, mountOptions)
	return
}

type MountHandle interface {
	Access(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, accessMode inode.InodeMode) (accessReturn bool)
	CallInodeToProvisionObject() (pPath string, err error)
	Create(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, dirInodeNumber inode.InodeNumber, basename string, filePerm inode.InodeMode) (fileInodeNumber inode.InodeNumber, err error)
	Flush(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber) (err error)
	Flock(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, lockCmd int32, inFlockStruct *FlockStruct) (outFlockStruct *FlockStruct, err error)
	Getstat(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber) (stat Stat, err error)
	GetType(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber) (inodeType inode.InodeType, err error)
	GetXAttr(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, streamName string) (value []byte, err error)
	IsDir(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber) (inodeIsDir bool, err error)
	IsFile(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber) (inodeIsFile bool, err error)
	IsSymlink(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber) (inodeIsSymlink bool, err error)
	Link(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, dirInodeNumber inode.InodeNumber, basename string, targetInodeNumber inode.InodeNumber) (err error)
	ListXAttr(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber) (streamNames []string, err error)
	Lookup(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, dirInodeNumber inode.InodeNumber, basename string) (inodeNumber inode.InodeNumber, err error)
	LookupPath(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, fullpath string) (inodeNumber inode.InodeNumber, err error)
	MiddlewareCoalesce(destPath string, elementPaths []string) (ino uint64, numWrites uint64, modificationTime uint64, err error)
	MiddlewareDelete(parentDir string, baseName string) (err error)
	MiddlewareGetAccount(maxEntries uint64, marker string) (accountEnts []AccountEntry, err error)
	MiddlewareGetContainer(vContainerName string, maxEntries uint64, marker string, prefix string) (containerEnts []ContainerEntry, err error)
	MiddlewareGetObject(volumeName string, containerObjectPath string, readRangeIn []ReadRangeIn, readRangeOut *[]inode.ReadPlanStep) (fileSize uint64, lastModified uint64, ino uint64, numWrites uint64, serializedMetadata []byte, err error)
	MiddlewareHeadResponse(entityPath string) (response HeadResponse, err error)
	MiddlewarePost(parentDir string, baseName string, newMetaData []byte, oldMetaData []byte) (err error)
	MiddlewareMkdir(vContainerName string, vObjectPath string, metadata []byte) (mtime uint64, inodeNumber inode.InodeNumber, numWrites uint64, err error)
	MiddlewarePutComplete(vContainerName string, vObjectPath string, pObjectPaths []string, pObjectLengths []uint64, pObjectMetadata []byte) (mtime uint64, fileInodeNumber inode.InodeNumber, numWrites uint64, err error)
	MiddlewarePutContainer(containerName string, oldMetadata []byte, newMetadata []byte) (err error)
	Mkdir(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, basename string, filePerm inode.InodeMode) (newDirInodeNumber inode.InodeNumber, err error)
	RemoveXAttr(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, streamName string) (err error)
	Rename(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, srcDirInodeNumber inode.InodeNumber, srcBasename string, dstDirInodeNumber inode.InodeNumber, dstBasename string) (err error)
	Read(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, offset uint64, length uint64, profiler *utils.Profiler) (buf []byte, err error)
	Readdir(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, prevBasenameReturned string, maxEntries uint64, maxBufSize uint64) (entries []inode.DirEntry, numEntries uint64, areMoreEntries bool, err error)
	ReaddirOne(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, prevDirLocation inode.InodeDirLocation) (entries []inode.DirEntry, err error)
	ReaddirPlus(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, prevBasenameReturned string, maxEntries uint64, maxBufSize uint64) (dirEntries []inode.DirEntry, statEntries []Stat, numEntries uint64, areMoreEntries bool, err error)
	ReaddirOnePlus(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, prevDirLocation inode.InodeDirLocation) (dirEntries []inode.DirEntry, statEntries []Stat, err error)
	Readsymlink(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber) (target string, err error)
	Resize(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, newSize uint64) (err error)
	Rmdir(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, basename string) (err error)
	Setstat(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, stat Stat) (err error)
	SetXAttr(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, streamName string, value []byte, flags int) (err error)
	StatVfs() (statVFS StatVFS, err error)
	Symlink(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, basename string, target string) (symlinkInodeNumber inode.InodeNumber, err error)
	Unlink(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, basename string) (err error)
	Validate(inodeNumber inode.InodeNumber) (err error)
	VolumeName() (volumeName string)
	Write(userID inode.InodeUserID, groupID inode.InodeGroupID, otherGroupIDs []inode.InodeGroupID, inodeNumber inode.InodeNumber, offset uint64, buf []byte, profiler *utils.Profiler) (size uint64, err error)
}

// Utility functions

func ValidateBaseName(baseName string) (err error) {
	err = validateBaseName(baseName)
	return
}

func ValidateFullPath(fullPath string) (err error) {
	err = validateFullPath(fullPath)
	return
}

func ValidateVolume(volumeName string, stopChan chan bool, errChan chan error) {
	stats.IncrementOperations(&stats.FsVolumeValidateOps)
	errChan <- validateVolume(volumeName, stopChan)
}

func AccountNameToVolumeName(accountName string) (volumeName string, ok bool) {
	volumeName, ok = inode.AccountNameToVolumeName(accountName)
	stats.IncrementOperations(&stats.FsAcctToVolumeOps)
	return
}

func VolumeNameToActivePeerPrivateIPAddr(volumeName string) (activePeerPrivateIPAddr string, ok bool) {
	activePeerPrivateIPAddr, ok = inode.VolumeNameToActivePeerPrivateIPAddr(volumeName)
	stats.IncrementOperations(&stats.FsVolumeToActivePeerOps)
	return
}

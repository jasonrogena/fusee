package fs

import (
	"hash/fnv"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/jasonrogena/fusee/internal/pkg/command"
)

func generateInodeNumber(path string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(path))
	return h.Sum64()
}

func GetDirectoryStableAttr(commandState *command.State) fs.StableAttr {
	return fs.StableAttr{
		Ino:  generateInodeNumber(commandState.MountRootDirPath + commandState.RelativePath),
		Mode: syscall.S_IFDIR,
	}
}

func GetFileStableAttr(commandState *command.State) fs.StableAttr {
	return fs.StableAttr{
		Ino:  generateInodeNumber(commandState.MountRootDirPath + commandState.RelativePath),
		Mode: syscall.S_IFREG,
	}
}

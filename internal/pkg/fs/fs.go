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

func GetDirectoryStableAttr(commandContext command.Context) fs.StableAttr {
	return fs.StableAttr{
		Ino:  generateInodeNumber(commandContext.MountRootDirPath + commandContext.RelativePath),
		Mode: syscall.S_IFDIR,
	}
}

func GetFileStableAttr(commandContext command.Context) fs.StableAttr {
	return fs.StableAttr{
		Ino:  generateInodeNumber(commandContext.MountRootDirPath + commandContext.RelativePath),
		Mode: syscall.S_IFREG,
	}
}

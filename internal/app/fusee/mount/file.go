package mount

import (
	"context"
	"os"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/jasonrogena/fusee/internal/app/fusee/config"
	"github.com/jasonrogena/fusee/internal/pkg/command"
	log "github.com/sirupsen/logrus"
)

type file struct {
	fs.Inode
	config         config.File
	commandContext command.Context
	atime          time.Time
	ctime          time.Time
	mtime          time.Time
	content        []byte
}

func NewFile(config config.File, commandContext command.Context) *file {
	return &file{
		config:         config,
		commandContext: commandContext,
	}
}

func (f *file) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	log.Debug("Read called on file")
	end := off + int64(len(dest))

	if end > int64(len(f.content)) {
		end = int64(len(f.content))
	}

	f.atime = time.Now()
	return fuse.ReadResultData(f.content[off:end]), 0
}

func (f *file) Open(ctx context.Context, openFlags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Debug("Open called for file")
	if isContentStale(f) {
		log.Info("Running command to get contents for ",
			f.commandContext.MountRootDirPath+string(os.PathSeparator)+f.commandContext.RelativePath)
		output, outputErr := command.Run(f.config.ReadCommand, f.commandContext)
		if outputErr != nil {
			log.Error(outputErr.Error())
		}
		f.content = output
		f.mtime = time.Now()
	}

	return f, fuse.FOPEN_DIRECT_IO, 0
}

func (f *file) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	log.Debug("Getaddr called for file")
	out.Mode = f.config.Mode
	out.Atime = uint64(f.atime.Unix())
	out.Mtime = uint64(f.mtime.Unix())
	out.Ctime = uint64(f.ctime.Unix())
	return 0
}

func (f *file) OnAdd(ctx context.Context) {
	log.Debug("OnAdd called on file")
	curTime := time.Now()
	f.mtime = curTime
	f.ctime = curTime
	f.atime = curTime
}

func (f *file) Release(ctx context.Context) syscall.Errno {
	log.Debug("Release called for file")
	if isContentStale(f) {
		log.Debug("File content is stale, clearing cache")
		f.content = []byte{}
	}

	return 0
}

func (f *file) getMtime() time.Time {
	return f.mtime
}

func (f *file) getCtime() time.Time {
	return f.ctime
}

func (f *file) getCacheSeconds() float64 {
	return f.config.CacheSeconds
}

func (f *file) shouldCache() bool {
	return f.config.Cache
}

var _ = (fs.InodeEmbedder)((*file)(nil))
var _ = (fs.FileHandle)((*file)(nil))
var _ = (fs.FileReader)((*file)(nil))    // Contains Read
var _ = (fs.FileGetattrer)((*file)(nil)) // Contains Getattr
var _ = (fs.NodeOnAdder)((*file)(nil))   // Contains OnAdd
var _ = (fs.FileReleaser)((*file)(nil))  // Contains Release
var _ = (fs.NodeOpener)((*file)(nil))    // Contains Open

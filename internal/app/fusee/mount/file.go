package mount

import (
	"context"
	"os"
	"sync"
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
	config            config.File
	commandState      *command.State
	attr              *fuse.Attr
	content           []byte
	commandRunnerPool *command.Pool
}

func NewFile(config config.File, commandState *command.State, commandRunnerPool *command.Pool) *file {
	return &file{
		config:            config,
		commandState:      commandState,
		commandRunnerPool: commandRunnerPool,
	}
}

func (f *file) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	log.Debug("Read called on file")
	end := off + int64(len(dest))

	if end > int64(len(f.content)) {
		end = int64(len(f.content))
	}

	f.attr.Atime = uint64(time.Now().Unix())
	return fuse.ReadResultData(f.content[off:end]), 0
}

func (f *file) Open(ctx context.Context, openFlags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Debug("Open called for file")
	if isContentStale(f) {
		var wg sync.WaitGroup
		wg.Add(1)
		log.Info("Running command to get contents for ",
			f.commandState.MountRootDirPath+string(os.PathSeparator)+f.commandState.RelativePath)
		f.commandRunnerPool.AddCommand(command.NewCommand(f.config.ReadCommand, f.commandState, func(output []byte, outputErr error) {
			if outputErr != nil {
				log.Error(outputErr.Error())
			}
			f.content = output
			f.attr.Mtime = uint64(time.Now().Unix())
			wg.Done()
		}))
		wg.Wait()
	}

	return f, fuse.FOPEN_DIRECT_IO, 0
}

func (f *file) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	log.Debug("Getaddr called for file")
	f.getattr(out)
	return 0
}

func (f *file) getattr(out *fuse.AttrOut) {
	out.Mode = f.config.Mode
	out.Mtime = f.attr.Mtime
	out.Ctime = f.attr.Ctime
	out.Atime = f.attr.Atime
}

func (f *file) OnAdd(ctx context.Context) {
	log.Debug("OnAdd called on file")
	curTime := time.Now()
	f.attr = &fuse.Attr{}
	f.attr.Atime = uint64(curTime.Unix())
	f.attr.Ctime = uint64(curTime.Unix())
	f.attr.Mtime = uint64(curTime.Unix())
}

func (f *file) Release(ctx context.Context) syscall.Errno {
	log.Debug("Release called for file")
	if isContentStale(f) {
		log.Debug("File content is stale, clearing cache")
		f.content = []byte{}
	}

	return 0
}

func (f *file) getAttr() *fuse.Attr {
	return f.attr
}

func (f *file) getCacheSeconds() uint64 {
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

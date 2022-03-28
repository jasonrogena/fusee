package mount

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/jasonrogena/fusee/internal/app/fusee/config"
	"github.com/jasonrogena/fusee/internal/pkg/command"
	log "github.com/sirupsen/logrus"
)

type root struct {
	fs.Inode
	config              config.Mount
	name                string
	readDirCounter      int
	attr                *fuse.Attr
	dirEntries          []fuse.DirEntry
	dirEntryPointer     int
	commandRunnerPool   *command.Pool
	cachedTestRunOutput []byte
}

func NewRoot(name string, conf config.Mount) *root {
	return &root{
		config:              conf,
		name:                name,
		cachedTestRunOutput: []byte{},
	}
}

func (r *root) Mount(debug bool) error {
	opts := &fs.Options{}
	// opts.Debug = debug

	log.Debug(fmt.Sprintf("Beginning the mounting process for '%s'", r.name))
	server, serverErr := fs.Mount(r.config.Path, r, opts)
	log.Debug(fmt.Sprintf("fs.Mount called for '%s'. About to start waiting for mount", r.name))
	server.Wait()
	return serverErr
}

func (r *root) OnAdd(ctx context.Context) {
	curTime := time.Now()
	r.attr = &fuse.Attr{}
	r.attr.SetTimes(&curTime, &curTime, &curTime)
	noThreads := r.config.ThreadCount
	if noThreads == 0 {
		noThreads = uint(runtime.NumCPU())
	}
	r.commandRunnerPool = command.NewPool(int(noThreads))
	r.commandRunnerPool.Start()
	var wg sync.WaitGroup
	err := loadChildren(ctx, r, &wg)
	wg.Wait()
	if err != nil {
		log.Error(err.Error())
	}
}

func (r *root) getAttr() *fuse.Attr {
	return r.attr
}

func (r *root) isContentStale() bool {
	return isContentStale(r)
}

func (r *root) getCacheSeconds() uint64 {
	return r.config.CacheSeconds
}

func (r *root) shouldCache() bool {
	return r.config.Cache
}

func (r *root) getDirectoryConfig() config.Directory {
	return r.config.Directory
}

func (r *root) getFileConfig() config.File {
	return r.config.File
}

func (r *root) getInode() *fs.Inode {
	return &r.Inode
}

func (r *root) getReadCommand() (string, error) {
	if len(r.config.ReadCommand) > 0 {
		return r.config.ReadCommand, nil
	}
	if len(r.config.Directory.ReadCommand) > 0 {
		return r.config.Directory.ReadCommand, nil
	}

	return "", errors.New("Read command not provided for mount root")
}

func (r *root) getNameSeparator() (string, error) {
	if len(r.config.NameSeparator) > 0 {
		return r.config.NameSeparator, nil
	}
	if len(r.config.Directory.NameSeparator) > 0 {
		return r.config.Directory.NameSeparator, nil
	}

	return "", errors.New("Name separator not provided for mount root")
}

func (r *root) getCommandState() *command.State {
	return command.NewState(r.name, r.config.Path, "", "")
}

func (r *root) getCommandRunnerPool() *command.Pool {
	return r.commandRunnerPool
}

func (r *root) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	r.getattr(out)
	return 0
}

func (r *root) getattr(out *fuse.AttrOut) {
	out.Mode = r.config.Mode
	out.Mtime = r.attr.Mtime
	out.Ctime = r.attr.Ctime
	out.Atime = r.attr.Atime
}

func (r *root) Release(ctx context.Context, f fs.FileHandle) syscall.Errno {
	r.commandRunnerPool.Stop()
	// TODO: umount
	return 0
}

func (r *root) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	log.Debug("Readdir called for root")
	var wg sync.WaitGroup
	loadErr := loadChildren(ctx, r, &wg)
	wg.Wait()
	if loadErr != nil {
		log.Error(loadErr.Error())
	}
	r.attr.Atime = uint64(time.Now().Unix())
	for childName, chidInode := range r.Children() {
		r.dirEntries = append(r.dirEntries, fuse.DirEntry{
			Name: childName,
			Ino:  chidInode.StableAttr().Ino,
			Mode: chidInode.Mode(),
		})
	}
	return r, 0
}

func (r *root) getCachedTestRunOutput() []byte {
	return r.cachedTestRunOutput
}

func (r *root) setCachedTestRunOutput(testRunOutput []byte) {
	r.cachedTestRunOutput = testRunOutput
}

func (r *root) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Debug("Lookup called for root")
	return lookupChild(ctx, r, name)
}

func (r *root) getChildren() map[string]*fs.Inode {
	return r.Children()
}

func (r *root) HasNext() bool {
	return r.dirEntryPointer < len(r.dirEntries)
}

func (r *root) Next() (fuse.DirEntry, syscall.Errno) {
	nextEntry := r.dirEntries[r.dirEntryPointer]
	r.dirEntryPointer++
	return nextEntry, 0
}

func (r *root) Close() {
	log.Debug("Close called for root")
	r.dirEntries = []fuse.DirEntry{}
	r.dirEntryPointer = 0
}

var _ = (fs.NodeGetattrer)((*root)(nil)) // Contains Getattr
var _ = (fs.NodeOnAdder)((*root)(nil))   // Contains OnAdd
var _ = (fs.DirStream)((*root)(nil))     // Contains HasNext, Next, and Close
var _ = (fs.NodeLookuper)((*root)(nil))  // Contains Lookup
var _ = (fs.NodeReaddirer)((*root)(nil)) // Contains Readdir
var _ = (fs.NodeReleaser)((*root)(nil))  // Contains Release

package mount

import (
	"context"
	"errors"
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
	config          config.Mount
	name            string
	readDirCounter  int
	atime           time.Time
	ctime           time.Time
	mtime           time.Time
	dirEntries      []fuse.DirEntry
	dirEntryPointer int
}

func NewRoot(name string, conf config.Mount) *root {
	return &root{
		config: conf,
		name:   name,
	}
}

func (r *root) Mount(debug bool) error {
	opts := &fs.Options{}
	// opts.Debug = debug

	log.Debug("Beginning the mounting process for '%s'", r.name)
	server, serverErr := fs.Mount(r.config.Path, r, opts)
	log.Debug("fs.Mount called for '%s'. About to start waiting for mount", r.name)
	server.Wait()
	return serverErr
}

func (r *root) OnAdd(ctx context.Context) {
	err := loadChildren(ctx, r)
	curTime := time.Now()
	r.mtime = curTime
	r.ctime = curTime
	r.atime = curTime
	if err != nil {
		log.Error(err.Error())
	}
}

func (r *root) setMtime(newTime time.Time) {
	r.mtime = newTime
}

func (r *root) isContentStale() bool {
	return isContentStale(r)
}

func (r *root) getMtime() time.Time {
	return r.mtime
}

func (r *root) getCtime() time.Time {
	return r.ctime
}

func (r *root) getCacheSeconds() float64 {
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

func (r *root) getCommandContext() command.Context {
	return command.Context{
		MountName:        r.name,
		MountRootDirPath: r.config.Path,
		RelativePath:     "",
		Name:             "",
	}
}

func (r *root) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = r.config.Mode
	out.Atime = uint64(r.atime.Unix())
	out.Mtime = uint64(r.mtime.Unix())
	out.Ctime = uint64(r.ctime.Unix())
	return 0
}

func (r *root) Release(ctx context.Context, f fs.FileHandle) syscall.Errno {
	// TODO: release persistent SSH connection if any
	// TODO: umount
	return 0
}

func (r *root) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	log.Debug("Readdir called for root")
	loadErr := loadChildren(ctx, r)
	if loadErr != nil {
		log.Error(loadErr.Error())
	}

	for childName, chidInode := range r.Children() {
		r.dirEntries = append(r.dirEntries, fuse.DirEntry{
			Name: childName,
			Ino:  chidInode.StableAttr().Ino,
			Mode: chidInode.Mode(),
		})
	}
	return r, 0
}

func (r *root) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Debug("Lookup called for root")
	loadErr := loadChildren(ctx, r)
	if loadErr != nil {
		log.Error(loadErr.Error())
	}
	child, childFound := r.Children()[name]
	if childFound {
		return child, 0
	}

	return nil, syscall.ENOENT
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

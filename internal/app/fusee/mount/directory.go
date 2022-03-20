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

type directory struct {
	fs.Inode
	dirConfig       config.Directory
	fileConfig      config.File
	commandContext  command.Context
	atime           time.Time
	ctime           time.Time
	mtime           time.Time
	dirEntries      []fuse.DirEntry
	dirEntryPointer int
}

func NewDirectory(dirConfig config.Directory, fileConfig config.File, commandContext command.Context) *directory {
	return &directory{
		commandContext: commandContext,
		dirConfig:      dirConfig,
		fileConfig:     fileConfig,
	}
}

func (d *directory) getDirectoryConfig() config.Directory {
	return d.dirConfig
}

func (d *directory) getFileConfig() config.File {
	return d.fileConfig
}

func (d *directory) getInode() *fs.Inode {
	return &d.Inode
}

func (d *directory) getReadCommand() (string, error) {
	if len(d.dirConfig.ReadCommand) > 0 {
		return d.dirConfig.ReadCommand, nil
	}

	return "", errors.New("Read command not provided for directory")
}

func (d *directory) getNameSeparator() (string, error) {
	if len(d.dirConfig.NameSeparator) > 0 {
		return d.dirConfig.NameSeparator, nil
	}

	return "", errors.New("Name separator not provided for directory")
}

func (d *directory) getCommandContext() command.Context {
	return d.commandContext
}

func (d *directory) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Debug("Getattr called for directory")
	out.Mode = d.dirConfig.Mode
	out.Atime = uint64(d.atime.Unix())
	out.Mtime = uint64(d.mtime.Unix())
	out.Ctime = uint64(d.ctime.Unix())
	return 0
}

func (d *directory) HasNext() bool {
	return d.dirEntryPointer < len(d.dirEntries)
}

func (d *directory) Next() (fuse.DirEntry, syscall.Errno) {
	nextEntry := d.dirEntries[d.dirEntryPointer]
	d.dirEntryPointer++
	return nextEntry, 0
}

func (d *directory) Close() {
	log.Debug("Close called for directory")
	d.dirEntries = []fuse.DirEntry{}
	d.dirEntryPointer = 0
}

func (d *directory) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Debug("Lookup called for directory")
	loadErr := loadChildren(ctx, d)
	if loadErr != nil {
		log.Error(loadErr.Error())
	}
	child, childFound := d.Children()[name]
	if childFound {
		return child, 0
	}

	return nil, syscall.ENOENT
}

func (d *directory) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	log.Debug("Readdir called for directory")
	loadErr := loadChildren(ctx, d)
	if loadErr != nil {
		log.Error(loadErr.Error())
	}

	for childName, chidInode := range d.Children() {
		d.dirEntries = append(d.dirEntries, fuse.DirEntry{
			Name: childName,
			Ino:  chidInode.StableAttr().Ino,
			Mode: chidInode.Mode(),
		})
	}
	return d, 0
}

func (d *directory) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Debug("Open called for directory")
	loadErr := loadChildren(ctx, d)
	if loadErr != nil {
		log.Error(loadErr.Error())
	}
	return d, fuse.FOPEN_DIRECT_IO, 0
}

func (d *directory) getMtime() time.Time {
	return d.mtime
}

func (d *directory) getCtime() time.Time {
	return d.ctime
}

func (d *directory) getCacheSeconds() float64 {
	return d.dirConfig.CacheSeconds
}

func (d *directory) shouldCache() bool {
	return d.dirConfig.Cache
}

func (d *directory) setMtime(newTime time.Time) {
	d.mtime = newTime
}

func (d *directory) isContentStale() bool {
	return isContentStale(d)
}

var _ = (fs.InodeEmbedder)((*directory)(nil))

var _ = (fs.NodeGetattrer)((*directory)(nil)) // Contains Getattr
var _ = (fs.DirStream)((*directory)(nil))     // Contains HasNext, Next, and Close
var _ = (fs.NodeLookuper)((*directory)(nil))  // Contains Lookup
var _ = (fs.NodeReaddirer)((*directory)(nil)) // Contains Readdir
var _ = (fs.NodeOpener)((*directory)(nil))    // Contains Open

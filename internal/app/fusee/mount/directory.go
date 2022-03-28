package mount

import (
	"context"
	"errors"
	"sync"
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
	dirConfig         config.Directory
	fileConfig        config.File
	commandState      *command.State
	attr              *fuse.Attr
	dirEntries        []fuse.DirEntry
	dirEntryPointer   int
	commandRunnerPool *command.Pool
	// Variable is used to store the output created when this directory's parent runs the
	// directory command against this directory's name to test whether it is a file or directory.
	// We cache the output from the test so that incase ReadDir is called against this directory
	// before its atime expires we just build its dirents using the cached test run output.
	cachedTestRunOutput []byte
}

func NewDirectory(dirConfig config.Directory, fileConfig config.File, cachedTestRunOutput []byte, commandState *command.State, commandRunnerPool *command.Pool) *directory {
	return &directory{
		commandState:        commandState,
		dirConfig:           dirConfig,
		fileConfig:          fileConfig,
		commandRunnerPool:   commandRunnerPool,
		cachedTestRunOutput: cachedTestRunOutput,
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

func (d *directory) getCommandState() *command.State {
	return d.commandState
}

func (d *directory) getCommandRunnerPool() *command.Pool {
	return d.commandRunnerPool
}

func (d *directory) getCachedTestRunOutput() []byte {
	return d.cachedTestRunOutput
}

func (d *directory) setCachedTestRunOutput(testRunOutput []byte) {
	d.cachedTestRunOutput = testRunOutput
}

func (d *directory) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Debug("Getattr called for directory")
	d.getattr(out)
	return 0
}

func (d *directory) getattr(out *fuse.AttrOut) {
	out.Mode = d.dirConfig.Mode
	out.Mtime = d.attr.Mtime
	out.Ctime = d.attr.Ctime
	out.Atime = d.attr.Atime
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

func (d *directory) getChildren() map[string]*fs.Inode {
	return d.Children()
}

func (d *directory) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Debug("Lookup called for directory")
	return lookupChild(ctx, d, name)
}

func (d *directory) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	log.Debug("Readdir called for directory")
	var wg sync.WaitGroup
	loadErr := loadChildren(ctx, d, &wg)
	wg.Wait()
	if loadErr != nil {
		log.Error(loadErr.Error())
	}
	d.attr.Atime = uint64(time.Now().Unix())
	for childName, chidInode := range d.Children() {
		d.dirEntries = append(d.dirEntries, fuse.DirEntry{
			Name: childName,
			Ino:  chidInode.StableAttr().Ino,
			Mode: chidInode.Mode(),
		})
	}
	return d, 0
}

func (d *directory) getAttr() *fuse.Attr {
	return d.attr
}

func (d *directory) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Debug("Open called for directory")
	var wg sync.WaitGroup
	loadErr := loadChildren(ctx, d, &wg)
	wg.Wait()
	if loadErr != nil {
		log.Error(loadErr.Error())
	}
	return d, fuse.FOPEN_DIRECT_IO, 0
}

func (d *directory) getCacheSeconds() uint64 {
	return d.dirConfig.CacheSeconds
}

func (d *directory) shouldCache() bool {
	return d.dirConfig.Cache
}

func (d *directory) isContentStale() bool {
	return isContentStale(d)
}

func (d *directory) OnAdd(ctx context.Context) {
	log.Debug("OnAdd called on directory")
	curTime := time.Now()
	d.attr = &fuse.Attr{}
	d.attr.Atime = uint64(curTime.Unix())
	d.attr.Ctime = uint64(curTime.Unix())
	d.attr.Mtime = uint64(curTime.Unix())
}

var _ = (fs.InodeEmbedder)((*directory)(nil))
var _ = (fs.NodeGetattrer)((*directory)(nil)) // Contains Getattr
var _ = (fs.DirStream)((*directory)(nil))     // Contains HasNext, Next, and Close
var _ = (fs.NodeLookuper)((*directory)(nil))  // Contains Lookup
var _ = (fs.NodeReaddirer)((*directory)(nil)) // Contains Readdir
var _ = (fs.NodeOpener)((*directory)(nil))    // Contains Open
var _ = (fs.NodeOnAdder)((*directory)(nil))   // Contains OnAdd

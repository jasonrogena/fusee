package mount

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/jasonrogena/fusee/internal/app/fusee/config"
	"github.com/jasonrogena/fusee/internal/pkg/command"
	fuseefs "github.com/jasonrogena/fusee/internal/pkg/fs"
	log "github.com/sirupsen/logrus"
)

type parent interface {
	getCommandContext() command.Context
	getInode() *fs.Inode
	getReadCommand() (string, error)
	getNameSeparator() (string, error)
	getDirectoryConfig() config.Directory
	getFileConfig() config.File
	isContentStale() bool
	getAttr() *fuse.Attr
}

func loadChildren(ctx context.Context, r parent) error {
	log.Debug("loadChildren called on ", r.getCommandContext().Name)
	log.Debug("Number of children before loading children is %d", len(r.getInode().Children()))
	defer log.Debug("Number of children after loading children is %d", len(r.getInode().Children()))
	if !r.isContentStale() {
		log.Debug("Content is not yet stale, not running command")
		return nil
	}

	log.Info("Running command to get dirents for ",
		r.getCommandContext().MountRootDirPath+string(os.PathSeparator)+r.getCommandContext().RelativePath)
	r.getAttr().Mtime = uint64(time.Now().Unix())
	readCommand, readCommandErr := r.getReadCommand()
	if readCommandErr != nil {
		return readCommandErr
	}
	commandOutput, commandErr := command.Run(readCommand, r.getCommandContext())
	if commandErr != nil {
		return commandErr
	}
	log.Debug("Command output is ", string(commandOutput[:]))
	separator, separatorErr := r.getNameSeparator()
	if separatorErr != nil {
		return separatorErr
	}
	names := strings.Split(string(commandOutput[:]), separator)
	commandContext := r.getCommandContext()
	for _, curFileName := range names {
		curFileName = strings.TrimSpace(curFileName)
		if len(curFileName) == 0 {
			log.Error("Could not add file with an empty name")
			continue
		}
		log.Debug("Adding file '%s'", curFileName)
		commandContext.Name = curFileName
		relativePath := r.getCommandContext().RelativePath
		if len(relativePath) > 0 {
			relativePath = relativePath + string(os.PathSeparator)
		}
		commandContext.RelativePath = relativePath + curFileName
		dirConfig := r.getDirectoryConfig()
		if len(dirConfig.ReadCommand) > 0 {
			// Try test the dir command
			_, curFileErr := command.Run(dirConfig.ReadCommand, commandContext)
			if curFileErr == nil {
				if addDirectoryChild(ctx, r, commandContext) {
					log.Debug("Successfully added directory '%s'", curFileName)
				}
				continue
			}
			log.Debug("There was an error attemting to run directory command against '%s', adding it as a file instead %v", curFileErr, curFileErr)
		}

		// If we have gotten to this point, assume child is a file
		if addFileChild(ctx, r, commandContext) {
			log.Debug("Successfully added file '%s'", curFileName)
		}
	}

	return nil
}

func addDirectoryChild(ctx context.Context, r parent, commandContext command.Context) bool {
	ch := r.getInode().NewInode(
		ctx,
		NewDirectory(
			r.getDirectoryConfig(),
			r.getFileConfig(),
			commandContext,
		),
		fuseefs.GetDirectoryStableAttr(commandContext))
	return r.getInode().AddChild(commandContext.Name, ch, true)
}

func addFileChild(ctx context.Context, r parent, commandContext command.Context) bool {
	ch := r.getInode().NewInode(
		ctx,
		NewFile(r.getFileConfig(), commandContext),
		fuseefs.GetFileStableAttr(commandContext))
	return r.getInode().AddChild(commandContext.Name, ch, true)
}

type cache interface {
	getAttr() *fuse.Attr
	getCacheSeconds() uint64
	shouldCache() bool
}

func isContentStale(f cache) bool {
	if f.shouldCache() && f.getAttr().Mtime != f.getAttr().Ctime {
		timeDiff := uint64(time.Now().Unix()) - f.getAttr().Mtime
		log.Debug("Time difference between last mtime and now is ", timeDiff)
		return timeDiff > f.getCacheSeconds()
	}

	return true
}

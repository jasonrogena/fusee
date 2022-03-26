package mount

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/jasonrogena/fusee/internal/app/fusee/config"
	"github.com/jasonrogena/fusee/internal/pkg/command"
	fuseefs "github.com/jasonrogena/fusee/internal/pkg/fs"
	log "github.com/sirupsen/logrus"
)

type parent interface {
	getCommandState() *command.State
	getInode() *fs.Inode
	getReadCommand() (string, error)
	getNameSeparator() (string, error)
	getDirectoryConfig() config.Directory
	getFileConfig() config.File
	isContentStale() bool
	getAttr() *fuse.Attr
	getCommandRunnerPool() *command.Pool
	getCachedTestRunOutput() []byte
	clearCachedTestRunOutput()
}

func loadChildren(ctx context.Context, r parent, wg *sync.WaitGroup) error {
	log.Debug(fmt.Sprintf("loadChildren called on '%s'", r.getCommandState().RelativePath))
	log.Debug(fmt.Sprintf("Number of children before loading children is %d", len(r.getInode().Children())))
	defer log.Debug(fmt.Sprintf("Number of children after loading children is %d", len(r.getInode().Children())))
	if !r.isContentStale() {
		log.Debug("Content is not yet stale, not running command")
		if len(r.getCachedTestRunOutput()) > 0 {
			log.Debug(fmt.Sprintf("Using the output for the command used to test whether '%s' is a directory to build its dirents", r.getCommandState().RelativePath))
			loadCommandOutput(ctx, r, r.getCachedTestRunOutput())
			r.clearCachedTestRunOutput()
		}
		return nil
	}

	log.Info("Running command to get dirents for ",
		r.getCommandState().MountRootDirPath+string(os.PathSeparator)+r.getCommandState().RelativePath)
	r.getAttr().Mtime = uint64(time.Now().Unix())
	readCommand, readCommandErr := r.getReadCommand()
	if readCommandErr != nil {
		return readCommandErr
	}
	wg.Add(1)
	r.getCommandRunnerPool().AddCommand(command.NewCommand(readCommand, r.getCommandState(), func(commandOutput []byte, commandErr error) {
		if commandErr != nil {
			log.Warn(fmt.Sprintf("Unable to load direntries for '%s' due to an error: %v", r.getCommandState().RelativePath, commandErr))
			return
		}
		loadCommandOutput(ctx, r, commandOutput)
		wg.Done()
	}))
	return nil
}

func loadCommandOutput(ctx context.Context, r parent, commandOutput []byte) {
	separator, separatorErr := r.getNameSeparator()
	if separatorErr != nil {
		log.Warn(fmt.Sprintf("Unable to load direntries for '%s' due to an error: %v", r.getCommandState().RelativePath, separatorErr))
		return
	}
	names := strings.Split(string(commandOutput[:]), separator)
	for _, curFileName := range names {
		commandState := command.CopyState(r.getCommandState())
		curFileName = strings.TrimSpace(curFileName)
		if len(curFileName) == 0 {
			log.Error("Could not add file with an empty name")
			continue
		}
		log.Debug(fmt.Sprintf("Adding dirent '%s'", curFileName))
		commandState.Name = curFileName
		relativePath := r.getCommandState().RelativePath
		if len(relativePath) > 0 {
			relativePath = relativePath + string(os.PathSeparator)
		}
		commandState.RelativePath = relativePath + curFileName
		dirConfig := r.getDirectoryConfig()
		if len(dirConfig.ReadCommand) > 0 {
			// Try test the dir command
			command.NewCommand(dirConfig.ReadCommand, commandState, func(testOutput []byte, testOutputErr error) {
				if testOutputErr == nil {
					addDirectoryChild(ctx, r, commandState, testOutput, r.getCommandRunnerPool())
				} else {
					log.Debug(fmt.Sprintf("There was an error attemting to run directory command against '%s', adding it as a file instead %v", commandState.RelativePath, testOutputErr))
					addFileChild(ctx, r, commandState, r.getCommandRunnerPool())
				}
			}).Run()
		} else { // Just treat as if dirent is a file
			addFileChild(ctx, r, commandState, r.getCommandRunnerPool())
		}
	}
}

func addDirectoryChild(ctx context.Context, r parent, commandState *command.State, commandOutput []byte, commandRunnerPool *command.Pool) bool {
	ch := r.getInode().NewInode(
		ctx,
		NewDirectory(
			r.getDirectoryConfig(),
			r.getFileConfig(),
			commandOutput,
			commandState,
			commandRunnerPool,
		),
		fuseefs.GetDirectoryStableAttr(commandState))
	success := r.getInode().AddChild(commandState.Name, ch, true)
	if success {
		log.Debug("Successfully added directory '%s'", commandState.RelativePath)
	} else {
		log.Warn("Could not add directory '%s'", commandState.RelativePath)
	}
	return success
}

func addFileChild(ctx context.Context, r parent, commandState *command.State, commandRunnerPool *command.Pool) bool {
	ch := r.getInode().NewInode(
		ctx,
		NewFile(r.getFileConfig(), commandState, commandRunnerPool),
		fuseefs.GetFileStableAttr(commandState))
	success := r.getInode().AddChild(commandState.Name, ch, true)
	if success {
		log.Debug("Successfully added file '%s'", commandState.RelativePath)
	} else {
		log.Warn("Could not add file '%s'", commandState.RelativePath)
	}
	return success
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

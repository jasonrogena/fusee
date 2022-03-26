package command

import (
	"bytes"
	"os/exec"
	"text/template"

	log "github.com/sirupsen/logrus"
)

type Command struct {
	state       *State
	template    string
	postRunHook func([]byte, error)
}

type State struct {
	MountName        string
	MountRootDirPath string
	RelativePath     string
	Name             string
}

func NewState(mountName string, mountRootDirPath string, relativePath string, fileName string) *State {
	return &State{
		MountName:        mountName,
		MountRootDirPath: mountRootDirPath,
		RelativePath:     relativePath,
		Name:             fileName,
	}
}

func CopyState(original *State) *State {
	return &State{
		MountName:        original.MountName,
		MountRootDirPath: original.MountRootDirPath,
		RelativePath:     original.RelativePath,
		Name:             original.Name,
	}
}

func NewCommand(template string, state *State, postRunHook func([]byte, error)) *Command {
	return &Command{
		state:       state,
		template:    template,
		postRunHook: postRunHook,
	}
}

func (c *Command) constructCommand() (string, error) {
	t, tErr := template.New("Command").Parse(c.template)
	if tErr != nil {
		return "", tErr
	}

	var CommandBuf bytes.Buffer
	execErr := t.Execute(&CommandBuf, *c.state)
	if execErr != nil {
		return "", execErr
	}

	return CommandBuf.String(), nil
}

func (c *Command) Run() {
	Command, CommandErr := c.constructCommand()
	if CommandErr != nil {
		if c.postRunHook != nil {
			c.postRunHook([]byte{}, CommandErr)
		}
		return
	}

	output, outputErr := exec.Command("sh", "-c", Command).Output()
	log.Debug("About to run postRunHook")
	if c.postRunHook != nil {
		c.postRunHook(output, outputErr)
	}
}

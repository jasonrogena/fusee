package command

import (
	"bytes"
	"os/exec"
	"text/template"
)

type Context struct {
	MountName        string
	MountRootDirPath string
	RelativePath     string
	Name             string
}

func constructCommand(commandTemplate string, context Context) (string, error) {
	t, tErr := template.New("command").Parse(commandTemplate)
	if tErr != nil {
		return "", tErr
	}

	var commandBuf bytes.Buffer
	execErr := t.Execute(&commandBuf, context)
	if execErr != nil {
		return "", execErr
	}

	return commandBuf.String(), nil
}

func Run(commandTemplate string, context Context) ([]byte, error) {
	command, commandErr := constructCommand(commandTemplate, context)
	if commandErr != nil {
		return []byte{}, commandErr
	}

	return exec.Command("sh", "-c", command).Output()
}

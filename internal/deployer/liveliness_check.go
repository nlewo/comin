package deployer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
	shellwords "github.com/mattn/go-shellwords"
	"github.com/nlewo/comin/internal/protobuf"
)

func runLivelinessCheckCommand(command string, deployment *protobuf.Deployment) (string, error) {
	t, err := template.New("liveliness-check-command").
		Funcs(sprig.TxtFuncMap()).
		Parse(command)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, deployment); err != nil {
		return "", err
	}

	parsedArgs, err := shellwords.Parse(tpl.String())
	if err != nil {
		return "", err
	}
	if len(parsedArgs) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.Command(parsedArgs[0], parsedArgs[1:]...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()

	return string(output), err
}

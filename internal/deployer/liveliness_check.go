package deployer

import (
	"bytes"
	"os"
	"os/exec"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
	"github.com/nlewo/comin/internal/protobuf"
)

func runLivelinessCheckCommand(command string, deployment *protobuf.Deployment) (string, error) {
	t, err := template.New("liveliness-check-command").Funcs(sprig.TxtFuncMap()).Parse(command)
	if err != nil {
		return "", err
	}
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, deployment); err != nil {
		return "", err
	}

	cmd := exec.Command(tpl.String())
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()

	return string(output), err
}

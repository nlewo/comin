package deployer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"
)

func envGitSha(d *store.Deployment) string {
	return d.Generation.SelectedCommitId
}

func envGitRef(d *store.Deployment) string {
	return fmt.Sprintf("%s/%s", d.Generation.SelectedRemoteName, d.Generation.SelectedBranchName)
}

func envGitMessage(d *store.Deployment) string {
	return strings.Trim(d.Generation.SelectedCommitMsg, "\n")
}

func envCominGeneration(d *store.Deployment) string {
	return d.Generation.UUID.String()
}

func envCominHostname(d *store.Deployment) string {
	return d.Generation.Hostname
}

func envCominStatus(d *store.Deployment) string {
	return store.StatusToString(d.Status)
}

func envCominErrorMessage(d *store.Deployment) string {
	return d.ErrorMsg
}

func envCominFlakeUrl(d *store.Deployment) string {
	return d.Generation.FlakeUrl
}

func runPostDeploymentCommand(command string, d *store.Deployment) (string, error) {

	cmd := exec.Command(command)

	cmd.Env = append(os.Environ(),
		"COMIN_GIT_SHA="+envGitSha(d),
		"COMIN_GIT_REF="+envGitRef(d),
		"COMIN_GIT_MSG="+envGitMessage(d),
		"COMIN_HOSTNAME="+envCominHostname(d),
		"COMIN_FLAKE_URL="+envCominFlakeUrl(d),
		"COMIN_GENERATION="+envCominGeneration(d),
		"COMIN_STATUS="+envCominStatus(d),
		"COMIN_ERROR_MSG="+envCominErrorMessage(d),
	)

	output, err := cmd.CombinedOutput()
	outputString := string(output)
	if err != nil {
		return outputString, err
	}

	logrus.Debugf("cmd:[%s] output:[%s]", command, outputString)

	return outputString, nil
}

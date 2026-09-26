package deployer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	pb "github.com/nlewo/comin/pkg/protobuf"

	"github.com/sirupsen/logrus"
)

func envGitSha(d *pb.Deployment) string {
	if d.Generation.Source != nil && d.Generation.Source.GetGit() != nil {
		return d.Generation.Source.GetGit().SelectedCommitId
	}
	return ""
}

func envGitRef(d *pb.Deployment) string {
	if d.Generation.Source != nil && d.Generation.Source.GetGit() != nil {
		git := d.Generation.Source.GetGit()
		return fmt.Sprintf("%s/%s", git.SelectedRemoteName, git.SelectedBranchName)
	}
	return ""
}

func envGitMessage(d *pb.Deployment) string {
	if d.Generation.Source != nil && d.Generation.Source.GetGit() != nil {
		return strings.Trim(d.Generation.Source.GetGit().SelectedCommitMsg, "\n")
	}
	return ""
}

func envCominGeneration(d *pb.Deployment) string {
	return d.Generation.Uuid
}

func envCominHostname(d *pb.Deployment) string {
	if d.Generation.Source != nil && d.Generation.Source.GetGit() != nil {
		return d.Generation.Source.GetGit().Hostname
	}
	return ""
}

func envCominStatus(d *pb.Deployment) string {
	return d.Status
}

func envCominErrorMessage(d *pb.Deployment) string {
	return d.ErrorMsg
}

func runPostDeploymentCommand(command string, d *pb.Deployment) (string, error) {

	cmd := exec.Command(command)

	cmd.Env = append(os.Environ(),
		"COMIN_PHASE=deploy",
		"COMIN_GIT_SHA="+envGitSha(d),
		"COMIN_GIT_REF="+envGitRef(d),
		"COMIN_GIT_MSG="+envGitMessage(d),
		"COMIN_HOSTNAME="+envCominHostname(d),
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

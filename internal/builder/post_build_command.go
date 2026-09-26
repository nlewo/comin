package builder

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	pb "github.com/nlewo/comin/pkg/protobuf"

	"github.com/sirupsen/logrus"
)

func envGitSha(g *pb.Generation) string {
	if g.Source != nil && g.Source.GetGit() != nil {
		return g.Source.GetGit().SelectedCommitId
	}
	return ""
}

func envGitRef(g *pb.Generation) string {
	if g.Source != nil && g.Source.GetGit() != nil {
		git := g.Source.GetGit()
		return fmt.Sprintf("%s/%s", git.SelectedRemoteName, git.SelectedBranchName)
	}
	return ""
}

func envGitMessage(g *pb.Generation) string {
	if g.Source != nil && g.Source.GetGit() != nil {
		return strings.Trim(g.Source.GetGit().SelectedCommitMsg, "\n")
	}
	return ""
}

func envCominHostname(g *pb.Generation) string {
	if g.Source != nil && g.Source.GetGit() != nil {
		return g.Source.GetGit().Hostname
	}
	return ""
}

func envCominGeneration(g *pb.Generation) string {
	return g.Uuid
}

func runPostBuildCommand(command string, g *pb.Generation, status string, errMsg string) (string, error) {
	cmd := exec.Command(command)

	cmd.Env = append(os.Environ(),
		"COMIN_PHASE=build",
		"COMIN_GIT_SHA="+envGitSha(g),
		"COMIN_GIT_REF="+envGitRef(g),
		"COMIN_GIT_MSG="+envGitMessage(g),
		"COMIN_HOSTNAME="+envCominHostname(g),
		"COMIN_GENERATION="+envCominGeneration(g),
		"COMIN_STATUS="+status,
		"COMIN_ERROR_MSG="+errMsg,
	)

	output, err := cmd.CombinedOutput()
	outputString := string(output)
	if err != nil {
		return outputString, err
	}

	logrus.Debugf("cmd:[%s] output:[%s]", command, outputString)

	return outputString, nil
}

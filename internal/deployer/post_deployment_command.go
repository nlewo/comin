package deployer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func envGitSha(d *Deployment) string {
	return d.Generation.SelectedCommitId
}

func envGitRef(d *Deployment) string {
	return fmt.Sprintf("%s/%s", d.Generation.SelectedRemoteName, d.Generation.SelectedBranchName)
}

func envGitMessage(d *Deployment) string {
	return strings.Trim(d.Generation.SelectedCommitMsg, "\n")
}

func envCominGeneration(d *Deployment) string {
	return d.Generation.UUID
}

func envCominHostname(d *Deployment) string {
	return d.Generation.Hostname
}

func envCominState(d *Deployment) string {
	return StatusToString(d.Status)
}

func envCominErrorMessage(d *Deployment) string {
	return d.ErrorMsg
}

// func envCominRestart(d *Deployment) string {
// 	if d.RestartComin {
// 		return "true"
// 	}
// 	return "false"
// }

func envCominFlakeUrl(d *Deployment) string {
	return d.Generation.FlakeUrl
}

func RunPostDeploymentCommand(command string, d *Deployment) error {

	cmd := exec.Command(command)

	cmd.Env = append(os.Environ(),
		"GIT_SHA="+envGitSha(d),
		"GIT_REF="+envGitRef(d),
		"GIT_MSG="+envGitMessage(d),
		"COMIN_HOSTNAME="+envCominHostname(d),
		"COMIN_FLAKE_URL="+envCominFlakeUrl(d),
		"COMIN_GENERATION="+envCominGeneration(d),
		"COMIN_STATE="+envCominState(d),
		// "COMIN_RESTART="+envCominRestart(d),
		"COMIN_ERROR_MSG="+envCominErrorMessage(d),
		// d.Generation.EvalErrStr,
		// d.Generation.BuildErrStr
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return err
	}

	fmt.Println(string(output))

	return nil
}

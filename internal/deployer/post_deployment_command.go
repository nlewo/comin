package deployer

import (
	"fmt"
	"os"
	"os/exec"
)

func envSha() string {
	return "SHA"
}

func envRef() string {
	return "REF"
}

func envState() string {
	return "STATE"
}

func RunPostDeploymentCommand(command string, d *Deployment) error {

	cmd := exec.Command(command)

	cmd.Env = append(os.Environ(),
		"GIT_SHA="+envSha(),
		"GIT_REF="+envRef(),
		"COMIN_STATE="+envState(),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}

	fmt.Println(string(output))

	return nil
}

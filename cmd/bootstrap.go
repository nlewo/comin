package cmd

import (
	"fmt"
	"github.com/nlewo/comin/git"
	"github.com/nlewo/comin/nix"
	"github.com/nlewo/comin/state"
	"github.com/nlewo/comin/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"os"
	"path/filepath"
)

var baseDir string
var askForGitLabAccessToken bool
var operation string

// promptSecret prompts user for an input that is not echo-ed on terminal.
func promptSecret(question string) (string, error) {
	var (
		prompt string
		answer string
	)
	fmt.Printf(question)
	raw, err := term.MakeRaw(0)
	if err != nil {
		return "", err
	}
	defer term.Restore(0, raw)
	terminal := term.NewTerminal(os.Stdin, prompt)
	for {
		char, err := terminal.ReadPassword(prompt)
		if err != nil {
			return "", err
		}
		answer += char
		if char == "" || char == answer {
			return answer, nil
		}
	}
}

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap <repository> <commit-id>",
	Short: "Bootstrap the local machine with Comin",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		var accessToken string
		var err error
		repo := args[0]
		rev := args[1]
		stateFilepath := filepath.Join(baseDir, "state.json")

		fmt.Printf("You are bootstraping this NixOS machine with Comin!\n\n")
		fmt.Printf("We will automatically proceed to the following steps:\n\n")
		fmt.Printf("  1. Clone your NixOS configuration repository '%s' with rev '%s'\n",
			repo, rev)
		fmt.Printf("  2. Build the configuration for the machine '%s'\n", hostname)
		fmt.Printf("  2. Run the switch-to-configuration operation '%s'\n", operation)
		fmt.Printf("  3. Write the comin state.json file into '%s'\n", baseDir)
		fmt.Printf("\n")

		if askForGitLabAccessToken {
			accessToken, err = promptSecret("Please provide your GitLab Access Token: ")
			if err != nil {
				fmt.Printf("Failed to get GitLab Access Token: '%s'", err)
				return
			}
		}

		fmt.Printf("\n")

		tmpDir, err := os.MkdirTemp("", "comin")
		if err != nil {
			logrus.Errorf("Failed to create the temporary directory: '%s'", err)
			return
		}
		defer os.RemoveAll(tmpDir)

		logrus.Infof("Cloning the repository '%s' with rev '%s'\n", repo, rev)
		err = git.RepositoryClone(tmpDir, repo, rev, accessToken)
		if err != nil {
			logrus.Errorf("Failed to clone the repository '%s' into '%s' with the error: '%s'", repo, tmpDir, err)
			return
		}

		// We write the state in order to provide a commit ID
		// for security reason: for future depoyment, this
		// commit needs to be an ancestor to garantee fast
		// forward pulls.
		var st state.State
		st.RepositoryStatus = git.RepositoryStatus{MainCommitId: rev}
		// We write the state before deploying the
		// configuration because we can kill the comin process
		// during bootstrap (when the bootstrap kill network
		// connections while comin bootstrap is executed via SSH )
		if err := state.Save(stateFilepath, st); err != nil {
			logrus.Errorf("Failed to save the state to '%s': '%s'", stateFilepath, err)
			return
		}

		logrus.Infof("Starting to deploy the configuration for machine %s\n", hostname)
		cominNeedRestart, err := nix.Deploy(
			hostname,
			baseDir,
			tmpDir,
			operation,
			false,
		)

		if err != nil {
			logrus.Error(err)
			logrus.Infof("Deployment failed")
			return
		} else {
			st.HeadCommitDeployed = err == nil
			if err := state.Save(stateFilepath, st); err != nil {
				return
			}
		}

		if cominNeedRestart {
			if err := utils.CominServiceRestart(); err != nil {
				return
			}
		}

	},
}

func init() {
	hostnameDefault, err := os.Hostname()
	if err != nil {
		fmt.Println(err)
	}
	bootstrapCmd.PersistentFlags().StringVarP(&operation, "operation", "o", "switch", "the switch-to-configuration operation")
	bootstrapCmd.PersistentFlags().StringVarP(
		&hostname, "hostname", "", hostnameDefault, "the name of the configuration to deploy")
	bootstrapCmd.PersistentFlags().StringVarP(&baseDir, "base-directory", "b", "/var/lib/comin", "the base comin directory")
	bootstrapCmd.PersistentFlags().BoolVarP(&askForGitLabAccessToken, "ask-for-gitlab-access-token", "a", false, "ask for a GitLab access token")
	rootCmd.AddCommand(bootstrapCmd)
}

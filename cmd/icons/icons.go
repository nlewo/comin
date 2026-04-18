package icons

import (
	_ "embed"

	"github.com/nlewo/comin/internal/client"
	"github.com/sirupsen/logrus"
)

type Icon []byte

//go:embed bare.png
var Bare Icon

//go:embed notify.png
var Notify Icon

func GetIcon(client client.Client) Icon {
	status, err := client.GetManagerState()
	if err != nil {
		logrus.Errorln("Failed to retrieve manager state.")
		return Notify
	}

	if status.BuildConfirmer.Submitted != "" || status.DeployConfirmer.Submitted != "" {
		return Notify
	}
	return Bare
}

package deployer

import (
	"encoding/json"
	"os"

	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
)

// GenlistEntry represents a generation entry in the genlist
type GenlistEntry struct {
	// Version is the generation index
	Version int
	// BuildTime is the generation build time as a Unix timestamp
	BuildTime int
	// OutPath is the generation Nix outpath
	OutPath string
	// Label is an arbitrary label added to the boot entry
	Label string
}

// GenlistFromDeploymentHistory converts a list of deployments to a list of GenlistEntry
func GenlistFromDeploymentHistory(deployments []*protobuf.Deployment, path string) error {
	entries := make([]GenlistEntry, 0, len(deployments))
	for i, d := range deployments {
		if d.Operation != "switch" && d.Operation != "boot" || d.Gc {
			continue
		}
		genlistEntry := GenlistEntry{
			Version:   i,
			OutPath:   d.Generation.OutPath,
			BuildTime: d.CreatedAt.AsTime().Second(),
			Label:     d.Uuid,
		}
		entries = append(entries, genlistEntry)
	}
	return writeGenlistJSON(path, entries)
}

// writeGenlistJSON writes a genlist.json file where each line contains a GenlistEntry as JSON
func writeGenlistJSON(path string, entries []GenlistEntry) error {
	logrus.Infof("Writing genlist to %s", path)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}

	return nil
}

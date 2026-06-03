package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nlewo/comin/internal/types"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
)

// Hydra evaluates and fetches the build result of a flake-based
// configuration from a Hydra CI instance, then pulls it from the
// Hydra binary cache (which the user must add to nix.settings.substituters).
type Hydra struct {
	baseUrl       url.URL
	project       string
	jobsetPrefix  string
	jobName       string
	retryInterval time.Duration
	maxEvalPages  int

	// systemAttr is "nixosConfigurations" or "darwinConfigurations" and
	// drives platform-specific dispatch (deploy, reboot, machine-id).
	systemAttr string

	// drv2Out maps drvPath -> outPath. The Executor interface only passes
	// drvPath to Build, but fetchBuild needs outPath, so Eval stashes
	// the mapping it just resolved.
	mu      sync.Mutex
	drv2Out map[string]string
}

func NewHydraExecutor(config types.HydraConfig, systemAttr string) (h *Hydra, err error) {
	baseUrl, err := url.Parse(config.BaseUrl)
	if err != nil {
		return nil, err
	}

	h = &Hydra{
		baseUrl:       *baseUrl,
		project:       config.Project,
		jobsetPrefix:  config.JobsetPrefix,
		jobName:       config.JobName,
		retryInterval: time.Duration(config.RetryInterval) * time.Second,
		maxEvalPages:  config.MaxEvalPages,
		systemAttr:    systemAttr,
		drv2Out:       make(map[string]string),
	}
	return
}

type hydraEval struct {
	Id     int    `json:"id"`
	Flake  string `json:"flake"`
	Builds []int  `json:"builds"`
}

type hydraEvalsPage struct {
	First string      `json:"first"`
	Next  string      `json:"next"`
	Evals []hydraEval `json:"evals"`
}

type hydraBuildOutput struct {
	Path string `json:"path"`
}

type hydraBuild struct {
	Id           int                         `json:"id"`
	Job          string                      `json:"job"`
	System       string                      `json:"system"`
	Finished     int                         `json:"finished"`
	BuildStatus  *int                        `json:"buildstatus"`
	DrvPath      string                      `json:"drvpath"`
	BuildOutputs map[string]hydraBuildOutput `json:"buildoutputs"`
}

// extractRevFromFlakeUrl extracts the rev from a flake URL like
// "github:owner/repo/<rev>?narHash=..." → "<rev>".
func extractRevFromFlakeUrl(flake string) string {
	if i := strings.Index(flake, "?"); i >= 0 {
		flake = flake[:i]
	}
	if i := strings.LastIndex(flake, "/"); i >= 0 {
		return flake[i+1:]
	}
	return flake
}

func (h *Hydra) getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			logrus.Warnf("hydra: failed to close response body: %v", cerr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hydra: GET %s returned status %d", u, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Eval polls the Hydra API for a build matching the given commit and
// jobName, blocking until the build has finished successfully. The jobset
// scanned is derived from the branch being deployed: jobset_prefix is
// prepended to selectedBranchName (so branch "main" with prefix "nixos-"
// scans jobset "nixos-main"). The returned machineId is always empty:
// deriving the expected machine-id would require a local flake evaluation,
// which defeats the purpose of the Hydra executor.
func (h *Hydra) Eval(ctx context.Context, repositoryPath, repositorySubdir, commitId, selectedBranchName, systemAttr, hostname string, submodules bool) (drvPath string, outPath string, machineId string, err error) {
	machineId = ""
	if commitId == "" {
		err = errors.New("hydra: commitId is required")
		return
	}
	if selectedBranchName == "" {
		err = errors.New("hydra: selectedBranchName is required to derive the jobset name")
		return
	}
	jobset := h.jobsetPrefix + selectedBranchName

	for {
		matchedButNotFinished := false

		for page := 1; page <= h.maxEvalPages; page++ {
			pageUrl := h.baseUrl.JoinPath("/jobset/", h.project, jobset, "evals")
			q := pageUrl.Query()
			q.Set("page", fmt.Sprintf("%d", page))
			pageUrl.RawQuery = q.Encode()
			logrus.Infof("hydra: fetching evaluations from %s", pageUrl)

			var evals hydraEvalsPage
			if err = h.getJSON(ctx, pageUrl.String(), &evals); err != nil {
				return
			}

			for _, ev := range evals.Evals {
				rev := extractRevFromFlakeUrl(ev.Flake)
				if rev == "" || !strings.HasPrefix(rev, commitId) {
					continue
				}
				for _, buildId := range ev.Builds {
					buildUrl := h.baseUrl.JoinPath("/build/", fmt.Sprintf("%d", buildId))
					var b hydraBuild
					if err = h.getJSON(ctx, buildUrl.String(), &b); err != nil {
						return
					}
					if b.Job != h.jobName {
						continue
					}
					if b.Finished == 0 {
						logrus.Infof("hydra: matched build %d for %s/%s but not finished yet, retrying...", b.Id, commitId, h.jobName)
						matchedButNotFinished = true
						break
					}
					if b.BuildStatus == nil || *b.BuildStatus != 0 {
						err = fmt.Errorf("hydra: build %d for %s/%s failed (buildstatus=%v)", b.Id, commitId, h.jobName, b.BuildStatus)
						return
					}
					out, ok := b.BuildOutputs["out"]
					if !ok || out.Path == "" {
						err = fmt.Errorf("hydra: build %d for %s/%s has no 'out' output path", b.Id, commitId, h.jobName)
						return
					}
					drvPath = b.DrvPath
					outPath = out.Path
					logrus.Infof("hydra: matched build %d for %s/%s drv=%s out=%s", b.Id, commitId, h.jobName, drvPath, outPath)
					h.mu.Lock()
					h.drv2Out[drvPath] = outPath
					h.mu.Unlock()
					return
				}
				if matchedButNotFinished {
					break
				}
			}
			if matchedButNotFinished {
				break
			}
			if evals.Next == "" {
				break
			}
		}

		select {
		case <-time.After(h.retryInterval):
		case <-ctx.Done():
			err = ctx.Err()
			return
		}
	}
}

func (h *Hydra) Build(ctx context.Context, drvPath string) (err error) {
	logrus.Infof("hydra: fetching build for %s", drvPath)
	h.mu.Lock()
	outPath, ok := h.drv2Out[drvPath]
	h.mu.Unlock()
	if !ok {
		return errors.New("hydra: build called before eval")
	}
	return fetchBuild(ctx, outPath)
}

func (h *Hydra) Deploy(ctx context.Context, outPath, operation string, profilePaths []string) (needToRestartComin bool, profilePath string, err error) {
	return deploy(ctx, outPath, operation, h.systemAttr, profilePaths)
}

func (h *Hydra) IsStorePathExist(storePath string) bool {
	return isStorePathExist(storePath)
}

func (h *Hydra) NeedToReboot(outPath, operation string) bool {
	if h.systemAttr == "darwinConfigurations" {
		// See NixFlakeLocal.NeedToReboot: Darwin lacks the
		// /run/current-system vs /run/booted-system mechanism, so
		// conservatively assume no reboot is needed.
		return false
	}
	return utils.NeedToRebootLinux(outPath, operation)
}

func (h *Hydra) ReadMachineId() (machineId string, err error) {
	if h.systemAttr == "darwinConfigurations" {
		return utils.ReadMachineIdDarwin()
	}
	return utils.ReadMachineIdLinux()
}

package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/groupcache/lru"
	"github.com/nlewo/comin/internal/types"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
)

// Garnix evaluates and fetches the build result of a flake-based
// configuration from garnix.io with a minimal local memory footprint:
// it relies on garnix.io's CI build artifacts and pulls them from the
// substituter (e.g. cache.garnix.io) at deploy time.
type Garnix struct {
	baseUrl       url.URL
	cacheUrl      url.URL
	retryInterval time.Duration

	// systemAttr is "nixosConfigurations" or "darwinConfigurations" and
	// determines the expected packageType from the garnix API and how
	// platform-specific helpers (reboot detection, machine-id) dispatch.
	systemAttr string

	drv2Out lru.Cache
}

func NewGarnixExecutor(config types.GarnixConfig, systemAttr string) (g *Garnix, err error) {
	var baseUrl *url.URL
	var cacheUrl *url.URL
	if config.BaseUrl == "" {
		baseUrl, _ = url.Parse("https://garnix.io/")
	} else {
		baseUrl, err = url.Parse(config.BaseUrl)
		if err != nil {
			return nil, err
		}
	}

	if config.CacheUrl == "" {
		cacheUrl, _ = url.Parse("https://cache.garnix.io/")
	} else {
		cacheUrl, err = url.Parse(config.CacheUrl)
		if err != nil {
			return nil, err
		}
	}

	if config.CacheSize == 0 {
		config.CacheSize = 2
	}

	if config.RetryInterval == 0 {
		config.RetryInterval = 60
	}

	g = &Garnix{
		baseUrl:       *baseUrl,
		cacheUrl:      *cacheUrl,
		retryInterval: time.Duration(config.RetryInterval) * time.Second,
		systemAttr:    systemAttr,
		drv2Out:       *lru.New(config.CacheSize),
	}
	return
}

type GarnixOutPath struct {
	Out string `json:"out"`
}

type GarnixBuild struct {
	Id              string        `json:"id"`
	DrvPath         string        `json:"drv_path"`
	OutPath         GarnixOutPath `json:"output_paths"`
	PackageType     string        `json:"package_type"`
	Package         string        `json:"package"`
	UploadedToCache bool          `json:"uploaded_to_cache"`
	EndTime         string        `json:"end_time"`
	Status          string        `json:"status"`
}

type GarnixCommit struct {
	GarnixBuilds []GarnixBuild `json:"builds"`
}

func (g *Garnix) expectedPackageType() string {
	if g.systemAttr == "darwinConfigurations" {
		return "darwinConfiguration"
	}
	return "nixosConfiguration"
}

// Eval polls the garnix API for a build matching the given commit and
// hostname, blocking until the build artifact has been uploaded to the
// cache. The returned machineId is always empty: deriving the expected
// machine-id would require a local flake evaluation, which defeats the
// purpose of the Garnix executor.
func (g *Garnix) Eval(ctx context.Context, repositoryPath, repositorySubdir, commitId, systemAttr, hostname string, submodules bool) (drvPath string, outPath string, machineId string, err error) {
	machineId = ""
	if commitId == "" {
		err = errors.New("garnix: commitId is required")
		return
	}

	expectedPackageType := g.expectedPackageType()

	for {
		commitUrl := g.baseUrl.JoinPath("/api/commits/", commitId)
		logrus.Infof("garnix: fetching commit result from %s", commitUrl)

		var resp *http.Response
		resp, err = http.Get(commitUrl.String())
		if err != nil {
			return
		}

		var commitInfo GarnixCommit
		decodeErr := json.NewDecoder(resp.Body).Decode(&commitInfo)
		if cerr := resp.Body.Close(); cerr != nil {
			logrus.Warnf("garnix: failed to close response body: %v", cerr)
		}
		if decodeErr != nil {
			err = decodeErr
			return
		}

		for _, build := range commitInfo.GarnixBuilds {
			if build.PackageType != expectedPackageType || build.Package != hostname {
				continue
			}
			if build.EndTime != "" && build.Status != "Success" {
				err = fmt.Errorf("garnix: build for %s/%s failed (status=%s)", commitId, hostname, build.Status)
				return
			}
			if !build.UploadedToCache {
				logrus.Infof("garnix: build for %s/%s not uploaded to cache yet, retrying...", commitId, hostname)
				break
			}
			drvPath = build.DrvPath
			outPath = build.OutPath.Out
			g.drv2Out.Add(drvPath, outPath)
			return
		}

		select {
		case <-time.After(g.retryInterval):
		case <-ctx.Done():
			err = ctx.Err()
			return
		}
	}
}

func (g *Garnix) Build(ctx context.Context, drvPath string) (err error) {
	logrus.Infof("garnix: fetching build for %s", drvPath)
	value, ok := g.drv2Out.Get(drvPath)
	if !ok {
		return errors.New("garnix: build called before eval")
	}
	outPath, ok := value.(string)
	if !ok {
		return errors.New("garnix: drv2Out cache contained a non-string value")
	}
	return fetchBuild(ctx, outPath)
}

func (g *Garnix) Deploy(ctx context.Context, outPath, operation string, profilePaths []string) (needToRestartComin bool, profilePath string, err error) {
	return deploy(ctx, outPath, operation, g.systemAttr, profilePaths)
}

func (g *Garnix) IsStorePathExist(storePath string) bool {
	return isStorePathExist(storePath)
}

func (g *Garnix) NeedToReboot(outPath, operation string) bool {
	if g.systemAttr == "darwinConfigurations" {
		// See NixFlakeLocal.NeedToReboot: Darwin lacks the
		// /run/current-system vs /run/booted-system mechanism, so
		// conservatively assume no reboot is needed.
		return false
	}
	return utils.NeedToRebootLinux(outPath, operation)
}

func (g *Garnix) ReadMachineId() (machineId string, err error) {
	if g.systemAttr == "darwinConfigurations" {
		return utils.ReadMachineIdDarwin()
	}
	return utils.ReadMachineIdLinux()
}

package executor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/golang/groupcache/lru"
	"github.com/nlewo/comin/internal/types"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
)

// TODO: Private repo
type Garnix struct {
	baseUrl       url.URL
	cacheUrl      url.URL
	retryInterval time.Duration

	configurationAttr string

	drv2Out lru.Cache
}

func NewGarnixExecutor(config types.GarnixConfig, configurationAttr string) (g *Garnix, err error) {
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
		*baseUrl,
		*cacheUrl,
		time.Duration(config.RetryInterval) * time.Second,
		configurationAttr,
		*lru.New(config.CacheSize),
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

func (g *Garnix) Eval(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, machineId string, err error) {
	//TODO: Find a way to get machineId without evalutation
	machineId = ""

	values, err := url.ParseQuery(flakeUrl)
	if err != nil {
		return
	}
	rev := values.Get("rev")
	if rev == "" {
		logrus.Errorf("Failed to parse flakeUrl: %s. It has to contain rev.", flakeUrl)
		return
	}

	var commitInfo GarnixCommit
	var resp *http.Response
	uploadedToCache := false
	for !uploadedToCache {
		commitUrl := g.baseUrl.JoinPath("/api/commits/", rev)
		logrus.Infof("garnix: fetching commit result from %s", commitUrl)
		resp, err = http.Get(commitUrl.String())
		if err != nil {
			return
		}

		defer func() {
			if err := resp.Body.Close(); err != nil {
				logrus.Warnf("Failed to close response body: %v", err)
			}
		}()

		err = json.NewDecoder(resp.Body).Decode(&commitInfo)
		if err != nil {
			return
		}

		for _, build := range commitInfo.GarnixBuilds {
			if build.PackageType == "nixosConfiguration" && build.Package == hostname {
				if build.EndTime != "" && build.Status != "Success" {
					logrus.Errorf("garnix: %s build failed", rev)
				}
				uploadedToCache = build.UploadedToCache
				if !uploadedToCache {
					logrus.Infof("garnix: not uploaded to cache yet, retrying...")
					break
				}

				drvPath = build.DrvPath
				outPath = build.OutPath.Out
				if err != nil {
					return
				}
				g.drv2Out.Add(drvPath, outPath)
				return
			}
		}
		time.Sleep(g.retryInterval)
	}

	logrus.Errorf("garnix: %s not found in build", hostname)
	return
}

func (g *Garnix) Build(ctx context.Context, drvPath string) (err error) {
	logrus.Infof("garnix: fetching build for %s", drvPath)
	value, ok := g.drv2Out.Get(drvPath)
	if !ok {
		return errors.New("garnix: build called before eval")
	}
	outPath, ok := value.(string)
	if !ok {
		panic("garnix: none string in outpath map")
	}

	return fetchBuild(ctx, outPath)
}

func (g *Garnix) Deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	return deploy(ctx, outPath, operation, g.configurationAttr)
}

func (g *Garnix) IsStorePathExist(storePath string) bool {
	if _, err := os.Stat(storePath); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func (g *Garnix) NeedToReboot() bool {
	return utils.NeedToReboot(g.configurationAttr)
}

func (g *Garnix) ReadMachineId() (machineId string, err error) {
	return utils.ReadMachineId(g.configurationAttr)
}

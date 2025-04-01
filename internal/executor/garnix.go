package executor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/groupcache/lru"
	"github.com/nix-community/go-nix/pkg/derivation"
	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/nix-community/go-nix/pkg/narinfo"
	"github.com/nix-community/go-nix/pkg/nixbase32"
	"github.com/nix-community/go-nix/pkg/storepath"
	"github.com/nlewo/comin/internal/types"
	"github.com/sirupsen/logrus"
)

// TODO: Private repo
type Garnix struct {
	baseUrl       url.URL
	cacheUrl      url.URL
	retryInterval time.Duration

	drv2Out lru.Cache
}

func NewGarnixExecutor(config types.GarnixConfig) (g *Garnix, err error) {
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
		*lru.New(config.CacheSize),
	}
	return
}

type GarnixBuild struct {
	Id              string `json:"id"`
	DrvPath         string `json:"drv_path"`
	PackageType     string `json:"package_type"`
	Package         string `json:"package"`
	UploadedToCache bool   `json:"uploaded_to_cache"`
	EndTime         string `json:"end_time"`
	Status          string `json:"status"`
}

type GarnixCommit struct {
	GarnixBuilds []GarnixBuild `json:"builds"`
}

func getNarInfo(url string) (narInfo *narinfo.NarInfo, err error) {
	narinfoUrl := url
	resp, err := http.Get(narinfoUrl)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	narInfo, err = narinfo.Parse(resp.Body)
	return
}

func getDrv(narUrl string) (drv *derivation.Derivation, err error) {
	resp, err := http.Get(narUrl)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	narReader, err := nar.NewReader(resp.Body)
	if err != nil {
		return
	}
	for {
		var hdr *nar.Header
		hdr, err = narReader.Next()
		if err != nil {
			return
		}

		if hdr.Path == "/" {
			drv, err = derivation.ReadDerivation(narReader)
			if err != nil {
				logrus.Error("garnix: failed to read derivation")
				return
			}
			return
		}
	}
}

func outpathFromCache(cacheUrl url.URL, drvPath string) (outPath string, err error) {
	logrus.Infof("garnix: the derivation path is %s", drvPath)

	storePath, err := storepath.FromAbsolutePath(drvPath)
	if err != nil {
		return
	}
	hash := nixbase32.EncodeToString(storePath.Digest)
	logrus.Debugf("garnix: hash in derivation path %s", hash)

	url := cacheUrl.JoinPath(hash + ".narinfo")
	logrus.Infof("garnix: fetching narinfo from %s", url)
	narInfo, err := getNarInfo(url.String())
	if err != nil {
		logrus.Error("garnix: failed to parse narInfo")
		return
	}

	narUrl := cacheUrl.JoinPath(narInfo.URL)

	drv, err := getDrv(narUrl.String())
	if err != nil {
		logrus.Errorf("garnix: failed to parse nar %s as derivation", url)
		return
	}
	outPath = drv.Outputs["out"].Path

	logrus.Infof("garnix: the output path is %s", outPath)
	return
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
		defer resp.Body.Close()

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
				outPath, err = outpathFromCache(g.cacheUrl, drvPath)
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
	return deploy(ctx, outPath, operation)
}

package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

type flakery struct {
	repository *repository
}

func NewFlakery(repository *repository) (*flakery, error) {
	if repository == nil {
		return nil, errors.New("repository is nil")
	}
	return &flakery{
		repository: repository,
	}, nil
}

func (f *flakery) FetchAndUpdate(ctx context.Context, remoteNames []string) (rsCh chan RepositoryStatus) {
	rsc := f.repository.FetchAndUpdate(ctx, remoteNames)
	rsCh = make(chan RepositoryStatus)
	go func() {
		// make a timer that will check the deployment status every 5 seconds
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-ticker.C:
				deploymentStatus, err := f.getDeploymentStatus()
				fmt.Println("deploymentStatus", deploymentStatus)
				if err != nil {
					// FIXME: log error
					panic(err)
				}

				if deploymentStatus == "success" {
					rs := <-rsc
					rsCh <- rs
				}
			}
		}
	}()
	return rsCh
}

func (f *flakery) getDeploymentStatus() (string, error) {
	// todo wrap this repository with a flakery repository
	// that will pause when deployment status is building
	// deplyomentID = panic("not implemented")
	// read deployment id from env
	template := os.Getenv("TEMPLATE_ID")
	if template == "" {
		return "", errors.New("TEMPLATE_ID is not set")
	}

	userToken := os.Getenv("USER_TOKEN")

	if userToken == "" {
		return "", errors.New("USER_TOKEN is not set")
	}

	// Prepare HTTP request to flakery.dev
	url := "https://flakery.dev/api/v0/template/build-status/" + string(template)

	req, err := http.NewRequest("GET", url, bytes.NewBuffer([]byte{}))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+string(userToken))
	req.Header.Set("Content-Type", "application/json")

	// Send HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("GET request failed")
	} else {
		fmt.Println("GET request success")
	}

	// Read and return string response
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	status := buf.String()
	if status == "" {
		return "", errors.New("empty response")
	}
	return buf.String(), nil

}

func WrapRepositoryWithFlakery(r *repository) (*flakery, error) {
	return NewFlakery(r)
}

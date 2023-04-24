package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cncf/devstatscode"
	"github.com/cncf/landscape/pkg/types"
	yaml "gopkg.in/yaml.v2"
)

func checkSync() (err error) {
	landscapePath := os.Getenv("LANDSCAPE_YAML_PATH")
	if landscapePath == "" {
		landscapePath = "https://raw.githubusercontent.com/cncf/landscape/master/landscape.yml"
	}
	var dataL []byte
	if strings.Contains(landscapePath, "https://") || strings.Contains(landscapePath, "http://") {
		var response *http.Response
		response, err = http.Get(landscapePath)
		if err != nil {
			fmt.Printf("http.Get '%s' -> %+v", landscapePath, err)
			return
		}
		defer func() { _ = response.Body.Close() }()
		dataL, err = ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Printf("ioutil.ReadAll '%s' -> %+v", landscapePath, err)
			return
		}
	} else {
		dataL, err = ioutil.ReadFile(landscapePath)
		if err != nil {
			fmt.Printf("unable to read file '%s': %v", landscapePath, err)
			return
		}
	}
	projectsPath := os.Getenv("PROJECTS_YAML_PATH")
	if projectsPath == "" {
		projectsPath = "https://raw.githubusercontent.com/cncf/devstats/master/projects.yaml"
	}
	var dataP []byte
	if strings.Contains(projectsPath, "https://") || strings.Contains(projectsPath, "http://") {
		var response *http.Response
		response, err = http.Get(projectsPath)
		if err != nil {
			fmt.Printf("http.Get '%s' -> %+v", projectsPath, err)
			return
		}
		defer func() { _ = response.Body.Close() }()
		dataP, err = ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Printf("ioutil.ReadAll '%s' -> %+v", projectsPath, err)
			return
		}
	} else {
		dataP, err = ioutil.ReadFile(projectsPath)
		if err != nil {
			fmt.Printf("unable to read file '%s': %v", projectsPath, err)
			return
		}
	}
	var landscape types.LandscapeList
	err = yaml.Unmarshal(dataL, &landscape)
	if err != nil {
		fmt.Printf("yaml.Unmarshal '%s' -> %+v", landscapePath, err)
		return
	}
	fmt.Printf("landscape:\n%+v\n", landscape)
	var projects devstatscode.AllProjects
	err = yaml.Unmarshal(dataP, &projects)
	if err != nil {
		fmt.Printf("yaml.Unmarshal '%s' -> %+v", projectsPath, err)
		return
	}
	fmt.Printf("projects:\n%+v\n", projects)
	return
}

func main() {
	dtStart := time.Now()
	err := checkSync()
	dtEnd := time.Now()
	fmt.Printf("Time: %v\n", dtEnd.Sub(dtStart))
	if err != nil {
		os.Exit(1)
	}
}

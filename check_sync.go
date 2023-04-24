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
	// Some names are different in DevStats than in landscape.yml (not so many for 170+ projects)
	devstats2landscape := map[string]string{
		"foniod":                              "fonio",
		"litmuschaos":                         "litmus",
		"opa":                                 "open policy agent (opa)",
		"tuf":                                 "the update framework (tuf)",
		"opcr":                                "open policy containers",
		"cni":                                 "container network interface (cni)",
		"cloud deployment kit for kubernetes": "cdk for kubernetes (cdk8S)",
		"piraeus-datastore":                   "piraeus datastore",
		"external secrets operator":           "external-secrets",
		"smi":                                 "service mesh interface (smi)",
		"hexa policy orchestrator":            "hexa",
	}
	// all (All CNCF) is a special project in DevStats containing all CNCF projects as repo groups - so it is not in landscape.yaml
	// Others are missing in landscape.yml, while they are present in DevStats
	skipList := map[string]struct{}{
		"gitopswg":        {},
		"all":             {},
		"vscodek8stools":  {},
		"kubevip":         {},
		"inspektorgadget": {},
	}
	// Some projects in Landscape are listed twice
	// Fort example Cilum was renamed to Tetragon and is listed twice
	// Those entries should not be reported as missing in DevStats
	// "Traefik Mesh" kinda mapped to SMI in landscape, while there is also a separate entry for SMI matching it better
	ignoreMissing := map[string]struct{}{
		"tetragon":     {},
		"traefik mesh": {},
	}
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
	var projects devstatscode.AllProjects
	err = yaml.Unmarshal(dataP, &projects)
	if err != nil {
		fmt.Printf("yaml.Unmarshal '%s' -> %+v", projectsPath, err)
		return
	}
	projectsNames := make(map[string]struct{})
	namesMapping := make(map[string]string)
	landscapeNames := make(map[string]struct{})
	disabledProjects := make(map[string]struct{})
	joinDatesP := make(map[string]string)
	joinDatesL := make(map[string]string)
	for name, data := range projects.Projects {
		/*
			MainRepo         string            `yaml:"main_repo"`
			FullName         string            `yaml:"name"`
			Status           string            `yaml:"status"`
			IncubatingDate   *time.Time        `yaml:"incubating_date"`
			GraduatedDate    *time.Time        `yaml:"graduated_date"`
			ArchivedDate     *time.Time        `yaml:"archived_date"`
		*/
		name = strings.ToLower(name)
		_, skip := skipList[name]
		if skip {
			continue
		}
		if data.Disabled {
			disabledProjects[name] = struct{}{}
			continue
		}
		fullName := strings.ToLower(data.FullName)
		mapped, ok := devstats2landscape[fullName]
		if ok {
			fullName = mapped
		}
		fullName = strings.ToLower(fullName)
		projectsNames[fullName] = struct{}{}
		if name != fullName {
			namesMapping[name] = fullName
			namesMapping[fullName] = name
		}
		joinDatesP[fullName] = data.JoinDate.Format("2006-01-02")
	}
	for _, data := range landscape.Landscape {
		for _, scat := range data.Subcategories {
			for _, item := range scat.Items {
				name := strings.ToLower(item.Name)
				_, ok := projectsNames[name]
				if !ok {
					mappedName, okMapped := namesMapping[name]
					if okMapped {
						_, ok = projectsNames[mappedName]
						if ok {
							name = mappedName
						}
					}
				}
				// Project can be missing in DevStats:projects.yaml
				if !ok && item.Extra.Accepted != "" {
					_, disabled := disabledProjects[name]
					_, ignored := ignoreMissing[name]
					if !disabled && !ignored {
						fmt.Printf("error: missing '%s' in devstats projects\n", name)
					}
				}
				if ok {
					landscapeNames[name] = struct{}{}
					//joinDatesL[data.FullName] = data.JoinDate.Format("2006-01-02")
				}
			}
		}
	}
	fmt.Printf("Projects join dates:\n%+v\n", joinDatesP)
	fmt.Printf("Landscape join dates:\n%+v\n", joinDatesL)
	fmt.Printf("Names mapping:\n%+v\n", namesMapping)
	for name := range projectsNames {
		_, ok := landscapeNames[name]
		if !ok {
			fmt.Printf("error: missing '%s' in landscape\n", name)
		}
	}
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
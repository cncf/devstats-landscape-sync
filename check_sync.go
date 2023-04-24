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
		// "gitops wg":                           "opengitops",
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
	// "opengitops" is marked as Sandbox project in landscape but there is no more info and I belive no such project was added (there is no DEvStats page for it)
	ignoreMissing := map[string]struct{}{
		"tetragon":     {},
		"traefik mesh": {},
		"opengitops":   {},
	}
	// Some landscape RepoURL entries are not matching DevStats and those where DevStats is correct are ignored here
	ignoreRepo := map[string]struct{}{
		"capsule":              {},
		"sealer":               {},
		"network service mesh": {},
	}
	// Some projects have wrong join date in landscape.yml, ignore this
	// KubeDL joined at the same day as few projects before and landscape.yml is 1 year off
	// Capsue has no join data in landscape.yml
	// landscape 'curve' join date '2022-09-14' is not equal to devstats join date '2022-06-17'
	// landscape 'clusterpedia' join date '2022-6-17' is not equal to devstats join date '2022-06-17'
	ignoreJoinDate := map[string]struct{}{
		"kubedl":       {},
		"capsule":      {},
		"curve":        {},
		"clusterpedia": {},
	}
	// Some incubating dates present in landscape and not present in DevStats can be ignored: this is for projects which joined with level >= incubating
	// Such projects have no incubation dates in DevStats becaus ethey were at least such at join time
	// The opposite is not true, we shoudl always have incubating dates in landscape.yml
	// "kubevirt" has no incubation date in landscape.yml and it moved to incubation but date is unknown
	ignoreIncubatingDate := map[string]struct{}{
		"kubevirt": {},
	}
	ignoreGraduatedDate := map[string]struct{}{}
	// To ignore specific projects tstatuses after confirmed they are OK
	// Capsule is missing in landscape.yml while MetalLB has no maturity level specified.
	ignoreStatus := map[string]struct{}{
		"capsule": {},
		"metallb": {},
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
	reposP := make(map[string]string)
	joinDatesP := make(map[string]string)
	incubatingDatesP := make(map[string]string)
	graduatedDatesP := make(map[string]string)
	reposL := make(map[string]string)
	joinDatesL := make(map[string]string)
	incubatingDatesL := make(map[string]string)
	graduatedDatesL := make(map[string]string)
	projectsByStateP := make(map[string]map[string]struct{})
	projectsByStateL := make(map[string]map[string]struct{})
	for name, data := range projects.Projects {
		// ArchivedDate *time.Time  `yaml:"archived_date"`
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
		reposP[fullName] = strings.TrimSpace(strings.ToLower(data.MainRepo))
		joinDatesP[fullName] = data.JoinDate.Format("2006-01-02")
		if data.IncubatingDate != nil {
			incubatingDatesP[fullName] = data.IncubatingDate.Format("2006-01-02")
		}
		if data.GraduatedDate != nil {
			graduatedDatesP[fullName] = data.GraduatedDate.Format("2006-01-02")
		}
		status := strings.TrimSpace(strings.ToLower(data.Status))
		_, ok = projectsByStateP[status]
		if !ok {
			projectsByStateP[status] = make(map[string]struct{})
		}
		projectsByStateP[status][fullName] = struct{}{}
	}
	devstatsMiss := 0
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
				status := strings.TrimSpace(strings.ToLower(item.Project))
				// Project can be missing in DevStats:projects.yaml
				if !ok && (item.Extra.Accepted != "" || status != "") {
					_, disabled := disabledProjects[name]
					_, ignored := ignoreMissing[name]
					if !disabled && !ignored {
						fmt.Printf("error: missing '%s' in devstats projects\n", name)
						devstatsMiss++
					}
				}
				if !ok {
					continue
				}
				var (
					joinDt  string
					incubDt string
				)
				landscapeNames[name] = struct{}{}
				_, present := reposL[name]
				if !present && item.RepoURL != "" {
					reposL[name] = strings.Replace(strings.TrimSpace(strings.ToLower(item.RepoURL)), "https://github.com/", "", -1)
				}
				_, present = joinDatesL[name]
				// Only first specified date will be used, no overwrite, especially with blank data
				if !present && item.Extra.Accepted != "" {
					dtS := strings.TrimSpace(item.Extra.Accepted)
					if len(dtS) > 10 {
						dtS = dtS[:10]
					}
					joinDatesL[name] = dtS
					joinDt = dtS
				}
				_, present = incubatingDatesL[name]
				if !present && item.Extra.Incubating != "" {
					dtS := strings.TrimSpace(item.Extra.Incubating)
					if len(dtS) > 10 {
						dtS = dtS[:10]
					}
					if dtS > joinDt {
						incubatingDatesL[name] = dtS
						incubDt = dtS
					}
				}
				_, present = graduatedDatesL[name]
				if !present && item.Extra.Graduated != "" {
					dtS := strings.TrimSpace(item.Extra.Graduated)
					if len(dtS) > 10 {
						dtS = dtS[:10]
					}
					if (incubDt == "" && dtS > joinDt) || (incubDt != "" && dtS > incubDt && dtS > joinDt) {
						graduatedDatesL[name] = dtS
					}
				}
				if status != "" {
					_, ok = projectsByStateL[status]
					if !ok {
						projectsByStateL[status] = make(map[string]struct{})
					}
					projectsByStateL[status][name] = struct{}{}
				}
			}
		}
	}
	landscapeMiss := 0
	for name := range projectsNames {
		_, ok := landscapeNames[name]
		if !ok {
			fmt.Printf("error: missing '%s' in landscape\n", name)
			landscapeMiss++
		}
	}
	reposErrs := make(map[string]struct{})
	for project, repoL := range reposL {
		_, ignore := ignoreRepo[project]
		if ignore {
			continue
		}
		repoP, ok := reposP[project]
		if !ok {
			fmt.Printf("error: landscape '%s' repo '%s' is missing in devstats\n", project, repoL)
			reposErrs[project] = struct{}{}
			continue
		}
		if repoL != repoP {
			fmt.Printf("error: landscape '%s' repo '%s' is not equal to devstats repo '%s'\n", project, repoL, repoP)
			reposErrs[project] = struct{}{}
		}
	}
	for project, repoP := range reposP {
		_, ignore := ignoreRepo[project]
		if ignore {
			continue
		}
		repoL, ok := reposL[project]
		if !ok {
			fmt.Printf("error: devstats '%s' repo '%s' is missing in landscape\n", project, repoP)
			reposErrs[project] = struct{}{}
			continue
		}
		if repoL != repoP {
			_, reported := reposErrs[project]
			if !reported {
				fmt.Printf("error: devstats '%s' repo '%s' is not equal to landscape repo '%s'\n", project, repoP, repoL)
				reposErrs[project] = struct{}{}
			}
		}
	}
	if len(reposErrs) > 0 {
		fmt.Printf("%d repos mismatches detected\n", len(reposErrs))
	}
	joinDatesErrs := make(map[string]struct{})
	for project, joinDateL := range joinDatesL {
		_, ignore := ignoreJoinDate[project]
		if ignore {
			continue
		}
		joinDateP, ok := joinDatesP[project]
		if !ok {
			fmt.Printf("error: landscape '%s' join date '%s' is missing in devstats\n", project, joinDateL)
			joinDatesErrs[project] = struct{}{}
			continue
		}
		if joinDateL != joinDateP {
			fmt.Printf("error: landscape '%s' join date '%s' is not equal to devstats join date '%s'\n", project, joinDateL, joinDateP)
			joinDatesErrs[project] = struct{}{}
		}
	}
	for project, joinDateP := range joinDatesP {
		_, ignore := ignoreJoinDate[project]
		if ignore {
			continue
		}
		joinDateL, ok := joinDatesL[project]
		if !ok {
			fmt.Printf("error: devstats '%s' join date '%s' is missing in landscape\n", project, joinDateP)
			joinDatesErrs[project] = struct{}{}
			continue
		}
		if joinDateL != joinDateP {
			_, reported := joinDatesErrs[project]
			if !reported {
				fmt.Printf("error: devstats '%s' join date '%s' is not equal to landscape join date '%s'\n", project, joinDateP, joinDateL)
				joinDatesErrs[project] = struct{}{}
			}
		}
	}
	if len(joinDatesErrs) > 0 {
		fmt.Printf("%d join dates mismatches detected\n", len(joinDatesErrs))
	}
	incubatingDatesErrs := make(map[string]struct{})
	for project, incubatingDateL := range incubatingDatesL {
		_, ignore := ignoreIncubatingDate[project]
		if ignore {
			continue
		}
		incubatingDateP, ok := incubatingDatesP[project]
		if !ok {
			fmt.Printf("error: landscape '%s' incubating date '%s' is missing in devstats\n", project, incubatingDateL)
			incubatingDatesErrs[project] = struct{}{}
			continue
		}
		if incubatingDateL != incubatingDateP {
			fmt.Printf("error: landscape '%s' incubating date '%s' is not equal to devstats incubating date '%s'\n", project, incubatingDateL, incubatingDateP)
			incubatingDatesErrs[project] = struct{}{}
		}
	}
	for project, incubatingDateP := range incubatingDatesP {
		_, ignore := ignoreIncubatingDate[project]
		if ignore {
			continue
		}
		incubatingDateL, ok := incubatingDatesL[project]
		if !ok {
			fmt.Printf("error: devstats '%s' incubating date '%s' is missing in landscape\n", project, incubatingDateP)
			incubatingDatesErrs[project] = struct{}{}
			continue
		}
		if incubatingDateL != incubatingDateP {
			_, reported := incubatingDatesErrs[project]
			if !reported {
				fmt.Printf("error: devstats '%s' incubating date '%s' is not equal to landscape incubating date '%s'\n", project, incubatingDateP, incubatingDateL)
				incubatingDatesErrs[project] = struct{}{}
			}
		}
	}
	if len(incubatingDatesErrs) > 0 {
		fmt.Printf("%d incubating dates mismatches detected\n", len(incubatingDatesErrs))
	}
	graduatedDatesErrs := make(map[string]struct{})
	for project, graduatedDateL := range graduatedDatesL {
		_, ignore := ignoreGraduatedDate[project]
		if ignore {
			continue
		}
		graduatedDateP, ok := graduatedDatesP[project]
		if !ok {
			fmt.Printf("error: landscape '%s' graduated date '%s' is missing in devstats\n", project, graduatedDateL)
			graduatedDatesErrs[project] = struct{}{}
			continue
		}
		if graduatedDateL != graduatedDateP {
			fmt.Printf("error: landscape '%s' graduated date '%s' is not equal to devstats graduated date '%s'\n", project, graduatedDateL, graduatedDateP)
			graduatedDatesErrs[project] = struct{}{}
		}
	}
	for project, graduatedDateP := range graduatedDatesP {
		_, ignore := ignoreGraduatedDate[project]
		if ignore {
			continue
		}
		graduatedDateL, ok := graduatedDatesL[project]
		if !ok {
			fmt.Printf("error: devstats '%s' graduated date '%s' is missing in landscape\n", project, graduatedDateP)
			graduatedDatesErrs[project] = struct{}{}
			continue
		}
		if graduatedDateL != graduatedDateP {
			_, reported := graduatedDatesErrs[project]
			if !reported {
				fmt.Printf("error: devstats '%s' graduated date '%s' is not equal to landscape graduated date '%s'\n", project, graduatedDateP, graduatedDateL)
				graduatedDatesErrs[project] = struct{}{}
			}
		}
	}
	if len(graduatedDatesErrs) > 0 {
		fmt.Printf("%d graduated dates mismatches detected\n", len(graduatedDatesErrs))
	}
	statusCountsL := make(map[string]int)
	statusCountsP := make(map[string]int)
	statusErrs := make(map[string]struct{})
	for status, projects := range projectsByStateL {
		for project := range projects {
			_, ignore := ignoreStatus[project]
			if ignore {
				continue
			}
			_, ok := projectsByStateP[status][project]
			if !ok {
				fmt.Printf("error: landscape %s '%s' is missing", status, project)
				for otherStatus := range projectsByStateP {
					_, ok := projectsByStateP[otherStatus][project]
					if ok {
						fmt.Printf(", but is present in %s", otherStatus)
						break
					}
				}
				fmt.Printf("\n")
				statusErrs[project] = struct{}{}
				continue
			}
			count, ok := statusCountsL[status]
			if !ok {
				statusCountsL[status] = 1
				continue
			}
			statusCountsL[status] = count + 1
		}
	}
	for status, projects := range projectsByStateP {
		for project := range projects {
			_, ignore := ignoreStatus[project]
			if ignore {
				continue
			}
			_, ok := projectsByStateL[status][project]
			if !ok {
				_, reported := statusErrs[project]
				if !reported {
					fmt.Printf("error: devstats %s '%s' is missing", status, project)
					for otherStatus := range projectsByStateL {
						_, ok := projectsByStateL[otherStatus][project]
						if ok {
							fmt.Printf(", but is present in %s", otherStatus)
							break
						}
					}
					fmt.Printf("\n")
					statusErrs[project] = struct{}{}
				}
			}
			count, ok := statusCountsP[status]
			if !ok {
				statusCountsP[status] = 1
				continue
			}
			statusCountsP[status] = count + 1
		}
	}
	if len(statusErrs) > 0 {
		fmt.Printf("%d status mismatches detected\n", len(statusErrs))
	}
	for status, countL := range statusCountsL {
		countP, ok := statusCountsP[status]
		if ok && countP == countL {
			fmt.Printf("%s: %d projects\n", status, countL)
			continue
		}
		fmt.Printf("%s: %d landscape projects, %d devstats projects\n", status, countL, countP)
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

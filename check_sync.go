package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cncf/devstatscode"
	"github.com/cncf/landscape/pkg/types"
	yaml "gopkg.in/yaml.v2"
)

func execCommandWithStdin(cmdAndArgs []string, stdIn *bytes.Buffer) (string, error) {
	var (
		stdOut bytes.Buffer
		stdErr bytes.Buffer
	)
	command := cmdAndArgs[0]
	arguments := cmdAndArgs[1:]
	cmd := exec.Command(command, arguments...)
	cmd.Stderr = &stdErr
	cmd.Stdout = &stdOut
	cmd.Stdin = stdIn
	err := cmd.Start()
	if err != nil {
		return "", err
	}
	err = cmd.Wait()
	if err != nil {
		outStr := stdOut.String()
		if len(outStr) > 0 {
			fmt.Printf("STDOUT:\n%v\n", outStr)
		}
		errStr := stdErr.String()
		if len(errStr) > 0 {
			fmt.Printf("STDERR:\n%v\n", errStr)
		}
		return stdOut.String(), err
	}
	outStr := stdOut.String()
	return outStr, nil
}

func sendStatusEmail(body, recipients string) error {
	fmt.Printf("sending email(s) to %s\n", recipients)
	title := "DevStats <=> landscape sync status"
	html := "<!DOCTYPE html>\n<html>\n<head>\n  <meta charset=\"utf-8\">\n  <title>%s</title>\n</head>\n<body>\n%s\n</body>\n</html>\n"
	htmlBody := fmt.Sprintf(html, title, strings.Replace(body, "\n", "<br/>\n", -1))
	hostname, _ := os.Hostname()
	hostname += ".io"
	ary := strings.Split(recipients, ",")
	for _, recipient := range ary {
		recipient = strings.TrimSpace(recipient)
		//fmt.Printf("sending email to %s\n", recipient)
		data := fmt.Sprintf(
			"From: devstats-landscape-sync@%s\n"+
				"To: %s\n"+
				"Subject: %s\n"+
				"Content-Type: text/html\n"+
				"MIME-Version: 1.0\n"+
				"\n"+
				"%s\n",
			hostname,
			recipient,
			title,
			htmlBody,
		)
		res, err := execCommandWithStdin([]string{"sendmail", recipient}, bytes.NewBuffer([]byte(data)))
		if err != nil {
			fmt.Printf("Error sending email to %s: %+v\n%s\n", recipient, err, res)
		}
		fmt.Printf("sent email to %s\n", recipient)
	}
	return nil
}

func checkSync() (err error) {
	dtStart := time.Now()
	recipients := os.Getenv("EMAIL_TO")
	if recipients == "" {
		recipients = "lukaszgryglicki@o2.pl,lgryglicki@cncf.io"
	}
	msgs := []string{}
	report := false
	msgPrintf := func(format string, args ...interface{}) {
		str := fmt.Sprintf(format, args...)
		msgs = append(msgs, str)
	}
	defer func() {
		if report {
			for _, msg := range msgs {
				fmt.Printf("%s", msg)
			}
			skipEmail := os.Getenv("SKIP_EMAIL")
			if skipEmail == "" {
				sendStatusEmail(strings.Join(msgs, ""), recipients)
			}
		}
		dtEnd := time.Now()
		fmt.Printf("time: %v\n", dtEnd.Sub(dtStart))
	}()
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
	// "opengitops" is marked as Sandbox project in landscape but there is no more info and I belive no such project was added (there is no DevStats page for it)
	ignoreMissing := map[string]struct{}{
		"tetragon":     {},
		"traefik mesh": {},
		"opengitops":   {},
	}
	// Some landscape RepoURL entries are not matching DevStats and those where DevStats is correct are ignored here
	// For some repos we know that landscape.yml has other repo than DevStats
	// For example Sealer still has an old Alibaba repo 'alibaba/sealer'
	// NSM (network service mesh) refers to an old archived repo 'networkservicemesh/networkservicemesh'
	// For notary -> notation (V1 to V2) there are not enough tags yet on V2 and much less commits, so we prefer to use V1 in DevStats
	// Also there was a discussion about splitting out Notary V2 into a separate project, so not updating main repo to match landscape
	// Knative landscape repo 'community' is way smaller than devstats one 'serving' and has no releases, so DevStats continues to use its own
	// OCM (open cluster management) devstats's repo 'api' has more commits and has tags, while landscape 'ocm' has no tags/releases
	// Same with OpenTelemetry 'opentelemetry-java' vs. 'community' repos (less commits and no tags/releases on community repo).
	// For OpenFeature 'community' repo has more commits than 'spec', but the latter has tags/releases needed for
	// DevStats to build annotations/ranges - so DevStats uses 'spec' repo
	// Format is 2 strings: expected landscape repo, expected devstats repo
	ignoreRepo := map[string][2]string{
		"capsule":                 {"clastix/capsule", "capsule-rs/capsule"},
		"sealer":                  {"alibaba/sealer", "sealerio/sealer"},
		"network service mesh":    {"networkservicemesh/networkservicemesh", "networkservicemesh/api"},
		"confidential containers": {"confidential-containers/documentation", "confidential-containers/operator"},
		"piraeus datastore":       {"piraeusdatastore/piraeus", "piraeusdatastore/piraeus-operator"},
		"devspace":                {"devspace-sh/devspace", "devspace-cloud/devspace-cloud"},
		"notary":                  {"notaryproject/notary", "notaryproject/notation"},
		"knative":                 {"knative/community", "knative/serving"},
		"open cluster management": {"open-cluster-management-io/ocm", "open-cluster-management-io/api"},
		"opentelemetry":           {"open-telemetry/community", "open-telemetry/opentelemetry-java"},
		"openfeature":             {"open-feature/community", "open-feature/spec"},
	}
	// Some projects have wrong join date in landscape.yml, ignore this
	// KubeDL joined at the same day as few projects before and landscape.yml is 1 year off
	// Capsue has no join data in landscape.yml
	// landscape 'curve' join date '2022-09-14' is not equal to devstats join date '2022-06-17'
	// landscape 'clusterpedia' join date '2022-6-17' is not equal to devstats join date '2022-06-17' (but technically the same)
	ignoreJoinDate := map[string]struct{}{
		"kubedl":       {},
		"capsule":      {},
		"curve":        {},
		"clusterpedia": {},
	}
	// Some incubating dates present in landscape and not present in DevStats can be ignored: this is for projects which joined with level >= incubating
	// Such projects have no incubation dates in DevStats because they were at least such at join time
	// The opposite is not true, we should always have incubating dates in landscape.yml
	// "kubevirt" had no incubation date in landscape.yml and it moved to incubation but date is unknown: this was fixed in landscape at 4/25/23.
	ignoreIncubatingDate := map[string]struct{}{}
	ignoreGraduatedDate := map[string]struct{}{}
	// To ignore specific projects tstatuses after confirmed they are OK
	// Capsule is missing in landscape.yml while MetalLB has no maturity level specified.
	ignoreStatus := map[string]struct{}{
		"capsule": {},
		"metallb": {},
	}
	// Read landscape.yml
	landscapePath := os.Getenv("LANDSCAPE_YAML_PATH")
	if landscapePath == "" {
		landscapePath = "https://raw.githubusercontent.com/cncf/landscape/master/landscape.yml"
	}
	var dataL []byte
	if strings.Contains(landscapePath, "https://") || strings.Contains(landscapePath, "http://") {
		var response *http.Response
		response, err = http.Get(landscapePath)
		if err != nil {
			msgPrintf("http.Get '%s' -> %+v", landscapePath, err)
			report = true
			return
		}
		defer func() { _ = response.Body.Close() }()
		dataL, err = ioutil.ReadAll(response.Body)
		if err != nil {
			msgPrintf("ioutil.ReadAll '%s' -> %+v", landscapePath, err)
			report = true
			return
		}
	} else {
		dataL, err = ioutil.ReadFile(landscapePath)
		if err != nil {
			msgPrintf("ioutil.Readfile: unable to read file '%s': %v", landscapePath, err)
			report = true
			return
		}
	}
	// Read devstats projects.yaml
	projectsPath := os.Getenv("PROJECTS_YAML_PATH")
	if projectsPath == "" {
		projectsPath = "https://raw.githubusercontent.com/cncf/devstats/master/projects.yaml"
	}
	var dataP []byte
	if strings.Contains(projectsPath, "https://") || strings.Contains(projectsPath, "http://") {
		var response *http.Response
		response, err = http.Get(projectsPath)
		if err != nil {
			msgPrintf("http.Get '%s' -> %+v", projectsPath, err)
			report = true
			return
		}
		defer func() { _ = response.Body.Close() }()
		dataP, err = ioutil.ReadAll(response.Body)
		if err != nil {
			msgPrintf("ioutil.ReadAll '%s' -> %+v", projectsPath, err)
			report = true
			return
		}
	} else {
		dataP, err = ioutil.ReadFile(projectsPath)
		if err != nil {
			msgPrintf("ioutil.ReadFile: unable to read file '%s': %v", projectsPath, err)
			report = true
			return
		}
	}
	// Read devstats-docker-images projects.yaml
	projects2Path := os.Getenv("DOCKER_PROJECTS_YAML_PATH")
	if projects2Path == "" {
		projects2Path = "https://raw.githubusercontent.com/cncf/devstats-docker-images/master/devstats-helm/projects.yaml"
	}
	var dataP2 []byte
	if strings.Contains(projects2Path, "https://") || strings.Contains(projects2Path, "http://") {
		var response *http.Response
		response, err = http.Get(projects2Path)
		if err != nil {
			msgPrintf("http.Get '%s' -> %+v", projects2Path, err)
			report = true
			return
		}
		defer func() { _ = response.Body.Close() }()
		dataP2, err = ioutil.ReadAll(response.Body)
		if err != nil {
			msgPrintf("ioutil.ReadAll '%s' -> %+v", projects2Path, err)
			report = true
			return
		}
	} else {
		dataP2, err = ioutil.ReadFile(projects2Path)
		if err != nil {
			msgPrintf("ioutil.ReadFile: unable to read file '%s': %v", projects2Path, err)
			report = true
			return
		}
	}
	// All yamls read
	var landscape types.LandscapeList
	err = yaml.Unmarshal(dataL, &landscape)
	if err != nil {
		msgPrintf("yaml.Unmarshal '%s' -> %+v", landscapePath, err)
		report = true
		return
	}
	var projects devstatscode.AllProjects
	err = yaml.Unmarshal(dataP, &projects)
	if err != nil {
		msgPrintf("yaml.Unmarshal '%s' -> %+v", projectsPath, err)
		report = true
		return
	}
	var projects2 devstatscode.AllProjects
	err = yaml.Unmarshal(dataP2, &projects2)
	if err != nil {
		msgPrintf("yaml.Unmarshal '%s' -> %+v", projects2Path, err)
		report = true
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
	// Iterate devstats projects.yaml
	for name, data := range projects.Projects {
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
	// Iterate devstats-docker-images projects.yaml
	diffFromDocker := 0
	for name, data := range projects2.Projects {
		name = strings.ToLower(name)
		_, skip := skipList[name]
		if skip {
			continue
		}
		if data.Disabled {
			continue
		}
		status := strings.TrimSpace(strings.ToLower(data.Status))
		if status == "-" {
			continue
		}
		fullName := strings.ToLower(data.FullName)
		mapped, ok := devstats2landscape[fullName]
		if ok {
			fullName = mapped
		}
		fullName = strings.ToLower(fullName)
		_, ok = projectsNames[fullName]
		if !ok {
			msgPrintf("error: missing docker project in devstats projects: '%s'\n", fullName)
			report = true
			diffFromDocker++
		}
		_, ok = projectsByStateP[status][fullName]
		if !ok {
			msgPrintf("error: missing or different status of docker project in devstats projects: %s '%s'\n", status, fullName)
			report = true
			diffFromDocker++
		}
		repoD := strings.TrimSpace(strings.ToLower(data.MainRepo))
		repoP, ok := reposP[fullName]
		if !ok || repoP != repoD {
			msgPrintf("error: missing or different docker main repo in devstats projects: %s '%s' <=> '%s'\n", fullName, repoD, repoP)
			report = true
			diffFromDocker++
		}
		joinDateD := data.JoinDate.Format("2006-01-02")
		joinDateP, ok := joinDatesP[fullName]
		if !ok || joinDateP != joinDateD {
			msgPrintf("error: missing or different docker join date in devstats projects: %s '%s' <=> '%s'\n", fullName, joinDateD, joinDateP)
			report = true
			diffFromDocker++
		}
		if data.IncubatingDate != nil {
			incubatingDateD := data.IncubatingDate.Format("2006-01-02")
			incubatingDateP, ok := incubatingDatesP[fullName]
			if !ok || incubatingDateP != incubatingDateD {
				msgPrintf("error: missing or different docker incubating date in devstats projects: %s '%s' <=> '%s'\n", fullName, incubatingDateD, incubatingDateP)
				report = true
				diffFromDocker++
			}
		}
		if data.GraduatedDate != nil {
			graduatedDateD := data.GraduatedDate.Format("2006-01-02")
			graduatedDateP, ok := graduatedDatesP[fullName]
			if !ok || graduatedDateP != graduatedDateD {
				msgPrintf("error: missing or different docker graduated date in devstats projects: %s '%s' <=> '%s'\n", fullName, graduatedDateD, graduatedDateP)
				report = true
				diffFromDocker++
			}
		}
	}
	if diffFromDocker > 0 {
		msgPrintf("error: devstats-docker-images projects.yaml differences vs devstats projects.yaml: %d\n", diffFromDocker)
		report = true
	}
	// Iterate landscape.yml
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
						msgPrintf("error: missing in devstats projects: '%s'\n", name)
						report = true
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
			msgPrintf("error: missing in landscape: '%s'\n", name)
			report = true
			landscapeMiss++
		}
	}
	reposErrs := make(map[string]struct{})
	for project, repoL := range reposL {
		ignored, ignore := ignoreRepo[project]
		if ignore {
			if ignored[0] == repoL {
				continue
			}
			msgPrintf("error: ignored landscape repo is incorrect '%s' '%s' <=> '%s'\n", project, repoL, ignored[0])
			report = true
			reposErrs[project] = struct{}{}
			continue
		}
		repoP, ok := reposP[project]
		if !ok {
			msgPrintf("error: landscape repo missing in devstats '%s' '%s'\n", project, repoL)
			report = true
			reposErrs[project] = struct{}{}
			continue
		}
		if repoL != repoP {
			msgPrintf("error: landscape repo not equal to devstats repo '%s' '%s' <=> '%s'\n", project, repoL, repoP)
			report = true
			reposErrs[project] = struct{}{}
		}
	}
	for project, repoP := range reposP {
		ignored, ignore := ignoreRepo[project]
		if ignore {
			if ignored[1] == repoP {
				continue
			}
			msgPrintf("error: ignored devstats repo is incorrect '%s' '%s' <=> '%s'\n", project, repoP, ignored[1])
			report = true
			reposErrs[project] = struct{}{}
			continue
		}
		repoL, ok := reposL[project]
		if !ok {
			msgPrintf("error: devstats repo missing in landscape '%s' '%s'\n", project, repoP)
			report = true
			reposErrs[project] = struct{}{}
			continue
		}
		if repoL != repoP {
			_, reported := reposErrs[project]
			if !reported {
				msgPrintf("error: devstats repo not equal to landscape repo '%s' '%s' <=> '%s'\n", project, repoP, repoL)
				report = true
				reposErrs[project] = struct{}{}
			}
		}
	}
	if len(reposErrs) > 0 {
		msgPrintf("error: repos mismatches detected: %d\n", len(reposErrs))
		report = true
	}
	joinDatesErrs := make(map[string]struct{})
	for project, joinDateL := range joinDatesL {
		_, ignore := ignoreJoinDate[project]
		if ignore {
			continue
		}
		joinDateP, ok := joinDatesP[project]
		if !ok {
			msgPrintf("error: landscape join date missing in devstats '%s' '%s'\n", project, joinDateL)
			report = true
			joinDatesErrs[project] = struct{}{}
			continue
		}
		if joinDateL != joinDateP {
			msgPrintf("error: landscape join date not equal to devstats join date '%s' '%s' <=> '%s'\n", project, joinDateL, joinDateP)
			report = true
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
			msgPrintf("error: devstats join adte missig in landscape '%s' '%s'\n", project, joinDateP)
			report = true
			joinDatesErrs[project] = struct{}{}
			continue
		}
		if joinDateL != joinDateP {
			_, reported := joinDatesErrs[project]
			if !reported {
				msgPrintf("error: devstats join date not equal to landscape join date '%s' '%s' <=> '%s'\n", project, joinDateP, joinDateL)
				report = true
				joinDatesErrs[project] = struct{}{}
			}
		}
	}
	if len(joinDatesErrs) > 0 {
		msgPrintf("error: %d join dates mismatches detected\n", len(joinDatesErrs))
	}
	incubatingDatesErrs := make(map[string]struct{})
	for project, incubatingDateL := range incubatingDatesL {
		_, ignore := ignoreIncubatingDate[project]
		if ignore {
			continue
		}
		incubatingDateP, ok := incubatingDatesP[project]
		if !ok {
			msgPrintf("error: landscape incubating date missing in devstats '%s' '%s'\n", project, incubatingDateL)
			report = true
			incubatingDatesErrs[project] = struct{}{}
			continue
		}
		if incubatingDateL != incubatingDateP {
			msgPrintf("error: landscape incubating date is not equal to devstats incubating date '%s' '%s' <=> '%s'\n", project, incubatingDateL, incubatingDateP)
			report = true
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
			msgPrintf("error: devstats incubating date missing in landscape '%s' '%s'\n", project, incubatingDateP)
			report = true
			incubatingDatesErrs[project] = struct{}{}
			continue
		}
		if incubatingDateL != incubatingDateP {
			_, reported := incubatingDatesErrs[project]
			if !reported {
				msgPrintf("error: devstats incubating date is not equal to landscape incubating date '%s' '%s' <=> '%s'\n", project, incubatingDateP, incubatingDateL)
				report = true
				incubatingDatesErrs[project] = struct{}{}
			}
		}
	}
	if len(incubatingDatesErrs) > 0 {
		msgPrintf("error: incubating dates mismatches detected: %d\n", len(incubatingDatesErrs))
		report = true
	}
	graduatedDatesErrs := make(map[string]struct{})
	for project, graduatedDateL := range graduatedDatesL {
		_, ignore := ignoreGraduatedDate[project]
		if ignore {
			continue
		}
		graduatedDateP, ok := graduatedDatesP[project]
		if !ok {
			msgPrintf("error: landscape graduated date missing in devstats '%s' '%s'\n", project, graduatedDateL)
			report = true
			graduatedDatesErrs[project] = struct{}{}
			continue
		}
		if graduatedDateL != graduatedDateP {
			msgPrintf("error: landscape graduated date not equal to devstats graduated date '%s' '%s' <=> '%s'\n", project, graduatedDateL, graduatedDateP)
			report = true
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
			msgPrintf("error: devstats graduated date missing in landscape '%s' '%s'\n", project, graduatedDateP)
			report = true
			graduatedDatesErrs[project] = struct{}{}
			continue
		}
		if graduatedDateL != graduatedDateP {
			_, reported := graduatedDatesErrs[project]
			if !reported {
				msgPrintf("error: devstats graduated date not equal to landscape graduated date '%s' '%s' <=> '%s'\n", project, graduatedDateP, graduatedDateL)
				report = true
				graduatedDatesErrs[project] = struct{}{}
			}
		}
	}
	if len(graduatedDatesErrs) > 0 {
		msgPrintf("error: graduated dates mismatches detected: %d\n", len(graduatedDatesErrs))
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
				msgPrintf("error: landscape is missing %s '%s'", status, project)
				report = true
				for otherStatus := range projectsByStateP {
					_, ok := projectsByStateP[otherStatus][project]
					if ok {
						msgPrintf(", but is present in %s", otherStatus)
						break
					}
				}
				msgPrintf("\n")
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
					msgPrintf("error: devstats is missing %s '%s'", status, project)
					report = true
					for otherStatus := range projectsByStateL {
						_, ok := projectsByStateL[otherStatus][project]
						if ok {
							msgPrintf(", but is present in %s", otherStatus)
							break
						}
					}
					msgPrintf("\n")
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
		msgPrintf("error: status mismatches detected: %d\n", len(statusErrs))
		report = true
	}
	for status, countL := range statusCountsL {
		countP, ok := statusCountsP[status]
		if ok && countP == countL {
			msgPrintf("%s: %d projects\n", status, countL)
			continue
		}
		msgPrintf("error: %s: %d landscape projects, %d devstats projects\n", status, countL, countP)
		report = true
	}
	return
}

func main() {
	err := checkSync()
	if err != nil {
		os.Exit(1)
	}
}

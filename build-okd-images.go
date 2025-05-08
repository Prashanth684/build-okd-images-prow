package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
	"strconv"
)

type ReleaseInfo struct {
	References struct {
		Spec struct {
			Tags []struct {
				Name string `json:"name"`
			} `json:"tags"`
		} `json:"spec"`
	} `json:"references"`
}

type ImageInfo struct {
	Config struct {
		History []struct {
			Created string `json:"created"`
		} `json:"history"`
		ContainerConfig struct {
			Labels map[string]string `json:"Labels"`
		} `json:"container_config"`
	} `json:"config"`
}

func runCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error: %v\n%s", err, stderr.String())
	}
	return out.Bytes(), nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <release-image> <trigger-job-threshold> <optional: release-branch>")
		os.Exit(1)
	}
	releaseImage := os.Args[1]
	releaseBranch := ""
	jobThreshold, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Error converting threshold to number:", err)
		return
	}

	//release branch is optional argument
	if len(os.Args) > 3{
		releaseBranch = os.Args[3]
	}

	// Get release info
	releaseJSON, err := runCommand("oc", "adm", "release", "info", releaseImage, "-o", "json")
	if err != nil {
		fmt.Printf("Failed to get release info: %v\n", err)
		os.Exit(1)
	}

	var releaseInfo ReleaseInfo
	if err := json.Unmarshal(releaseJSON, &releaseInfo); err != nil {
		fmt.Printf("Failed to parse release JSON: %v\n", err)
		os.Exit(1)
	}

	for _, tag := range releaseInfo.References.Spec.Tags {
		component := tag.Name
		fmt.Printf("Component: %s ", component)

		imageRefRaw, err := runCommand("oc", "adm", "release", "info", releaseImage, "--image-for="+component)
		if err != nil {
			fmt.Printf("  Could not resolve image: %v\n", err)
			continue
		}
		imageRef := strings.TrimSpace(string(imageRefRaw))

		imageInfoRaw, err := runCommand("oc", "image", "info", imageRef, "-o", "json")
		if err != nil {
			fmt.Printf("  Failed to get image info: %v\n", err)
			continue
		}

		var imageInfo ImageInfo
		if err := json.Unmarshal(imageInfoRaw, &imageInfo); err != nil {
			fmt.Printf("  Failed to parse image info: %v\n", err)
			continue
		}

		var createdTimes []time.Time
		for _, h := range imageInfo.Config.History {
			if h.Created != "" {
				t, err := time.Parse(time.RFC3339, h.Created)
				if err == nil {
					createdTimes = append(createdTimes, t)
				}
			}
		}

		if len(createdTimes) == 0 {
			fmt.Println("  No valid created timestamps found")
			continue
		}

		sort.Slice(createdTimes, func(i, j int) bool {
			return createdTimes[i].Before(createdTimes[j])
		})

		latest := createdTimes[len(createdTimes)-1]
		now := time.Now().UTC()
		diff := now.Sub(latest)

		var timeStr string
		days := 0
		if diff < time.Minute {
			timeStr = fmt.Sprintf("%ds", int(diff.Seconds()))
		} else if diff < time.Hour {
			timeStr = fmt.Sprintf("%dm", int(diff.Minutes()))
		} else if diff < 24*time.Hour {
			timeStr = fmt.Sprintf("%dh", int(diff.Hours()))
		} else {
			days = int(diff.Hours()) / 24
			hours := int(diff.Hours()) % 24
			timeStr = fmt.Sprintf("%dd %dh", days, hours)
		}
		
		labels := imageInfo.Config.ContainerConfig.Labels
		buildBranch := labels["io.openshift.build.commit.ref"]
		if releaseBranch != "" {
			buildBranch = releaseBranch
		}
		fmt.Printf("created: %s ago %s %s\n", timeStr, labels["vcs-url"], buildBranch)
		if days >= jobThreshold && labels["vcs-url"]!= "" && buildBranch != "" {
			time.Sleep(10 * time.Second)
			fmt.Printf("Running ./os-postsubmit.sh %s %s\n", labels["vcs-url"], buildBranch)
			err := runPostSubmit(labels["vcs-url"], buildBranch)
			if err != nil && strings.Contains(err.Error(), " ❌ Failed to retrieve base SHA for branch") && (buildBranch == "master"){
				fmt.Printf("Retrying with 'main' as fallback\n")
				err = runPostSubmit(labels["vcs-url"], "main")
				if err != nil {
					fmt.Printf(" ❌ Retry also failed: %v\n", err)
				}
			}
			fmt.Println("==============================================================================================================================")
		}
	}
}

func runPostSubmit(vcsURL, ref string) error {
	cmd := exec.Command("./os-postsubmit.sh", vcsURL, ref)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		combined := strings.TrimSpace(outBuf.String() + "\n" + errBuf.String())
		return fmt.Errorf("script error: %s", combined)
	}
	return nil
}

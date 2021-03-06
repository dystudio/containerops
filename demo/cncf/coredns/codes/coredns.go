/*
Copyright 2016 - 2017 Huawei Technologies Co., Ltd. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// Parse CO_DATA value, and return CoreDNS repository URI and action (build/test/release).
func parseEnv(env string) (uri, action, release string, err error) {
	files := strings.Fields(env)
	if len(files) == 0 {
		return "", "", "", fmt.Errorf("CO_DATA value is null\n")
	}

	for _, v := range files {
		s := strings.Split(v, "=")
		key, value := s[0], s[1]

		switch key {
		case "coredns":
			uri = value
		case "action":
			action = value
		case "release":
			release = value
		default:
			fmt.Fprintf(os.Stdout, "[COUT] Unknown Parameter: [%s]\n", s)
		}
	}

	return uri, action, release, nil
}

// Git clone the CoreDNS repository, and process will redirect to system stdout.
func gitClone(repo, dest string) error {
	cmd := exec.Command("git", "clone", repo, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[COUT] Git clone error: %s\n", err.Error())
		fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")
		os.Exit(1)
	}

	return nil
}

// Execute `make coredns` in the CoreDNS folder.
func corednsBuild() error {
	cmd := exec.Command("make", "coredns")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[COUT] Make build error: %s\n", err.Error())
		fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")

		return err
	}

	return nil
}

// Execute `make test` in the CoreDNS folder.
func corednsTest() error {
	cmd := exec.Command("make", "test")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[COUT] Make test error: %s\n", err.Error())
		fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")

		return err
	}

	return nil
}

// Execute `make build` in the CoreDNS folder, upload the coredns binary file to the artifact repository.
func corednsRelease(repo, release string) error {
	if err := corednsBuild(); err != nil {
		return err
	}

	filePath := fmt.Sprintf("%s/coredns", repo)

	if f, err := os.Open(filePath); err != nil {
		fmt.Fprintf(os.Stderr, "[COUT] Read coredns binary file error: %s\n", err.Error())
		fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")

		return err
	} else {
		defer f.Close()

		// Pattern: <domain>/<namespace>/<repository>/<tag>
		// hub.opshub.sh/containerops/cncf-demo/demo

		domain := strings.Split(release, "/")[0]
		namespace := strings.Split(release, "/")[1]
		repository := strings.Split(release, "/")[2]
		tag := strings.Split(release, "/")[3]

		if req, err := http.NewRequest(http.MethodPut,
			fmt.Sprintf("https://%s/binary/v1/%s/%s/binary/%s/%s",
				domain, namespace, repository, filepath.Base(filePath), tag), f); err != nil {

			fmt.Fprintf(os.Stderr, "[COUT] Upload coredns binary file error: %s\n", err.Error())
			fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")

			return err
		} else {
			req.Header.Set("Content-Type", "text/plain")

			client := &http.Client{}
			if resp, err := client.Do(req); err != nil {
				fmt.Fprintf(os.Stderr, "[COUT] Upload coredns binary file error: %s\n", err.Error())
				fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")

				return err
			} else {
				defer resp.Body.Close()

				switch resp.StatusCode {
				case http.StatusOK:
					uri := fmt.Sprintf("https://%s/binary/v1/%s/%s/binary/%s/%s\n",
						domain, namespace, repository, filepath.Base(filePath), tag)
					fmt.Fprintf(os.Stdout, "[COUT] CO_COREDNS_URI = %s", uri)

					return nil
				case http.StatusBadRequest:
					fmt.Fprint(os.Stderr, "[COUT] Upload coredns binary file, service return 400 error.")
					fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")

					return fmt.Errorf("Binary upload failed.")
				case http.StatusUnauthorized:
					fmt.Fprint(os.Stderr, "[COUT] Upload coredns binary file, service return 401 error.")
					fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")

					return fmt.Errorf("Action unauthorized.")
				default:
					fmt.Fprint(os.Stderr, "[COUT] Upload coredns binary file, service return unknown error.")
					fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")

					return fmt.Errorf("Unknown error.")
				}
			}
		}
	}

	return nil
}

func main() {
	//Get the CO_DATA from environment parameter "CO_DATA"
	data := os.Getenv("CO_DATA")
	if len(data) == 0 {
		fmt.Fprintf(os.Stderr, "[COUT] %s\n", "The CO_DATA value is null.")
		fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "The CO_DATA value is null.")
		os.Exit(1)
	}

	if corednsRepo, action, release, err := parseEnv(data); err != nil {
		fmt.Fprintf(os.Stderr, "[COUT] Parse the CO_DATA error: %s\n", err.Error())
		fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")
		os.Exit(1)
	} else {
		//Create the base path within GOPATH.
		basePath := path.Join(os.Getenv("GOPATH"), "src", "github.com", "coredns", "coredns")
		os.MkdirAll(basePath, os.ModePerm)

		//Clone the git repository
		if err := gitClone(corednsRepo, basePath); err != nil {
			fmt.Fprintf(os.Stderr, "[COUT] Clone the coredns repository error: %s\n", err.Error())
			fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")
			os.Exit(1)
		}

		//Execute action
		switch action {
		case "build":

			if err := corednsBuild(); err != nil {
				os.Exit(1)
			}

		case "test":

			if err := corednsTest(); err != nil {
				os.Exit(1)
			}

		case "release":

			if err := corednsRelease(basePath, release); err != nil {
				os.Exit(1)
			}

		default:
			fmt.Fprintf(os.Stderr, "[COUT] %s\n", "Unknown action, the component only support build, test and release action.")
			fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "false")
			os.Exit(1)
		}

	}

	//Print result
	fmt.Fprintf(os.Stdout, "[COUT] CO_RESULT = %s\n", "true")
	os.Exit(0)
}

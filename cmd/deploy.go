// Copyright © 2018 Yusuke KUOKA
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"github.com/mumoshu/division/api"
	"github.com/mumoshu/division/dynamodb"
	"github.com/spf13/cobra"
	"os"
	"reflect"
	"strings"
	"time"
)

type DeployOptions struct {
	Project string
	Ref     string
	App     string
	Watch   bool
}

var deployOpts DeployOptions

func init() {
	deployOpts = DeployOptions{}
}

type installResult struct {
	err error
}

func NewCmdDeploy() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys application(s) defined in PROJECT",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			db, err := dynamodb.NewDB(globalOpts.Config, globalOpts.Namespace)
			if err != nil {
				return err
			}

			logs, err := dynamodb.NewLogs(globalOpts.Config, globalOpts.Namespace)
			if err != nil {
				return err
			}

			targetedAppName := deployOpts.App
			targetedProjectName := deployOpts.Project

			knownProjects, err := db.GetSync("project", targetedProjectName, []string{})
			if err != nil {
				return err
			}
			targetedProjects := map[string]*api.Resource{}
			for _, proj := range knownProjects {
				projName := proj.NameHashKey
				if targetedProjectName != "" && projName != targetedProjectName {
					continue
				}
				targetedProjects[projName] = proj
			}

			targetedProjectNames := make([]string, len(targetedProjects))
			i := 0
			for _, p := range targetedProjects {
				targetedProjectNames[i] = fmt.Sprintf("  * %s", p.NameHashKey)
				i++
			}
			fmt.Fprintf(os.Stderr, `%d projects found:

%s

`, len(targetedProjectNames), strings.Join(targetedProjectNames, "\n"))

			knownApps, err := db.GetSync("application", "", []string{})
			if err != nil {
				return err
			}
			targetedApps := map[string]*api.Resource{}
			for _, app := range knownApps {
				appName := app.NameHashKey
				if targetedAppName != "" && appName != targetedAppName {
					continue
				}
				appProject := app.Spec["project"].(string)
				if _, ok := targetedProjects[appProject]; !ok {
					continue
				}
				targetedApps[appName] = app
			}

			{
				targetedAppNames := make([]string, len(targetedApps))
				i := 0
				for _, app := range targetedApps {
					targetedAppNames[i] = fmt.Sprintf("  * %s", app.NameHashKey)
					i++
				}
				fmt.Fprintf(os.Stderr, `%d applications found:

%s

`, len(targetedAppNames), strings.Join(targetedAppNames, "\n"))
			}

			for appName, _ := range targetedApps {
				//sha1, err := fetchSha1ForRef(deployOpts.Ref)
				//if err != nil {
				//	return err
				//}
				targetedSha1 := deployOpts.Ref

				// e.g. "myproj_myapp1"
				deployName := appName
				var deploy *api.Resource
				deploys, err := db.GetSync("deployment", deployName, []string{})
				switch e := err.(type) {
				case *dynamodb.ErrResourceNotFound:
					newDeploy := &api.Resource{
						NameHashKey: deployName,
						Kind:        "Deployment",
						Metadata: api.Metadata{
							Name: deployName,
						},
						Spec: map[string]interface{}{
							"project": targetedProjectName,
							"app":     appName,
							"sha1":    targetedSha1,
						},
					}
					err := db.Apply(newDeploy)
					if err != nil {
						return err
					}
					deploy = newDeploy
				case error:
					return e
				case nil:
					deploy = deploys[0]
				default:
					panic(fmt.Errorf("unexpected type of error %v: %v", reflect.TypeOf(e), e))
				}

				if deploy.Spec["sha1"].(string) == targetedSha1 {
					// already deployed
					break
				}

				deploy.Spec["sha1"] = targetedSha1
				err = db.Apply(deploy)
				if err != nil {
					return err
				}
			}

			targetedClusters, err := db.GetSync("cluster", "", []string{})
			if err != nil {
				return err
			}

			// wait for completion
			for _, app := range targetedApps {
				appName := app.NameHashKey

				for _, cluster := range targetedClusters {
					clusterName := cluster.NameHashKey

					// TODO record start time
				L:
					for {
						// TODO timeout
						deployId := appName
						deploys, err := db.GetSync("deployment", deployId, []string{})
						if err != nil {
							return err
						}
						if len(deploys) == 0 {
							continue
						}
						deploy := deploys[0]

						//sha1, err := fetchSha1ForRef(deployOpts.Ref)
						//if err != nil {
						//	return err
						//}
						sha1 := deploy.Spec["sha1"]

						installName := fmt.Sprintf("%s-%s-%s", appName, clusterName, sha1)
						installs, err := db.GetSync("install", installName, []string{})
						switch e := err.(type) {
						case *dynamodb.ErrResourceNotFound:
							fmt.Fprintf(os.Stderr, "waiting for install of %s to start\n", installName)
							time.Sleep(5 * time.Second)
						case nil:
							// stream logs until end
							install := installs[0]
							installName := install.NameHashKey

							installResults := make(chan *installResult)

							go func() {
								for {
									time.Sleep(5 * time.Second)
									is, err := db.GetSync("install", installName, []string{})
									if err != nil {
										fmt.Fprintf(os.Stderr, "error while polling install status. ignoring: %v\n", err)
									}
									if is != nil {
										ins := is[0]
										installPhase := ins.Spec["phase"]
										fmt.Fprintf(os.Stderr, "install %s: phase=%s\n", installName, installPhase)
										if installPhase == "completed" {
											installResults <- &installResult{}
											return
										} else if installPhase == "failed" {
											installResults <- &installResult{err: fmt.Errorf("install %s has failed", installName)}
											return
										}
									}
								}
							}()

							logMsgs, logErrs := logs.Read("install", installName, 0, true)

							for logMsgs != nil || logErrs != nil || installResults != nil {
								select {
								case r, ok := <-installResults:
									if ok {
										logMsgs = nil
										logErrs = nil
										if r.err != nil {
											return err
										}
										break L
									}
								case msg, ok := <-logMsgs:
									if ok {
										fmt.Fprintf(os.Stderr, "%s: %s", installName, msg)
									} else {
										logMsgs = nil
									}
								case err, ok := <-logErrs:
									logErrs = nil
									switch err.(type) {
									case *dynamodb.ErrLogsNotFound:
										fmt.Fprintf(os.Stderr, "waiting for logs of %s to flow\n", installName)
										time.Sleep(5 * time.Second)
										continue L
									case nil:
										if ok {
											panic("[bug] expected error, but it was nil")
										}
									default:
										return err
									}
								}
							}
						case error:
							return err
						default:
							panic(fmt.Errorf("unexpected type of error %v: %v", reflect.TypeOf(e), e))
						}
					}
				}
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&deployOpts.Project, "project", "", "Name of the project to be deployed. Defaults to all")
	flags.StringVar(&deployOpts.App, "app", "", "Name of the app to be deployed. Defaults to all")
	flags.StringVarP(&deployOpts.Ref, "ref", "", "master", "Commit SHA1 or branch name to be deployed. Defaults to master")
	flags.BoolVar(&deployOpts.Watch, "wait", false, "After listing/getting the requested object, watch for changes. Uninitialized objects are excluded if no object name is provided.")
	cmd.MarkFlagRequired("ref")

	return cmd
}

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
	"github.com/Azure/brigade/pkg/script"
	"github.com/mumoshu/division/api"
	"github.com/mumoshu/division/dynamodb"
	"github.com/spf13/cobra"
	"io"
	"k8s.io/client-go/kubernetes"
	"os"
	"strings"
)

type GatewayOptions struct {
	Cluster string
	Project string
}

var gatewayOpts GatewayOptions

func NewCmdGateway() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "brigade gateway that exec command according to div resource changes like new deployment",
		Args:  cobra.RangeArgs(0, 0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			// Namespace corresponds to environment, that differentiates e.g. production vs staging vs development
			c, err := kubeClient()
			if err != nil {
				return err
			}

			env := globalOpts.Namespace
			db, err := dynamodb.NewDB(globalOpts.Config, env)
			if err != nil {
				return err
			}

			logs, err := dynamodb.NewLogs(globalOpts.Config, env)
			if err != nil {
				return err
			}

			clusterName := gatewayOpts.Cluster
			targetedProjectName := gatewayOpts.Project

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

			if targetedProjectName != "" {
				noProject := true
				for _, p := range knownProjects {
					noProject = noProject && p.NameHashKey != targetedProjectName
				}
				if noProject {
					return fmt.Errorf("no project named \"%s\" found", targetedProjectName)
				}
			}

			knownApps, err := db.GetSync("application", "", []string{})
			if err != nil {
				return err
			}
			targetedApps := map[string]*api.Resource{}
			for _, app := range knownApps {
				appName := app.NameHashKey
				if deployOpts.App != "" && appName != deployOpts.App {
					continue
				}
				if app.Spec["project"] == nil {
					panic(fmt.Sprintf("application \"%s\" has no \"project\" field", appName))
				}
				appProject := app.Spec["project"].(string)
				if _, ok := targetedProjects[appProject]; !ok {
					continue
				}
				targetedApps[appName] = app
			}

			g := &gateway{
				targetedProjects,
				targetedApps,
				clusterName,
				env,
				c,
				logs,
				db,
			}

			newInstalls := make(chan *api.Resource, 1)
			deploys, deployErrs := db.GetAsync("deployment", "", []string{}, true)
			releases, releaseErrs := db.GetAsync("release", "", []string{}, true)
			installs, installErrs := db.GetAsync("install", "", []string{}, true)
			for {
				select {
				case d := <-deploys:
					if d.Spec["project"] == nil {
						panic(fmt.Sprintf("deployment \"%s\" has no \"project\" field", d.NameHashKey))
					}
					if d.Spec["app"] == nil {
						panic(fmt.Sprintf("deployment \"%s\" has no \"app\" field", d.NameHashKey))
					}
					deployProj := d.Spec["project"].(string)
					deployApp := d.Spec["app"].(string)
					_, hasTargetedProj := targetedProjects[deployProj]
					_, hasTargetedApp := targetedApps[deployApp]
					if hasTargetedProj && hasTargetedApp {
						releaseName := fmt.Sprintf("%s-%s", d.NameHashKey, clusterName)
						sha1 := d.Spec["sha1"]

						rs, e := db.GetSync("release", releaseName, []string{})
						if e != nil {
							switch e.(type) {
							case *dynamodb.ErrResourceNotFound:

							default:
								panic(e)
							}
						}
						if len(rs) == 0 || rs[0].Spec["sha1"] != sha1 {
							newRelease := &api.Resource{
								NameHashKey: releaseName,
								Metadata: api.Metadata{
									Name: releaseName,
								},
								Kind: "Release",
								Spec: map[string]interface{}{
									"project": deployProj,
									"app":     deployApp,
									"sha1":    sha1,
									"cluster": clusterName,
								},
							}
							err := db.Apply(newRelease)
							if err != nil {
								panic(err)
							}
						}
					}
				case r := <-releases:
					relProj := r.Spec["project"].(string)
					relApp := r.Spec["app"].(string)
					relCluster := r.Spec["cluster"].(string)
					_, hasTargetedProj := targetedProjects[relProj]
					_, hasTargetedApp := targetedApps[relApp]
					hasTargetedCluster := relCluster == clusterName
					if hasTargetedProj && hasTargetedApp && hasTargetedCluster {
						sha1 := r.Spec["sha1"]
						installName := fmt.Sprintf("%s-%s", r.NameHashKey, sha1)

						is, e := db.GetSync("install", installName, []string{})
						if e != nil {
							switch e.(type) {
							case *dynamodb.ErrResourceNotFound:

							default:
								panic(e)
							}
						}
						if len(is) == 0 {
							newInstall := &api.Resource{
								NameHashKey: installName,
								Metadata: api.Metadata{
									Name: installName,
								},
								Kind: "Install",
								Spec: map[string]interface{}{
									"project": relProj,
									"app":     relApp,
									"sha1":    sha1,
									"cluster": clusterName,
									"phase":   "pending",
								},
							}
							err := db.Apply(newInstall)
							if err != nil {
								panic(err)
							}
							// newInstall is propagated to
							newInstalls <- newInstall
						} else {
							ins := is[0]
							fmt.Fprintf(os.Stderr, "install \"%s\" is already %s\n. no need to trigger another install. skipping...", ins.NameHashKey, ins.Spec["phase"])
						}
					}
				case i, ok := <-installs:
					if !ok {
						installs = nil
						panic("TODO: install stream stopped unexpectedly. implement automatic retry")
					}
					g.handleInstall(i)
				case i, ok := <-newInstalls:
					if !ok {
						installs = nil
						panic("TODO: install stream stopped unexpectedly. implement automatic retry")
					}
					g.handleInstall(i)
				case e := <-installErrs:
					panic(e)
				case e := <-deployErrs:
					panic(e)
				case e := <-releaseErrs:
					panic(e)
				}
			}

			// <namespace=production>
			// + (state) myproj-app1 (application w/ id=myproj-app1 name=app1 project=myproj, app1 in project myproj)
			// | + upsert + version : deploy myproj/app1 --ref v1.0.0
			// + (state) myproj-app1 (deployment w/ id=app1 version=v1.0.0, upsert a deployment identified by id=app1)
			//    | + upsert + cluster by in-cluster server
			//    +--+ (state) myproj-app1-prod1 (release w/ id=app1-prod1 version=v1.0.0, list-watch all production deployments like app1, and release if missing)
			//    |     | created
			//    |     +--+ (history) myproj-app1-prod1-v1.0.0 (install w/ id=app1-prod1-v1.0.0, list-watch all prod1 releases like app1-prod1, and install if missing)
			//    |
			//    +--+ myproj-app1-prod2 (release w/ id=app1-prod2 version=v1.0.0)
			//          |
			//          +--+ myproj-app1-prod2-v1.0.0 (install w/ id=app1-prod2-v1.0.0, list-watch all prod2 releases like app1-prod2, and install if missing)
			//
			// + production-last-releases(environment-state)     <- updated by release
			//
			// + production-prod1-last-installs(cluster-state)   <- created by environment-last-installs, updated by installs, holds all the list of installs for prod1, used for `sync --prune`.
			return nil
		},
	}

	options := cmd.Flags()
	options.StringVar(&gatewayOpts.Cluster, "cluster", "", "Unique name of the cluster on which this gateway is running")
	options.StringVar(&gatewayOpts.Project, "project", "", "Unique name of the project which this gateway watches")
	cmd.MarkFlagRequired("cluster")

	return cmd

}

type gateway struct {
	targetedProjects map[string]*api.Resource
	targetedApps     map[string]*api.Resource
	clusterName      string
	env              string
	c                *kubernetes.Clientset
	logs             *dynamodb.LogStore
	db               dynamodb.Store
}

func (g *gateway) handleInstall(i *api.Resource) error {
	targetedProjects := g.targetedProjects
	targetedApps := g.targetedApps
	clusterName := g.clusterName
	env := g.env
	c := g.c
	logs := g.logs
	db := g.db

	fmt.Fprintf(os.Stderr, "detected install: %v\n", i.Spec)
	insProj := i.Spec["project"].(string)
	insApp := i.Spec["app"].(string)
	insCluster := i.Spec["cluster"].(string)
	_, hasTargetedProj := targetedProjects[insProj]
	_, hasTargetedApp := targetedApps[insApp]
	hasTargetedCluster := insCluster == clusterName
	if hasTargetedProj && hasTargetedApp && hasTargetedCluster {
		insPhase := i.Spec["phase"]
		switch insPhase {
		case "pending":
			//set, _ := i.Spec["set"].(string)
			set := ""
			sha1 := i.Spec["sha1"].(string)
			// dedup deployment to deployment_status by `app` and `cluster`
			//statusKey := fmt.Sprintf("%s-%s-%s", env, cluster, app)

			label := "-l=name=" + insApp
			envFlag := "--environment=" + env
			setFlag := "--set=ref=" + sha1
			if set != "" {
				setFlag += "," + set
			}
			payload := []byte(fmt.Sprintf(`
{"command": ["echo", "helmfile", "--log-level=debug", "-f=helmfile.yaml", ""%s", "%s", "%s", "apply", "--auto-approve"]}
`, envFlag, label, setFlag))
			s := []byte(`
const { events, Job } = require("brigadier")

events.on("div:install", (e, p) => {
  console.log({"event": e, "payload": p})

  var ep = JSON.parse(e.payload)

  var job = new Job("helmfile-apply", "alpine:3.4")
  job.tasks = [
    "echo Hello",
    "echo World",
    ep.command.join(" ")
  ]

  job.run()
})
`)
			persistentLogsWriter, err := logs.Writer("install", i.NameHashKey)
			if err != nil {
				panic(err)
			}

			mul := io.MultiWriter(persistentLogsWriter, os.Stderr)

			r, err := script.NewDelegatedRunner(c, "default")
			if err != nil {
				panic(err)
			}
			r.ScriptLogDestination = mul
			r.RunnerLogDestination = mul

			i.Spec["phase"] = "running"
			if err := db.Apply(i); err != nil {
				panic(err)
			}

			var postPhase string
			err = r.SendScript(insProj, s, "div:install", "", sha1, payload, "")
			if err != nil {
				fmt.Fprintf(os.Stderr, "brigade failed: %v\n", err)
				postPhase = "failed"
			} else {
				postPhase = "completed"
			}

			i.Spec["phase"] = postPhase
			if err := db.Apply(i); err != nil {
				panic(err)
			}
		case "failed":
			// TODO retry
			fmt.Fprintf(os.Stderr, "TODO: retrying failed install: %s\n", i.NameHashKey)
		case "running":
			// TODO Mark it failed on timeout
			fmt.Fprintf(os.Stderr, "TODO: install \"%s\" is already running. Remove it and redeploy in order to rerun\n", i.NameHashKey)
		case "completed":
			fmt.Fprintf(os.Stderr, "install \"%s\" is already completed. skipping\n", i.NameHashKey)
		default:
			panic(fmt.Errorf("unexpected phase for \"%s\": %s", i.NameHashKey, insPhase))
		}
	}
	return nil
}

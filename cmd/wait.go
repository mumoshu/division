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
	"github.com/mumoshu/division/dynamodb"
	"github.com/spf13/cobra"
	"time"
)

type WaitOptions struct {
	Logs    bool
	Timeout time.Duration
}

var waitOpts WaitOptions

func init() {
	waitOpts = WaitOptions{}
}

func NewCmdWait() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wait RESOURCE NAME QUERY",
		Short: "wait until the resource meets the criteria",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			db, err := dynamodb.NewDB(globalOpts.Config, globalOpts.Namespace)
			if err != nil {
				return err
			}

			return db.Wait(args[0], args[1], args[2], globalOpts.Output, waitOpts.Timeout, waitOpts.Logs)
		},
	}

	flags := cmd.Flags()
	flags.DurationVar(&waitOpts.Timeout, "timeout", 0, "Stop after a duration like 5s, 2m, or 3h. Defaults to 0s=forever.")
	flags.BoolVar(&waitOpts.Logs, "logs", false, "Specify if the logs should be streamed.")

	return cmd

}

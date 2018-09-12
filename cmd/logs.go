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

type LogsReadOptions struct {
	Follow bool
	Since  time.Duration
}

type LogsWriteOptions struct {
	File string
}

var logsReadOpts LogsReadOptions
var logsWriteOpts LogsWriteOptions

func init() {
	logsReadOpts = LogsReadOptions{}
	logsWriteOpts = LogsWriteOptions{}
}

func NewCmdLogs() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [read|write] RESOURCE NAME",
		Short: "read or write logs",
	}

	readCmd := &cobra.Command{
		Use:   "read RESOURCE NAME",
		Short: "read logs",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			logs, err := dynamodb.NewLogs(globalOpts.Config, globalOpts.Namespace)
			if err != nil {
				return err
			}

			return logs.ReadPrint(args[0], args[1], logsReadOpts.Since, logsReadOpts.Follow)
		},
	}
	rflags := readCmd.Flags()
	rflags.DurationVar(&logsReadOpts.Since, "since", 0, "Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs.")
	rflags.BoolVarP(&logsReadOpts.Follow, "follow", "f", false, "Specify if the logs should be streamed.")

	writeCmd := &cobra.Command{
		Use:   "write RESOURCE NAME",
		Short: "write logs",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			logs, err := dynamodb.NewLogs(globalOpts.Config, globalOpts.Namespace)
			if err != nil {
				return err
			}

			return logs.WriteFile(args[0], args[1], logsWriteOpts.File)
		},
	}
	wflags := writeCmd.Flags()
	wflags.StringVarP(&logsWriteOpts.File, "file", "f", "-", "Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs.")

	deleteCmd := &cobra.Command{
		Use:   "delete RESOURCE NAME",
		Short: "delete logs",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			logs, err := dynamodb.NewLogs(globalOpts.Config, globalOpts.Namespace)
			if err != nil {
				return err
			}

			return logs.Delete(args[0], args[1])
		},
	}

	cmd.AddCommand(readCmd, writeCmd, deleteCmd)

	return cmd
}

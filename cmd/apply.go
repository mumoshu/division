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
)

type ApplyOptions struct {
	File string
}

var applyOpts ApplyOptions

func NewCmdApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Put record(s)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			db, err := dynamodb.NewDB(globalOpts.Config, globalOpts.Namespace)
			if err != nil {
				return err
			}
			return db.ApplyFile(applyOpts.File)
		},
	}

	options := cmd.Flags()
	options.StringVarP(&applyOpts.File, "file", "f", "", "Path to input File (optional; default is stdin)")
	cmd.MarkFlagRequired("file")

	return cmd

}

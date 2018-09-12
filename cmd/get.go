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
	"github.com/mumoshu/division/dynamodb"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

type GetOptions struct {
	Selectors []string
	Watch     bool
}

var getOpts GetOptions

func init() {
	getOpts = GetOptions{
		Selectors: []string{},
	}
}

func NewCmdGet() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get RESOURCE [NAME]",
		Short: "Displays one or more resources",
		Args:  cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			db, err := dynamodb.NewDB(globalOpts.Config, globalOpts.Namespace)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				crds, err := db.GetCRDs()
				if err != nil {
					return err
				}
				names := make([]string, len(crds))
				for i, crd := range crds {
					names[i] = fmt.Sprintf("  * %s", crd.Metadata.Name)
				}
				fmt.Fprintf(os.Stderr, `You must specify the type of resource to get. Valid resource types include:

%s
`, strings.Join(names, "\n"))
				os.Exit(1)
			} else {
				var name string
				if len(args) > 1 {
					name = args[1]
				} else {
					name = ""
				}
				resource := args[0]
				err = db.GetPrint(resource, name, getOpts.Selectors, globalOpts.Output, getOpts.Watch)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVarP(&getOpts.Selectors, "selector", "l", []string{}, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	flags.BoolVarP(&getOpts.Watch, "watch", "w", false, "After listing/getting the requested object, watch for changes. Uninitialized objects are excluded if no object name is provided.")

	return cmd

}

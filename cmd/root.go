// Copyright © 2018 Yusuke KUOKA
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

type GlobalOptions struct {
	Config    string `validate:"config"`
	Namespace string `validate:"required"`
	Output    string `validate:"required"`
	Name      string
}

var globalOpts GlobalOptions

// RootCmd represents the base command when called without any subcommands
func NewCmdRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "div",
		Short: "custom resources database",
		Long:  `access custom resources stored in various datastores via simple cli`,
	}
	cobra.OnInitialize(initConfig)

	cmd.PersistentFlags().StringVar(&profile, "profile", "default", "AWS profile")
	cmd.PersistentFlags().StringVar(&region, "region", "ap-northeast-1", "AWS region")

	viper.BindPFlag("profile", cmd.PersistentFlags().Lookup("profile"))
	viper.BindPFlag("region", cmd.PersistentFlags().Lookup("region"))

	flags := cmd.PersistentFlags()
	flags.StringVarP(&globalOpts.Namespace, "namespace", "n", "default", "Namespace to restrict fetched resources")
	flags.StringVarP(&globalOpts.Config, "config", "c", "div.yaml", "Config file containing custom resource definitions")
	flags.StringVarP(&globalOpts.Output, "output", "o", "json", "Output format. One of: text|json|yaml")

	cmd.AddCommand(NewCmdGet())
	cmd.AddCommand(NewCmdApply())
	cmd.AddCommand(NewCmdDel())
	cmd.AddCommand(NewCmdWait())
	cmd.AddCommand(NewCmdLogs())
	cmd.AddCommand(NewCmdDel())
	cmd.AddCommand(NewCmdGateway())
	cmd.AddCommand(NewCmdDeploy())

	return cmd
}

var (
	profile string
	region  string
)

func init() {
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".gody" (without extension).
		viper.SetConfigName(".div")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

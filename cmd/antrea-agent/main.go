// Copyright 2019 Antrea Authors
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

// Package main under directory cmd parses and validates user input,
// instantiates and initializes objects imported from pkg, and runs
// the process.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"antrea.io/antrea/pkg/log"
	"antrea.io/antrea/pkg/version"
)

func main() {
	fmt.Println("hello")
	command := newAgentCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

func newAgentCommand() *cobra.Command {
	opts := newOptions()

	cmd := &cobra.Command{
		Use:  "antrea-agent",
		Long: "The Antrea agent runs on each node.",
		Run: func(cmd *cobra.Command, args []string) {
			log.InitLogs(cmd.Flags())
			defer log.FlushLogs()
			if err := opts.complete(args); err != nil {
				klog.Fatalf("Failed to complete: %v", err)
			}
			if err := opts.validate(args); err != nil {
				klog.Fatalf("Failed to validate: %v", err)
			}
			if err := run(opts); err != nil {
				klog.Fatalf("Error running agent: %v", err)
			}
		},
		Version: version.GetFullVersionWithRuntimeInfo(),
	}

	flags := cmd.Flags()
	opts.addFlags(flags)
	log.AddFlags(flags)
	return cmd
}

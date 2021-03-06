// Copyright © 2017 Domen Ipavec <domen@ipavec.net>
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
	"log"
	"os"

	"github.com/matematik7/didcj/compile"
	"github.com/matematik7/didcj/config"
	"github.com/matematik7/didcj/daemon"
	"github.com/matematik7/didcj/generate"
	"github.com/matematik7/didcj/inventory"
	"github.com/matematik7/didcj/runner"
	"github.com/matematik7/didcj/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var RemoteNodes int

// remoteCmd represents the remote command
var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Get()
		if err != nil {
			log.Fatal(err)
		}

		if RemoteNodes > 0 {
			cfg.NumberOfNodes = RemoteNodes
		}

		inv, err := inventory.Init(viper.GetString("inventory"))
		if err != nil {
			log.Fatalf("could not init inventory: %v", err)
		}
		servers, err := inv.Get()
		if err != nil {
			log.Fatalf("could not get inventory: %v", err)
		}

		if len(servers) < cfg.NumberOfNodes {
			log.Fatal("not enough running servers")
		}

		cfg.Servers = servers[:cfg.NumberOfNodes]

		err = generate.MessageH(cfg.NumberOfNodes)
		if err != nil {
			log.Fatalf("could not generate message.h: %v", err)
		}

		file, err := utils.FindFileBasename("cpp", "dcj")
		if err != nil {
			log.Fatalf("could not find file cpp: %v", err)
		}

		utils.GetHFileFromDownloads(file)

		log.Println("Compiling ...")
		err = compile.Transpile(file)
		if err != nil {
			log.Fatalf("could not transpile: %v", err)
		}
		err = compile.Compile(file)
		if err != nil {
			log.Fatalf("could not compile: %v", err)
		}

		log.Println("Removing message.h")
		err = os.Remove("message.h")
		if err != nil {
			log.Fatalf("could not remove message.h: %v", err)
		}

		log.Println("Distributing ...")
		fileApp := file + ".app"
		err = utils.Upload(fileApp, fileApp, cfg.Servers...)
		if err != nil {
			log.Fatalf("could not upload %s: %v", fileApp, err)
		}

		log.Println("Running...")
		report := &daemon.RunReport{}
		err = utils.Send(cfg.Servers[0], "/run/", cfg, report)
		if err != nil {
			log.Fatalf("could not run: %v", err)
		}

		maxTime := int64(0)
		maxMemory := 0

		onlyOneNodeMessages := true
		oneNodeMessages := []string{}

		for _, report := range report.Reports {
			if report.RunTime > maxTime {
				maxTime = report.RunTime
			}
			if report.MaxMemory > maxMemory {
				maxMemory = report.MaxMemory
			}
			log.Printf(
				"Node %s (msgs: %d, largest: %s, time: %s, memory: %s):",
				report.Name,
				report.SendCount,
				utils.FormatSize(report.LargestMsg),
				utils.FormatDuration(report.RunTime),
				utils.FormatSize(report.MaxMemory),
			)
			if len(report.Messages) > 0 {
				for _, message := range report.Messages {
					log.Println(message)
				}

				if len(oneNodeMessages) == 0 {
					oneNodeMessages = report.Messages
				} else {
					onlyOneNodeMessages = false
				}
			}
		}

		if report.Status == runner.DONE {
			log.Printf("Run successful in %s with %s memory!",
				utils.FormatDuration(maxTime),
				utils.FormatSize(maxMemory),
			)
			if onlyOneNodeMessages {
				log.Println("Output from only one node:")
				for _, message := range oneNodeMessages {
					log.Println(message)
				}
			}
		} else {
			log.Printf("Run failed in %s with %s memory!",
				utils.FormatDuration(maxTime),
				utils.FormatSize(maxMemory),
			)
		}
	},
}

func init() {
	RootCmd.AddCommand(remoteCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// remoteCmd.PersistentFlags().String("foo", "", "A help for foo")

	remoteCmd.PersistentFlags().String("inventory", "docker", "Which node inventory to use (docker, google)")
	viper.BindPFlag("inventory", remoteCmd.PersistentFlags().Lookup("inventory"))

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// remoteCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	remoteCmd.Flags().IntVar(&RemoteNodes, "nodes", -1, "Number of remote nodes")
}

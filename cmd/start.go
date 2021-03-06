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
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/matematik7/didcj/inventory"
	"github.com/matematik7/didcj/models"
	"github.com/matematik7/didcj/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var StartNodes int
var StartDaemonOnly bool

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		inv, err := inventory.Init(viper.GetString("inventory"))
		if err != nil {
			log.Fatal(err)
		}

		if !StartDaemonOnly {
			err = inv.Start(StartNodes)
			if err != nil {
				log.Fatal(err)
			}
		}

		servers, err := inv.Get()
		if err != nil {
			log.Fatal(err)
		}

		if StartDaemonOnly && len(servers) != StartNodes {
			log.Fatal("Not enough servers started")
		}

		log.Println("Killing didcj")
		err = utils.Run(servers, "killall", "-q", "didcj")
		if err != nil && errors.Cause(err).Error() != "exit status 1" {
			log.Fatalf("could not kill didcj: %v", err)
		}

		if !StartDaemonOnly {
			executable, err := os.Executable()
			if err != nil {
				log.Fatal(err)
			}

			log.Println("Uploading didcj")
			err = utils.Upload(executable, "didcj", servers...)
			if err != nil {
				log.Fatal(err)
			}
		}

		log.Println("Starting daemon")
		for _, server := range servers {
			go startDaemon(server)
		}

		select {}
	},
}

func startDaemon(server *models.Server) {
	for {
		allParams := append(utils.SSHParams,
			fmt.Sprintf("%s@%s", server.Username, server.IP.String()),
			"./didcj",
			"daemon",
		)
		cmd := exec.Command(
			"ssh",
			allParams...,
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			log.Println(err)
		}

		log.Printf("Restarting daemon on %s", server.Name)
	}
}

func init() {
	remoteCmd.AddCommand(startCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	startCmd.Flags().IntVar(&StartNodes, "nodes", 100, "Number of remote nodes")
	startCmd.Flags().BoolVar(&StartDaemonOnly, "daemon", false, "Only start daemon")
}

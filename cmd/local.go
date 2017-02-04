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
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var Nodes int

// localCmd represents the local command
var localCmd = &cobra.Command{
	Use:   "local",
	Short: "Run locally using dcj.sh tool",
	Long: `Runs the codejam code locally using dcj.sh which should be in
your path. It looks for updated .h file in ~/Downloads/`,
	Run: func(cmd *cobra.Command, args []string) {
		cppFiles, err := filepath.Glob("*.cpp")
		if err != nil {
			log.Fatal(err)
		}

		if len(cppFiles) < 1 {
			log.Fatal("No cpp files found!")
		}

		if len(cppFiles) > 1 {
			log.Fatal("Multiple cpp files found")
		}

		usr, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}

		hFile := strings.TrimSuffix(cppFiles[0], ".cpp") + ".h"
		downloadedHFile := path.Join(usr.HomeDir, "Downloads", hFile)

		if _, err = os.Stat(downloadedHFile); err == nil {
			log.Printf("Found new %s file at %s\n", hFile, downloadedHFile)
			os.Rename(downloadedHFile, hFile)
		}

		dcjCmd := exec.Command("dcj.sh", "test", "--source", cppFiles[0], "--nodes", strconv.Itoa(Nodes))
		dcjCmd.Stdout = os.Stdout
		dcjCmd.Stderr = os.Stderr
		dcjCmd.Run()
	},
}

func init() {
	RootCmd.AddCommand(localCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// localCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// localCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	localCmd.Flags().IntVar(&Nodes, "nodes", 10, "Number of local nodes")
}

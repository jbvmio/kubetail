// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
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
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

const (
	maxSize = 16384
)

var (
	flags      tailFlags
	regexOrder []RegexMaker
	logPool    *sync.Pool
	regexPool  *sync.Pool

	wg = &sync.WaitGroup{}
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kubetail <podname> <podname> ...",
	Short: "Tails and Follows logs from multiple Kubernetes Pods Simultaneously",
	Long: `Tail and Follow logs from multiple Kubernetes Pods Simultaneously. Searches and returns results using podnames as a wildcard search.
All matches that contain the podname string will be returned.

Examples:
  kubetail -i apache nginx                 // Tails logs from pods containing "apache" or "nginx", adding an id header.
  kubetail -i pod1 pod2 --tail 20          // Tails pod1 and pod2 beginning with the last 20 lines.
  kubetail --in-cluster pod1               // Use --in-cluster flag if running within a Pod itself.

Using white-list (--grep) and black-list (--vgrep) filters:

  kubetail -i apache nginx --vgrep 'GET,connection refused'
  kubetail -i apache nginx --grep "example.com,mysite.com" --vgrep POST

  The latter will tail logs from any pod with "apache" or "nginx" in it's name, filtering for anything containing
  either example.com or mysite.com but not containing the word POST.

  Filter order is determined by the order entered on the command line.
  For example:
   kubetail -i apache nginx --grep example.com --vgrep POST  // Filters for lines containing example.com first, then filters out any lines containing POST
   kubetail -i apache nginx --vgrep POST --grep example.com  // Filters out any lines containing POST first, then filters for lines containing example.com

Output is followed until stopped with Ctrl-C.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flags.blackRegex.Type = Black
		flags.whiteRegex.Type = White
		cmd.Flags().Visit(checkFlags)
		var pods []*PodLogger
		flags.targetPods.List = args
		podFind := flags.targetPods.GetRegex()
		var clientset *kubernetes.Clientset
		var targets []k8sSelfLink
		if flags.inCluster {
			clientset = createICClientSet()
		} else {
			clientset = createOCClientSet()
		}
		core := clientset.CoreV1()
		opts := logDefaults(flags.tailLines)
		urls := getSelfLinks("pods", clientset)
		for _, u := range urls {
			if podFind.MatchString(u.Name) {
				targets = append(targets, u)
			}
		}
		if len(targets) < 1 {
			fmt.Println("No Matching Pods Found.")
			os.Exit(0)
		}
		logPool = &sync.Pool{
			New: func() interface{} {
				return nil
			},
		}
		regexPool = &sync.Pool{
			New: func() interface{} {
				return nil
			},
		}
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		stopChan := make(chan struct{}, 1)
		for _, t := range targets {
			fmt.Println("Found: ", t.Name)
			req := core.Pods(t.Namespace).GetLogs(t.Name, opts)
			pods = append(pods, &PodLogger{
				pod:      t,
				req:      req,
				stopChan: stopChan,
				logPool:  logPool,
			})
		}

		switch {
		case len(regexOrder) > 0:
			wg.Add(2)
			go regexLogs(logPool, regexPool, regexOrder, stopChan)
			go processLogs(regexPool, flags.displayHeader, stopChan)
		default:
			wg.Add(1)
			go processLogs(logPool, flags.displayHeader, stopChan)
		}

		for _, pod := range pods {
			wg.Add(1)
			go pod.StartStream()
		}

		<-sigChan
		fmt.Println("Stopping")
		close(stopChan)
		wg.Wait()
	},
}

// Execute runs the command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVar(&flags.inCluster, "in-cluster", false, "enables kubetail to be used inside a pod.")
	rootCmd.Flags().BoolVarP(&flags.displayHeader, "id", "i", false, "display the pod name as a header along with the output.")
	rootCmd.Flags().StringSliceVar(&flags.whiteRegex.List, "grep", []string{}, `only display lines matching the specified text. Use a comma seperated string for multiple args.`)
	rootCmd.Flags().StringSliceVar(&flags.blackRegex.List, "vgrep", []string{}, `exclude any lines matching the specified text. Use a comma seperated string for multiple args.`)
	rootCmd.Flags().Int64Var(&flags.tailLines, "tail", 2, "start tail with defined no. of lines.")
	rootCmd.Flags().SortFlags = false
}

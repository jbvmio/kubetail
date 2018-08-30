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
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/jbvmio/channelrouter"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

//var cfgFile string
var (
	ic       bool
	id       bool
	match    bool
	tl       int64
	ss       int64
	white    []string
	black    []string
	wg       sync.WaitGroup
	mainStop bool

	print = true
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:              "kubetail <podname> <podname> ...",
	TraverseChildren: true,
	Short:            "Can Tail logs from multiple Kubernetes Pods Simultaneously",
	Long: `Tail logs from multiple Kubernetes Pods Simultaneously. Searches and returns results using podnames as a wildcard search.
All matches that contain the podname string will be returned.

Examples:
  kubetail -i apache nginx                 // Tails logs from pods containing "apache" or "nginx", adding an id header.
  kubetail -i pod1 pod2 --tail-lines 20    // Tails pod1 and pod2 beginning with the last 20 lines.
  kubetail --in-cluster pod1               // Use --in-cluster flag if running within a Pod itself.

Using white-list and black-list filters:

  kubetail -i apache nginx -w "example.com,mysite.com" -b POST

  This will tail logs from any pod with "apache" or "nginx" in it's name, filtering for anything containing
  either example.com or mysite.com but not containing the word POST.

  * Blacklist overrides Whitelist.

Output is followed until stopped with Ctrl-C or timeout occurs.`,

	Run: func(cmd *cobra.Command, args []string) {
		var str []string
		if len(args) < 1 {
			err := color.RedString("Want Logs? Which Pod(s)?\n")
			fmt.Printf("\n%v\n", err)
			cmd.Help()
			os.Exit(1)
		}
		str = args[:]
		if len(black) > 0 || len(white) > 0 {
			fmt.Printf("Filters > Blacklist:%v  Whitelist:%v\n", len(black), len(white))
			match = true
		}
		var clientset *kubernetes.Clientset
		var targets []k8sSelfLink
		if ic {
			clientset = createICClientSet()
		} else {
			clientset = createOCClientSet()
		}
		rand.Seed(time.Now().UnixNano())
		r := rand.Intn(43)
		s := spinner.New(spinner.CharSets[r], 100*time.Millisecond)
		s.Prefix = "Searching ... "
		s.Start()
		core := clientset.CoreV1()
		opts := logDefaults()
		opts.TailLines = &tl
		//WiP*
		//opts.SinceSeconds = &ss
		urls := getSelfLinks("pods", clientset)
		s.Stop()
		for _, u := range str {
			for _, i := range urls {
				if strings.Contains(i.Name, u) {
					fmt.Println("Found:", i.Name)
					targets = append(targets, i)
				}
			}
		}
		var err error

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		cr := channelrouter.NewChannelRouter(40960)
		//cr.Logger = log.New(os.Stdout, "[channelRouter] ", log.LstdFlags)
		stringChan := cr.AddChannel(4096)
		cr.Route()

		var pods []pod
		for _, t := range targets {
			p := cr.AddChannel(40960)
			writer := cr.MakeIoChannel(p)
			pd := pod{
				key:  p,
				cr:   cr,
				name: t.Name,
			}
			pods = append(pods, pd)
			ps := podLogStreamer{}
			ps.Req = core.Pods(t.Namespace).GetLogs(t.Name, opts)
			ps.ReadCloser, err = ps.Req.Stream()
			if err != nil {
				log.Fatalf("Error creating PodLogStreamer: %v\n", err)
			}
			wg.Add(1)
			go getPodLogs(writer, ps.ReadCloser)
		}
		for _, pd := range pods {
			wg.Add(1)
			go tailPodLogs(pd, stringChan, sigChan)
		}
		for i := 0; i < len(pods); i++ {
			wg.Done()
			wg.Done()
		}
		var n int
		for {
			if mainStop {
				break
			}
			select {
			case sig := <-sigChan:
				fmt.Printf("Caught signal %v: terminating\n", sig)
				mainStop = true
				break
			default:
				if cr.Available(stringChan) > 0 {
					str := cr.Receive(stringChan)
					s := str.ToString()
					if match {
						print = false
						if len(white) > 0 {
							if matchWhite(s, white) {
								print = true
							}
						}
						if len(black) > 0 {
							if matchBlack(s, black) {
								print = false
							} else {
								print = true
							}
						}
						if print {
							fmt.Printf(s)
						}
					} else {
						fmt.Printf(s)
					}
					n = 0
				} else {
					time.Sleep(time.Millisecond * 100)
					n++
				}
				if n >= len(pods)*100 {
					break
				}
			}
		}
		fmt.Println("kubetail stopped.")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVarP(&ic, "k8s", "k", false, "enables kubetail to be used inside a pod.")
	rootCmd.Flags().BoolVarP(&id, "id", "i", false, "display the pod name as a header along with the output.")
	rootCmd.Flags().StringSliceVarP(&white, "white-list", "w", []string{}, `only display lines matching the specified text. Use a comma seperated string for multiple args.`)
	rootCmd.Flags().StringSliceVarP(&black, "black-list", "b", []string{}, `exclude any lines matching the specified text. Use a comma seperated string for multiple args.`)
	rootCmd.Flags().Int64VarP(&tl, "tail-lines", "t", 10, "start tail with defined no. of lines.")

}

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jbvmio/channelrouter"
	cv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sSelfLink struct contains a mapping of a K8s resource and its selflink url.
type k8sSelfLink struct {
	Name      string
	Namespace string
	Kind      string
	URL       string
}

// GetSelfLinks get the resource as indicated by a string type (ie. "pods") and returns a set of self links for that resource.
func getSelfLinks(r string, cs *kubernetes.Clientset) []k8sSelfLink {
	var k8sLinks []k8sSelfLink
	switch r {
	case "pods":
		resource, err := cs.CoreV1().Pods("").List(metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		for _, i := range resource.Items {
			sl := k8sSelfLink{}
			sl.Name = getK8sValue("Name", i)
			sl.Namespace = getK8sValue("Namespace", i)
			sl.Kind = getK8sValue("Kind", i)
			if sl.Kind == "" {
				sl.Kind = "Pod"
			}
			sl.URL = getK8sValue("SelfLink", i)
			k8sLinks = append(k8sLinks, sl)
		}
	default:
		fmt.Println("Nothing to work with here.")
		os.Exit(1)
	}
	return k8sLinks
}

// GetK8sValue takes a K8s item and retrieves a value for a given field.
func getK8sValue(field string, item interface{}) string {
	i := reflect.ValueOf(item)
	return i.FieldByName(field).String()
}

func createOCClientSet() *kubernetes.Clientset {

	var kubeconfig string
	if home := homeDir(); home != "" {
		kubeconfig = string(home + "/" + ".kube" + "/" + "config")
	} else {
		fmt.Println("Cannot Locate kubeconfig at", kubeconfig)
		os.Exit(1)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Println("Cannot Locate kubeconfig at", kubeconfig)
		os.Exit(1)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("Cannot Locate kubeconfig at", kubeconfig)
		os.Exit(1)
	}
	return clientset
}

// CreateICClientSet Creates an In Cluster Clientset
func createICClientSet() *kubernetes.Clientset {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Println("unable to load in-cluster configuration, you sure you're running in a k8s cluster?")
		os.Exit(1)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("unable to load in-cluster configuration, you sure you're running in a k8s cluster?")
		os.Exit(1)
	}
	return clientset
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// PodLogStreamer struct contains the necessary components for requesting pod logs.
type podLogStreamer struct {
	Req        *rest.Request
	ReadCloser io.ReadCloser
}

type pod struct {
	key  channelrouter.Key
	cr   *channelrouter.ChannelRouter
	name string
}

// GetPodLogs streams pod logs to the given io.Writer.
func getPodLogs(wr io.Writer, rd io.ReadCloser) {
	defer rd.Close()
	wg.Wait()
	_, err := io.Copy(wr, rd)
	if err != nil {
		log.Fatalf("Error encountered reading tailing logs: %v\n", err)
	}
}

//TailPodLogs Here.
//func TailPodLogs(cr *channelrouter.ChannelRouter, k channelrouter.Key) {
func tailPodLogs(pd pod, stringChan channelrouter.Key, sigChan chan os.Signal) {
	//var stop bool
	var errdStop bool
	var more bool
	var errd error
	var b []byte
	var line string
	buf := bytes.NewBuffer(b)
	buf.Reset()
	defer func() {
		if errdStop {
			fmt.Println(pd.name, "Error:", errd)
		}
		fmt.Println(pd.name, "stopped.")
	}()

	wg.Wait()

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
			if buf.Len() > 256 {
				//fmt.Println(pd.name, "Buf Length", buf.Len())
				line, errd = buf.ReadString(10)
				if errd != nil {
					if errd.Error() == "EOF" {
						//fmt.Println(pd.name, "EOF Here.")
						more = true
					} else {
						mainStop = true
						errdStop = true
					}
				}
				if line == "" {
					more = true
				}

				if !more {
					if line != "" {
						if id {
							var s string
							head := color.YellowString("[%v]", pd.name)
							s = fmt.Sprintf("%v\n%v", head, line)
							pd.cr.Send(stringChan, s)
						} else {
							pd.cr.Send(stringChan, line)
						}
						line = ""
						if buf.Len() > 256 {
							more = false
						}
					}

				} else {
					//fmt.Println(pd.name, "spooling up")
					//fmt.Println(pd.name, "Buffer Length", buf.Len())
					time.Sleep(time.Millisecond * 300)
					if buf.Len() > 256 {
						more = false
					}
				}

			} else {
				if pd.cr.Available(pd.key) > 256 {
					var bits []byte
					var i uint32 = 0
					var available = pd.cr.Available(pd.key)
					//fmt.Println(pd.name, available)
					for i < available {
						bits = append(bits, byte(pd.cr.Receive(pd.key).ToInt()))
						i++
					}
					_, err := buf.Write(bits)
					if err != nil {
						fmt.Println("buffer error", err)
						mainStop = true
						break
					}
					more = false
				}
			}
		}
	}
}

func logDefaults() *cv1.PodLogOptions {
	var tl int64 = 10
	o := cv1.PodLogOptions{}
	o.Follow = true
	o.TailLines = &tl
	return &o
}

func matchWhite(s string, list []string) bool {
	for _, l := range list {
		if strings.Contains(s, l) {
			return true
		}
	}
	return false
}

func matchWhiteBytes(s string, list []string) bool {
	sb := []byte(s)
	for _, l := range list {
		lb := []byte(l)
		if bytes.Contains(sb, lb) {
			return true
		}
	}
	return false
}

func matchBlack(s string, list []string) bool {
	for _, l := range list {
		if strings.Contains(s, l) {
			return true
		}
	}
	return false
}

func matchBlackBytes(s string, list []string) bool {
	sb := []byte(s)
	for _, l := range list {
		lb := []byte(l)
		if bytes.Contains(sb, lb) {
			return true
		}
	}
	return false
}

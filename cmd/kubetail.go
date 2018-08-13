package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strings"

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

// CreateOCClientSet Creates an Out of Cluster Clientset
func createOCClientSet() *kubernetes.Clientset {
	// creates the out-cluster config
	var kubeconfig string
	if home := homeDir(); home != "" {
		kubeconfig = string(home + "/" + ".kube" + "/" + "config")
	} else {
		fmt.Println("Cannot Locate kubeconfig at", kubeconfig)
		os.Exit(1)
	}
	//flag.Parse()

	// creates the in-cluster config
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
	_, err := io.Copy(wr, rd)
	if err != nil {
		log.Fatalf("Error encountered reading tailing logs: %v\n", err)
	}
}

//TailPodLogs Here.
//func TailPodLogs(cr *channelrouter.ChannelRouter, k channelrouter.Key) {
func tailPodLogs(pd pod) {
	defer func() {
		fmt.Println("timed out.")
	}()
	var send = true
	var count int
	var line []byte
	for {
		l := pd.cr.Receive(pd.key)
		if string(byte(l.Int())) != "\n" {
			line = append(line, byte(l.Int()))
		} else {
			s := string(line)
			if match {
				if len(white) > 0 {
					if matchWhite(s, white) == true {
						send = true
					}
				}
				if len(black) > 0 {
					if matchBlack(s, black) == true {
						send = false
					}
				}
			}
			if send {
				if id {
					head := color.YellowString("[%v]", pd.name)
					s = fmt.Sprintf("%v\n%v", head, s)
				}
				fmt.Println(s) // Explore sending to seperate channel.
			}
			line = []byte{}
			if pd.cr.Available(pd.key) == 0 {
				count++
			}
		}
		if count > 1000 {
			break
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

func matchBlack(s string, list []string) bool {
	for _, l := range list {
		if strings.Contains(s, l) {
			return true
		}
	}
	return false
}

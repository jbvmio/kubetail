package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"reflect"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type tailFlags struct {
	inCluster     bool
	displayHeader bool
	tailLines     int64
	targetPods    RegexList
	blackRegex    RegexList
	whiteRegex    RegexList
}

func checkFlags(f *pflag.Flag) {
	switch f.Name {
	case "grep":
		regexOrder = append(regexOrder, &flags.whiteRegex)
	case "vgrep":
		regexOrder = append(regexOrder, &flags.blackRegex)
	}
}

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

// GetPodLogs streams pod logs to the given io.Writer.
func getPodLogs(wr io.Writer, rd io.ReadCloser) {
	defer rd.Close()
	//wg.Wait()
	_, err := io.Copy(wr, rd)
	if err != nil {
		log.Fatalf("Error encountered reading tailing logs: %v\n", err)
	}
}

package main

import (
	"flag"
	"time"

	"github.com/golang/glog"
	
	autopodderclientset "github.com/morvencao/kube-auto-podder/pkg/client/clientset/versioned"
	autopodderinformerfactory "github.com/morvencao/kube-auto-podder/pkg/client/informers/externalversions"
	"github.com/morvencao/kube-auto-podder/pkg/controller"
	"github.com/morvencao/kube-auto-podder/pkg/signals"
	
	kubeinformerfactory "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	masterURL		string
	kubeconfigfile	string
)

func main() {
	flag.Parse()
	
	// setup signals to handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	
	// bootstrap the config, if kubeconfigfile is empty, 
	// use cluster information when the code is running inside a pod
	config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigfile)
	if err != nil {
		glog.Fatalf("Failed to create config object from kube-config file: %q (error: %v)", 
		kubeconfigfile, err)
	}
	
	// create the clientset
	kubeclientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create clientset object from kube-config: error: %v", err)
	}
	
	apclientset, err := autopodderclientset.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create autopodder clientset object from kube-config: error: %v", err)
	}
	
	glog.Info("Successfully constructed k8s client!")
	
	kubeInformerFactory := kubeinformerfactory.NewSharedInformerFactory(kubeclientset, time.Second*30)
	apInformerFactory := autopodderinformerfactory.NewSharedInformerFactory(apclientset, time.Second*30)
	
	controller, err := controller.NewController(kubeclientset, apclientset, 
		kubeInformerFactory.Core().V1().Pods(), 
		apInformerFactory.Morven().V1().AutoPodders())
	if err != nil {
		glog.Fatalf("Error creating controller: %s", err.Error())
	}

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	apInformerFactory.Start(stopCh)
	
	// start the controller with two workers
	if err := controller.Run(2, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&masterURL, "masterURL", "", 
		"The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&kubeconfigfile, "kubeconfig", "", 
		"Use a Kubernetes configuration file instead of in-cluster configuration.")
}

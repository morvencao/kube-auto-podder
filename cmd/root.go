package cmd

import (
	goflag "flag"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type ServerArgs struct {
	kubeconfigfile	string
}

var (
	serverArgs	= ServerArgs{}
	rootCmd = &cobra.Command{
		Use:	"auto-podder",
		Short:	"Auto-podder is a simple pod kube controller.",
		Long:	`A simple kube controller that generate pods based on 'auto-pod' custom resource.`,
		Run:	func(cmd *cobra.Command, args []string) {
			// TODO
			glog.Info("Start...")
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverArgs.kubeconfigfile, "kubeconfig", "", 
		"Use a Kubernetes configuration file instead of in-cluster configuration.")

	// Add glog flags to pflag falgset, see: https://github.com/spf13/pflag#supporting-go-flags-when-using-pflag
	// And here: https://flowerinthenight.com/blog/2017/12/01/golang-cobra-glog
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	goflag.Parse()
}

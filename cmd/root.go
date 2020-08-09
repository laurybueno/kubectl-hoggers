/*
Copyright © 2020 Laury Bueno <laury.bueno@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/spf13/viper"
)

// Clients for the k8s' APIs
var clientset *kubernetes.Clientset
var metricsClientset *metricsv.Clientset

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kubectl-hoggers",
	Short: "Shed a light on the most resource intensive applications in a Kubernetes cluster",
	Long: `This is a kubectl plugin that uses multiple Kubernetes API endpoints to show data
about resource consumption in a convenient way.
The KUBECONFIG environment variable must be configured for it to work.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getClientSet(kubeconfig string) *kubernetes.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return clientset
}

func getMetricsClientset(kubeconfig string) *metricsv.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	metricsClientset, err := metricsv.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return metricsClientset
}

var kbFile string

func init() {
	// Try to get a kubeconfig flag, if available
	rootCmd.PersistentFlags().StringVar(&kbFile, "kubeconfig", "", "path to kubeconfig file (default is environment variable $KUBECONFIG")

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	// If kbFile is already set, then it has been passed as a command line flag
	// If not, try the get its value from environment
	if kbFile == "" {
		viper.AutomaticEnv() // read in environment variables
		if viper.GetString("KUBECONFIG") == "" {
			panic("$KUBECONFIG environment variable is not set. Aborting")
		}
		kbFile = viper.GetString("KUBECONFIG")
	}

	clientset = getClientSet(kbFile)
	metricsClientset = getMetricsClientset(kbFile)
}

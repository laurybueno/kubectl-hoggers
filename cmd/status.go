/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

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
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

// PodData from metrics
type PodData struct {
	name      string
	namespace string
	node      *NodeData
	CPU       *resource.Quantity
	RAM       *resource.Quantity
}

// NodeData from the main Kubernetes APIs
type NodeData struct {
	name string
	CPU  int64
	RAM  int64
}

// Clients for the k8s' APIs
var clientset *kubernetes.Clientset
var metricsClientset *metricsv.Clientset

// UI components
var grid *ui.Grid
var table *widgets.Table
var gauge *widgets.Gauge

// Table row limit
var rowsLimit int = 20

// API checking interval in seconds
const apiCheckInterval = 10

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: run,
}

func run(cmd *cobra.Command, args []string) {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	// Prepare the grid interface
	grid = ui.NewGrid()
	termWidth, termHeight := ui.TerminalDimensions()
	grid.SetRect(0, 0, termWidth, termHeight)

	go prepareDataTable()

	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		switch e.ID {
		case "q", "<C-c>":
			return
		}
	}
}

func prepareDataTable() {
	for {
		pods := getPodsData()
		populateWithNodeData(pods)

		// Prepare a table for the data
		table = widgets.NewTable()
		table.Rows = make([][]string, len(pods)+1)
		table.Rows[0] = []string{
			"namespace",
			"name",
			"node",
			"CPU",
			"RAM",
		}

		for k := range pods[:rangeLimit(pods)] {
			var nodeName string
			if pods[k].node == nil {
				nodeName = ""
			} else {
				nodeName = pods[k].node.name
			}
			table.Rows[k+1] = []string{
				pods[k].namespace,
				pods[k].name,
				nodeName,
				fmt.Sprintf("%vm", pods[k].CPU.MilliValue()),
				formatRAMStat(pods[k].RAM),
			}
		}

		updateInterface()

		// Wait before checking again
		time.Sleep(apiCheckInterval * time.Second)
	}
}

func updateInterface() {
	if table != nil {
		grid.Set(
			ui.NewRow(1.0/10,
				ui.NewCol(1.0/2, nil),
				ui.NewCol(1.0/2, gauge),
			),
			ui.NewRow(9.0/10,
				ui.NewCol(1.0/1, table),
			),
		)
	} else {
		// Tableless grid
		grid.Set(
			ui.NewRow(1.0/10,
				ui.NewCol(1.0/2, nil),
				ui.NewCol(1.0/2, gauge),
			),
		)
	}

	ui.Render(grid)
}

func updateGauge(current, total int) {
	gauge = widgets.NewGauge()
	gauge.Percent = int((float64(current) / float64(total)) * 100.0)

	updateInterface()
}

func getPodsData() []PodData {
	podMetricsList, err := metricsClientset.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	pods := make([]PodData, len(podMetricsList.Items))
	for k, v := range podMetricsList.Items {
		cpu := v.Containers[0].Usage.Cpu()
		ram := v.Containers[0].Usage.Memory()

		pods[k] = PodData{
			name:      v.GetName(),
			namespace: v.GetNamespace(),
			CPU:       cpu,
			RAM:       ram,
		}
	}

	sort.Slice(pods, func(i, j int) bool {
		return pods[i].CPU.MilliValue() > pods[j].CPU.MilliValue()
	})

	go updateGauge(1, len(pods))

	return pods
}

func populateWithNodeData(pods []PodData) {
	for k := range pods[:rangeLimit(pods)] {
		pod, err := clientset.CoreV1().Pods(pods[k].namespace).Get(context.TODO(), pods[k].name, metav1.GetOptions{})
		if err != nil {
			panic(err.Error())
		}
		pods[k].node = &NodeData{name: pod.Spec.NodeName}
		go updateGauge(k+1, len(pods))
	}
}

func formatRAMStat(n *resource.Quantity) string {
	// We do the RAM math here just like "kubectl top"
	// https://github.com/kubernetes/kubectl/blob/1cd20c9a5d1819f38ef95b87748ab04dc749ddb2/pkg/metricsutil/metrics_printer.go#L313
	return fmt.Sprintf("%vMi", n.Value()/(1024*1024))
}

func init() {
	rootCmd.AddCommand(statusCmd)
	viper.AutomaticEnv()

	clientset = getClientSet()
	metricsClientset = getMetricsClientset()

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// statusCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// statusCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func getClientSet() *kubernetes.Clientset {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", viper.GetString("KUBECONFIG"))
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

func getMetricsClientset() *metricsv.Clientset {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", viper.GetString("KUBECONFIG"))
	if err != nil {
		panic(err.Error())
	}

	metricsClientset, err := metricsv.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return metricsClientset
}

func rangeLimit(pods []PodData) int {
	minInt := func(x, y int) int {
		if x < y {
			return x
		}
		return y
	}

	return minInt(rowsLimit, len(pods))
}

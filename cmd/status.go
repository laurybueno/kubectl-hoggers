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
var progressText *widgets.Paragraph

// Table row limit
var rowsLimit int = 20

// API checking interval in seconds
const apiCheckInterval = 10

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Watch pods consuming most resources",
	Long: `List pods consuming most CPU resources along with its corresponding nodes.
Refreshes every 10 seconds.`,
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
		getNodeNameForPods(pods)

		// Prepare a table for the data
		if table == nil {
			table = widgets.NewTable()
			table.TextAlignment = ui.AlignCenter
			table.TextStyle = ui.NewStyle(ui.ColorGreen)
			table.BorderStyle = ui.NewStyle(ui.ColorGreen)
		}
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
	firstLine := ui.NewRow(1.0/10,
		ui.NewCol(3.0/4, progressText),
		ui.NewCol(1.0/4, gauge),
	)

	if table != nil {
		grid.Set(
			firstLine,
			ui.NewRow(9.0/10,
				ui.NewCol(1.0/1, table),
			),
		)
	} else {
		// Tableless grid
		grid.Set(
			firstLine,
		)
	}

	ui.Render(grid)
}

func updateGauge(current, total int) {
	if gauge == nil {
		gauge = widgets.NewGauge()

		progressText = widgets.NewParagraph()
		progressText.Text = fmt.Sprintf("[Fetching data from k8s API (every %v seconds)](fg:green)", apiCheckInterval)
		progressText.BorderStyle.Fg = ui.ColorGreen
	}

	percent := int((float64(current) / float64(total)) * 100.0)
	if percent == 100 {
		gauge.Title = "Waiting"
		gauge.BarColor = ui.ColorGreen
		gauge.TitleStyle.Fg = ui.ColorGreen
		gauge.BorderStyle.Fg = ui.ColorGreen
	} else {
		gauge.Title = "Refreshing data"
		gauge.BarColor = ui.ColorYellow
		gauge.TitleStyle.Fg = ui.ColorYellow
		gauge.BorderStyle.Fg = ui.ColorYellow
	}
	gauge.Percent = percent

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

	go updateGauge(1, rangeLimit(pods))

	return pods
}

// Get a list of pods and find out in wich node each is
func getNodeNameForPods(pods []PodData) {
	for k := range pods[:rangeLimit(pods)] {
		pod, err := clientset.CoreV1().Pods(pods[k].namespace).Get(context.TODO(), pods[k].name, metav1.GetOptions{})
		if err != nil {
			panic(err.Error())
		}
		pods[k].node = &NodeData{name: pod.Spec.NodeName}
		go updateGauge(k+1, rangeLimit(pods))
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

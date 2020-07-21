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

	go showInterface()

	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		switch e.ID {
		case "q", "<C-c>":
			return
		}
	}
}

func showInterface() {

	for {
		pods := getPodsData()

		// Prepare a table for the data
		table := widgets.NewTable()
		table.Rows = make([][]string, len(pods)+1)
		table.Rows[0] = []string{"namespace", "name", "CPU", "RAM"}
		for k, v := range pods {
			table.Rows[k+1] = []string{
				v.namespace,
				v.name,
				fmt.Sprintf("%vm", v.CPU.MilliValue()),
				formatRAMStat(v.RAM.MilliValue()),
			}
		}

		// Prepare the grid interface
		grid := ui.NewGrid()
		termWidth, termHeight := ui.TerminalDimensions()
		grid.SetRect(0, 0, termWidth, termHeight)
		grid.Set(
			ui.NewRow(1.0/1,
				ui.NewCol(1.0/1, table),
			),
		)
		ui.Render(grid)

		// Wait one second
		time.Sleep(1000 * time.Millisecond)
	}

}

func getPodsData() []PodData {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", viper.GetString("KUBECONFIG"))
	if err != nil {
		panic(err.Error())
	}

	clientset, err := metricsv.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	podMetricsList, err := clientset.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
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

	return pods
}

func formatRAMStat(n int64) string {
	return fmt.Sprintf("%vM", n/10e8)
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// statusCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// statusCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

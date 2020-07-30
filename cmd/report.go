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

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	tb "github.com/nsf/termbox-go"
)

// nodeData is a compiled struct from several API resources
type nodeData struct {
	name             string
	totalPods        int
	unRestrictedPods int64
	allocatableCPU   int64
	allocatableRAM   int64
	reservedCPU      int64
	reservedRAM      int64
	committedCPU     int64
	committedRAM     int64
	commitmentCPU    float64
	commitmentRAM    float64
}

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: runReport,
}

func runReport(cmd *cobra.Command, args []string) {
	nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	nodes := make([]nodeData, len(nodeList.Items))
	for k := range nodeList.Items {
		options := metav1.ListOptions{
			FieldSelector: fmt.Sprintf("spec.nodeName=%v", nodeList.Items[k].GetName()),
		}
		podList, err := clientset.CoreV1().Pods("").List(context.TODO(), options)
		if err != nil {
			panic(err.Error())
		}

		// Sum up everyting that pods on the current node request and may use
		var reservedCPU int64
		var reservedRAM int64
		var committedCPU int64
		var committedRAM int64
		var unRestrictedPods int64
		for k := range podList.Items {
			// TODO: take multiple containers per pod into account
			// TODO: what happens when only one of requests or limits is defined?

			restricted := false
			requests := podList.Items[k].Spec.Containers[0].Resources.Requests
			if requests != nil {
				reservedCPU += requests.Cpu().MilliValue()
				reservedRAM += requests.Memory().Value()
				restricted = true
			}

			limits := podList.Items[k].Spec.Containers[0].Resources.Limits
			if limits != nil {
				committedCPU += limits.Cpu().MilliValue()
				committedRAM += limits.Memory().Value()
				restricted = true
			}
			if !restricted {
				unRestrictedPods++
			}
		}

		// Consolidate node data
		allocatableCPU := nodeList.Items[k].Status.Allocatable.Cpu().MilliValue()
		allocatableRAM := nodeList.Items[k].Status.Allocatable.Memory().Value()
		nodes[k] = nodeData{
			name:             nodeList.Items[k].GetName(),
			totalPods:        len(podList.Items),
			unRestrictedPods: unRestrictedPods,
			allocatableCPU:   allocatableCPU,
			allocatableRAM:   allocatableRAM,
			reservedCPU:      reservedCPU,
			reservedRAM:      reservedRAM,
			committedCPU:     committedCPU,
			committedRAM:     committedRAM,
			commitmentCPU:    float64(committedCPU) / float64(allocatableCPU),
			commitmentRAM:    float64(committedRAM) / float64(allocatableRAM),
		}
	}

	// Show data in a good looking way
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	// Disable mouse input so that copy and paste works
	tb.SetInputMode(tb.InputEsc)
	defer ui.Close()

	// Prepare the grid interface
	grid = ui.NewGrid()
	termWidth, termHeight := ui.TerminalDimensions()
	grid.SetRect(0, 0, termWidth, termHeight)

	title := widgets.NewParagraph()
	title.Text = "[Resource commitment by node](fg:green)"
	title.Border = false

	table := widgets.NewTable()
	table.TextStyle = ui.NewStyle(ui.ColorGreen)
	table.BorderStyle = ui.NewStyle(ui.ColorGreen)
	table.Border = false
	table.TextAlignment = ui.AlignCenter
	table.SetRect(0, 0, termWidth, termHeight)

	grid.Set(
		ui.NewRow(1.0/10,
			ui.NewCol(1.0/1, title),
		),
		ui.NewRow(9.0/10,
			ui.NewCol(1.0/1, table),
		),
	)

	table.Rows = make([][]string, len(nodes)+1)

	// Table header
	table.Rows[0] = []string{
		"name",
		"totalPods",
		"unRestrictedPods",
		"committedCPU",
		"committedRAM",
		"committedCPU (% of total)",
		"committedRAM (% of total)",
	}

	for k := range nodes {
		table.Rows[k+1] = []string{
			nodes[k].name,
			fmt.Sprintf("%v", nodes[k].totalPods),
			fmt.Sprintf("%v", nodes[k].unRestrictedPods),
			fmt.Sprintf("%vm", nodes[k].committedCPU),
			fmt.Sprintf("%vMi", nodes[k].committedRAM/(1024*1024)),
			fmt.Sprintf("%.2f%%", nodes[k].commitmentCPU*100),
			fmt.Sprintf("%.2f%%", nodes[k].commitmentRAM*100),
		}
	}

	ui.Render(grid)

	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		switch e.ID {
		case "q", "<C-c>":
			return
		}
	}
}

func init() {
	rootCmd.AddCommand(reportCmd)
}

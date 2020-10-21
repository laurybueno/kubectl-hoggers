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
	_ "k8s.io/client-go/plugin/pkg/client/auth"

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
	pReservedCPU     float64
	pReservedRAM     float64
	pCommittedCPU    float64
	pCommittedRAM    float64
}

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Check CPU and RAM commitment by node",
	Long:  `Check CPU and RAM reservations/limits for each node in a cluster`,
	Run:   runReport,
}

func runReport(cmd *cobra.Command, args []string) {
	// Get all nodes
	nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	// For each node, check all pods and summarize data
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
			// TODO: what happens when only one of requests or limits is defined?
			restricted := false
			for _, v := range podList.Items[k].Spec.Containers {
				requests := v.Resources.Requests
				if requests != nil {
					reservedCPU += requests.Cpu().MilliValue()
					reservedRAM += requests.Memory().Value()
					restricted = true
				}

				limits := v.Resources.Limits
				if limits != nil {
					committedCPU += limits.Cpu().MilliValue()
					committedRAM += limits.Memory().Value()
					restricted = true
				}
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
			pReservedCPU:     float64(reservedCPU) / float64(allocatableCPU),
			pReservedRAM:     float64(reservedRAM) / float64(allocatableRAM),
			pCommittedCPU:    float64(committedCPU) / float64(allocatableCPU),
			pCommittedRAM:    float64(committedRAM) / float64(allocatableRAM),
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

	// Prepare title
	title := widgets.NewParagraph()
	title.Text = "[Resources reservations and limits by pods for each node](fg:green)"
	title.Border = false

	// Prepare table styles
	table := widgets.NewTable()
	table.TextStyle = ui.NewStyle(ui.ColorGreen)
	table.BorderStyle = ui.NewStyle(ui.ColorGreen)
	table.Border = false
	table.TextAlignment = ui.AlignCenter
	table.SetRect(0, 0, termWidth, termHeight)

	// Prepare main grid
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
		"total pods",
		"unrestricted pods",
		"CPU reservations",
		"CPU limits",
		"RAM reservations",
		"RAM limits",
	}

	// Table data
	for k := range nodes {
		table.Rows[k+1] = []string{
			nodes[k].name,
			fmt.Sprintf("%v", nodes[k].totalPods),
			fmt.Sprintf("%v", nodes[k].unRestrictedPods),
			fmt.Sprintf("%.2f%%", nodes[k].pReservedCPU*100),
			fmt.Sprintf("%.2f%%", nodes[k].pCommittedCPU*100),
			fmt.Sprintf("%.2f%%", nodes[k].pReservedRAM*100),
			fmt.Sprintf("%.2f%%", nodes[k].pCommittedRAM*100),
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

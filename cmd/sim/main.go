package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"vadadb/sim"
)

func main() {
	writer := csv.NewWriter(os.Stdout)
	_ = writer.Write([]string{"structure", "processes", "replication_nodes", "fusion_nodes", "savings"})
	for _, processes := range []int{10, 20, 30, 40, 50} {
		results, err := sim.Run(sim.Config{Processes: processes, Operations: 10000, InsertPercent: 60, Seed: 1})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, result := range results {
			_ = writer.Write([]string{result.Structure, strconv.Itoa(processes), strconv.Itoa(result.ReplicationNodes), strconv.Itoa(result.FusionNodes), fmt.Sprintf("%.2f", result.Savings)})
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

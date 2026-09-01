package sim

import (
	"fmt"
	"math/rand"

	"vadadb/structures"
)

type Config struct {
	Processes     int
	Operations    int
	InsertPercent int
	Seed          int64
}

type Result struct {
	Structure        string  `json:"structure"`
	Processes        int     `json:"processes"`
	ReplicationNodes int     `json:"replication_nodes"`
	FusionNodes      int     `json:"fusion_nodes"`
	Savings          float64 `json:"savings"`
}

type ReferenceRow struct {
	Processes      int     `json:"processes"`
	Queues         float64 `json:"queues"`
	Stacks         float64 `json:"stacks"`
	PriorityQueues float64 `json:"priority_queues"`
	Sets           float64 `json:"sets"`
	LinkedLists    float64 `json:"linked_lists"`
}

func ReferenceTable() []ReferenceRow {
	return []ReferenceRow{
		{10, 8.6, 9.6, 1.3, 6.5, 1.5},
		{20, 16.4, 18.3, 1.6, 12.3, 1.8},
		{30, 23.9, 26.7, 1.8, 18.5, 2.1},
		{40, 31.7, 34.8, 2.0, 24.3, 2.4},
		{50, 38.7, 43.7, 2.2, 30.5, 2.6},
	}
}

func Run(config Config) ([]Result, error) {
	if config.Processes < 2 {
		return nil, fmt.Errorf("processes must be at least two")
	}
	if config.Operations < 1 {
		return nil, fmt.Errorf("operations must be positive")
	}
	if config.InsertPercent < 1 || config.InsertPercent > 100 {
		return nil, fmt.Errorf("insert percent must be between 1 and 100")
	}
	type simulation struct {
		name   string
		insert func(int, uint64, *rand.Rand)
		remove func(int, *rand.Rand)
		size   func(int) int
		nodes  func() int
	}
	stack, _ := structures.NewFusedListStack[uint64](config.Processes)
	queue, _ := structures.NewFusedListQueue[uint64](config.Processes)
	priority, _ := structures.NewFusedPriorityQueue[uint64](config.Processes)
	set, _ := structures.NewFusedSet[uint64](config.Processes)
	linked, _ := structures.NewFusedLinkedList[uint64](config.Processes)
	simulations := []simulation{
		{"Stacks", func(p int, v uint64, _ *rand.Rand) { _ = stack.Push(p, v) }, func(p int, _ *rand.Rand) { _, _ = stack.Pop(p) }, func(p int) int { return len(stack.Source(p)) }, stack.NodeCount},
		{"Queues", func(p int, v uint64, _ *rand.Rand) { _ = queue.Enqueue(p, v) }, func(p int, _ *rand.Rand) { _, _ = queue.Dequeue(p) }, func(p int) int { return len(queue.Source(p)) }, queue.NodeCount},
		{"Priority Queues", func(p int, v uint64, _ *rand.Rand) { _ = priority.Push(p, v) }, func(p int, _ *rand.Rand) { _, _ = priority.PopMin(p) }, func(p int) int { return len(priority.Source(p)) }, priority.NodeCount},
		{"Sets", func(p int, v uint64, _ *rand.Rand) { _ = set.Add(p, v) }, func(p int, rng *rand.Rand) {
			values := set.Source(p)
			if len(values) > 0 {
				_, _ = set.Remove(p, values[rng.Intn(len(values))])
			}
		}, func(p int) int { return len(set.Source(p)) }, set.NodeCount},
		{"Linked Lists", func(p int, v uint64, rng *rand.Rand) { _ = linked.Insert(p, rng.Intn(len(linked.Source(p))+1), v) }, func(p int, rng *rand.Rand) {
			values := linked.Source(p)
			if len(values) > 0 {
				_, _ = linked.Delete(p, rng.Intn(len(values)))
			}
		}, func(p int) int { return len(linked.Source(p)) }, linked.NodeCount},
	}
	results := make([]Result, 0, len(simulations))
	for index, simulation := range simulations {
		rng := rand.New(rand.NewSource(config.Seed + int64(index)))
		for range config.Operations {
			process := rng.Intn(config.Processes)
			if simulation.size(process) == 0 || rng.Intn(100) < config.InsertPercent {
				simulation.insert(process, rng.Uint64(), rng)
			} else {
				simulation.remove(process, rng)
			}
		}
		replication := 0
		for process := range config.Processes {
			replication += simulation.size(process)
		}
		fusion := simulation.nodes()
		savings := 0.0
		if fusion > 0 {
			savings = float64(replication) / float64(fusion)
		}
		results = append(results, Result{simulation.name, config.Processes, replication, fusion, savings})
	}
	return results, nil
}

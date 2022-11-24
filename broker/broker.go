package main

import (
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type Boundary struct {
	Top    int
	Bottom int
}

func getSegmenttHeights(height, threads int) []int {
	segmentHeight := height / threads
	spare := height - (segmentHeight * threads)
	heights := make([]int, 0)
	for i := 0; i < threads; i++ {
		currentHeight := segmentHeight
		if spare > 0 {
			currentHeight++
			spare--
		}
		heights = append(heights, currentHeight)
	}
	return heights
}

func worldFromLiveCells(liveCells []util.Cell, height, width int) [][]byte {
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	for _, cell := range liveCells {
		world[cell.Y][cell.X] = 255
	}
	return world
}

func getLiveCells(world [][]byte) []util.Cell {
	liveCells := make([]util.Cell, 0)
	for y, row := range world {
		for x, status := range row {
			if status == 255 {
				liveCells = append(liveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return liveCells
}

var clients []*rpc.Client

type GolBroker struct{}

func (g *GolBroker) MainGol(req stubs.WorldData, res *stubs.WorldResponse) (err error) {
	if req.Threads <= len(clients) {

		controller, _ := rpc.Dial("tcp", req.ClientIP)

		heights := getSegmenttHeights(req.Height, req.Threads)
		segmentStart := 0

		responses := make([]stubs.WorldResponse, 0)
		doneChannels := make([]chan *rpc.Call, 0)
		boundaries := make([]Boundary, 0)

		for i := 0; i < req.Threads; i++ {
			initialisationData := stubs.WorldDataBounded{Data: req, Top: segmentStart, Bottom: segmentStart + heights[i]}
			clients[i].Call(stubs.InitialiseWorker, initialisationData, &stubs.Report{})

			responses = append(responses, stubs.WorldResponse{})
			doneChannels = append(doneChannels, make(chan *rpc.Call, 2))
			boundaries = append(boundaries, Boundary{Top: (req.Height + segmentStart - 1) % req.Height, Bottom: (segmentStart + heights[i]) % req.Height})

			segmentStart += heights[i]
		}

		world := req.World
		turn := 0
		liveCells := getLiveCells(world)
		for turn < req.Turn {

			select {
			default:
				liveCellsTemp := make([]util.Cell, 0)
				for i := 0; i < req.Threads; i++ {
					clients[i].Go("TakeTurn", stubs.BoundaryUpdate{Top: world[boundaries[i].Top], Bottom: world[boundaries[i].Bottom]}, &stubs.Report{}, doneChannels[i])
				}

				for i := 0; i < req.Threads; i++ {
					<-doneChannels[i]
					for _, cell := range responses[i].LiveCells {
						liveCellsTemp = append(liveCellsTemp, cell)
					}
				}

				world = worldFromLiveCells(liveCellsTemp, req.Height, req.Width)
				liveCells = liveCellsTemp
			}
			turn++
		}
	}

	return
}

func main() {

}

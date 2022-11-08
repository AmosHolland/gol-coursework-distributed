package gol

import (
	"fmt"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// distributor divides the work between workers and interacts with other goroutines.

func getLiveCells(world [][]byte, p Params) []util.Cell {
	liveCells := make([]util.Cell, 0)
	number := 0
	for y, row := range world {
		for x, status := range row {
			if status == 255 {
				liveCells = append(liveCells, util.Cell{X: x, Y: y})
				number++
			}
		}
	}
	return liveCells
}

func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	c.ioCommand <- ioInput
	fileName := fmt.Sprint(p.ImageWidth, "x", p.ImageHeight)
	fmt.Println(fileName)
	c.ioFilename <- fileName

	for y, row := range world {
		for x, _ := range row {
			world[y][x] = <-c.ioInput
		}
	}

	turn := 0

	// TODO: Execute all turns of the Game of Life.

	server := "54.84.37.1:8030"
	client, _ := rpc.Dial("tcp", server)

	err := client.Call(stubs.WorldLoader, stubs.WorldData{LiveCells: getLiveCells(world, p), Height: p.ImageHeight, Width: p.ImageWidth}, &stubs.Report{Message: ""})
	fmt.Println(err)

	response := stubs.WorldData{Height: p.ImageHeight, Width: p.ImageWidth}
	err = client.Call(stubs.TakeTurns, stubs.TurnRequest{Turn: p.Turns}, &response)
	fmt.Println(err)
	// TODO: Report the final state using FinalTurnCompleteEvent.

	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: response.LiveCells}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

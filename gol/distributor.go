package gol

import (
	"fmt"
	"net/rpc"
	"time"

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

func makeWorld(p Params, c distributorChannels) [][]byte {
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

	return world
}

func liveCellsReport(client *rpc.Client, c distributorChannels, p Params) {
	response := stubs.LiveCellsCount{LiveCells: 0, Turn: 0}
	for {
		time.Sleep(2 * time.Second)
		client.Call(stubs.GetLiveCells, stubs.TurnRequest{Turn: 0}, response)
		c.events <- AliveCellsCount{CompletedTurns: response.Turn, CellsCount: response.LiveCells}
	}
}

func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	world := makeWorld(p, c)

	turn := 0

	// TODO: Execute all turns of the Game of Life.

	server := "54.84.37.1:8030"
	client, _ := rpc.Dial("tcp", server)

	client.Call(stubs.WorldLoader, stubs.WorldData{LiveCells: getLiveCells(world, p), Height: p.ImageHeight, Width: p.ImageWidth}, &stubs.Report{Message: ""})

	response := stubs.WorldData{Height: p.ImageHeight, Width: p.ImageWidth}

	turnsFinished := make(chan *rpc.Call, 2)
	client.Go(stubs.TakeTurns, stubs.TurnRequest{Turn: p.Turns}, &response, turnsFinished)

	go liveCellsReport(client, c, p)
	// TODO: Report the final state using FinalTurnCompleteEvent.
	<-turnsFinished
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: response.LiveCells}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

package gol

import (
	"fmt"
	"net"
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

var eventPasser = make(chan Event)
var continueChan = make(chan bool)

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
	c.ioFilename <- fileName

	for y, row := range world {
		for x, _ := range row {
			world[y][x] = <-c.ioInput
		}
	}

	return world
}

type StatusReceiver struct{}

func (s *StatusReceiver) LiveCellReport(req stubs.LiveCellsCount, res *stubs.Report) (err error) {
	fmt.Println("Report received")
	eventPasser <- AliveCellsCount{CompletedTurns: req.Turn, CellsCount: req.LiveCells}
	return
}

func acceptListener(listener *net.Listener) {
	rpc.Accept(*listener)
}

func writePgm(world [][]byte, c distributorChannels, p Params) {
	c.ioCommand <- ioOutput
	fileName := fmt.Sprint(p.ImageWidth, "x", p.ImageHeight, "x", p.Turns)
	c.ioFilename <- fileName
	for _, row := range world {
		for _, cell := range row {
			c.ioOutput <- cell
		}
	}
}

func worldFromLiveCells(liveCells []util.Cell, p Params) [][]byte {
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	for _, cell := range liveCells {
		world[cell.Y][cell.X] = 255
	}
	return world
}

func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	world := makeWorld(p, c)

	turn := 0

	// TODO: Execute all turns of the Game of Life.

	server := "127.0.0.1:8030"
	client, _ := rpc.Dial("tcp", server)

	rpc.Register(&StatusReceiver{})
	listener, _ := net.Listen("tcp", ":8040")
	defer listener.Close()
	response := stubs.WorldResponse{}

	turnsFinished := make(chan *rpc.Call, 2)
	client.Go(stubs.TakeTurns, stubs.WorldData{LiveCells: getLiveCells(world, p), Width: p.ImageWidth, Height: p.ImageHeight, Turn: p.Turns, ClientIP: "127.0.0.1:8040"}, &response, turnsFinished)
	go acceptListener(&listener)
	// TODO: Report the final state using FinalTurnCompleteEvent.
	halt := false
	makePGM := false
	paused := false
	for {
		select {
		case event := <-eventPasser:
			c.events <- event
		case <-turnsFinished:
			halt = true
			makePGM = true
		case keyPress := <-c.ioInput:
			// send keyPress to golengine
			if keyPress == 'q' && !paused {
				halt = true
			}
			if keyPress == 'k' {
				halt = true
				makePGM = true
				<-continueChan
			}
			if keyPress == 'p' {
				if paused {
					fmt.Println("Continuing")
					paused = false
				} else {
					<-continueChan
					fmt.Println(response.Turn)
					paused = true
				}

			}

		}
		if halt {
			break
		}
	}

	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: response.LiveCells}
	if makePGM {
		writePgm(worldFromLiveCells(response.LiveCells, p), c, p)
	}

	// Make sure that the Io has finished any output before exiting.

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

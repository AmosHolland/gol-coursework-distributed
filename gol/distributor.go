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

// channel for sending events from rpc calls to the main program loop
var eventPasser = make(chan Event)

// distributor divides the work between workers and interacts with other goroutines.

// function to get a list of live cells from a given world
// goes through entire world, if a cell is live, it is added to the return list
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

// function to make a new 2D slice to represent a world given parameters and channels
// sends to IO asking for the world, and gives it the file name, then reads in all cell values from the IO channel
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

// struct for RPC calls
type StatusReceiver struct{}

// RPC function to allow the server to send live cell reports to the controller
func (s *StatusReceiver) LiveCellReport(req stubs.LiveCellsCount, res *stubs.Report) (err error) {
	fmt.Println("Report received")
	eventPasser <- AliveCellsCount{CompletedTurns: req.Turn, CellsCount: req.LiveCells}
	return
}

// function for accepting a listener without blocking
func acceptListener(listener *net.Listener) {
	rpc.Accept(*listener)
}

// function to write a PGM file using IO channels, sends each cell down IO channel after initialising
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

// function to create a full world from a list of live cells
// makes an empty world, default that all cells are dead, then sets all live cells to alive value
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

// main distributor function
func distributor(p Params, c distributorChannels) {

	world := makeWorld(p, c)

	turn := 0

	// setting up two-way RPC calls, server IP needs to be hardcoded
	server := "127.0.0.1:8030"
	client, _ := rpc.Dial("tcp", server)

	rpc.Register(&StatusReceiver{})
	listener, _ := net.Listen("tcp", ":8040")
	defer listener.Close()
	response := stubs.WorldResponse{}

	// making a channel for the golengine to report down after all turns have been completed, then calling
	// the server to process these turns, and accepting the server for rpc calls back
	turnsFinished := make(chan *rpc.Call, 2)
	client.Go(stubs.TakeTurns, stubs.WorldData{LiveCells: getLiveCells(world, p), Width: p.ImageWidth, Height: p.ImageHeight, Turn: p.Turns, ClientIP: "127.0.0.1:8040"}, &response, turnsFinished)
	go acceptListener(&listener)

	// flag variables to manage pausing and halting
	halt := false
	makePGM := false
	paused := false

	// main loop for dealing with events from outside of the controller
	for {
		// if a keypress has led to a halt, or the golengine has finished processing, then end the loop
		if halt {
			break
		}
		// select on relevant channels, code inside handles dealing with each event
		select {

		// just passes a passed event on to events channel (needs to be done here as it needs access to c)
		case event := <-eventPasser:
			c.events <- event
		// if the server is done processsing, then we need to stop and then generate a PGM
		case <-turnsFinished:
			halt = true
			makePGM = true

		// if a key is pressed then we need to handle this press
		case keyPress := <-c.ioInput:
			// key press is first send along to the golengine to deal with things on that end
			// this will block until things are finished on the server side
			// send keyPress to golengine
			// then deal with any client side behaviour by setting flag variables, and printing to console if
			// required
			if keyPress == 's' {
				// write a PGM specifically for the current turn, maybe bring name generation outside of writePGM?
			}
			if keyPress == 'q' && !paused {
				halt = true
			}
			if keyPress == 'k' {
				halt = true
				makePGM = true
			}
			if keyPress == 'p' {
				// if paused, unpause, otherwise pause
				if paused {
					fmt.Println("Continuing")
					paused = false
				} else {
					fmt.Println(response.Turn)
					paused = true
				}

			}

		}
	}

	// after main loop has ended send an event for the final turn, and create a final PGM of the world if necessary
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

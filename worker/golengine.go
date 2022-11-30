package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

// Channels to handle communication from rpc functions.
var keyPresses chan rune = make(chan rune)
var turnChan chan stubs.BoundaryUpdate = make(chan stubs.BoundaryUpdate)
var worldResponses chan []util.Cell = make(chan []util.Cell)
var stopRunning chan bool = make(chan bool)

// Function to remove 0s from a list of cells before sending.
func encodeCells(cells []util.Cell) []util.Cell {
	newCells := make([]util.Cell, 0)
	for _, cell := range cells {
		newCells = append(newCells, util.Cell{X: cell.X + 1, Y: cell.Y + 1})
	}
	return newCells
}

// Structure to store the world state of a segment as both a 2D slice and list of live cells.
type WorldState struct {
	World     [][]byte
	LiveCells []util.Cell
}

// Main engine of Game of life, calculates the next state of a world in game of life, and returns it.
func calculateNextState(world [][]byte, width, height, top, bottom int) WorldState {

	// makes an empty world to store live cells
	newWorld := make([][]byte, width)
	for i := range world {
		newWorld[i] = make([]byte, height)
	}

	// Empty list for new live cells
	newLiveCells := make([]util.Cell, 0)

	// Goes through every cell
	for y, row := range world {
		// Only processes a cell if it's within the workers range.
		if y >= top && y < bottom {
			for x, status := range row {
				// scores each cell
				score := scoreCell(x, y, width, height, world)
				// sets the next status of the world in accordance with the rules of the game of life
				var newStatus byte = 0
				if status == 255 {
					if score == 2 || score == 3 {
						newStatus = 255
					}
				} else {
					if score == 3 {
						newStatus = 255
					}
				}

				// If a cell is alive add it to the list of live cells.
				if newStatus == 255 {
					newLiveCells = append(newLiveCells, util.Cell{X: x, Y: y})
				}

				// sets the value of the cell in the new world to this value
				newWorld[y][x] = newStatus
			}
		}
	}

	// Return the new world state.
	return WorldState{World: newWorld, LiveCells: newLiveCells}
}

// function to score an individual cell by the number of live neighbours it has
func scoreCell(x, y, w, h int, world [][]byte) byte {

	var score byte = 0

	// goes through every neighbour to the cell
	for i := y - 1; i <= y+1; i++ {
		for j := x - 1; j <= x+1; j++ {
			// ensures we don't include the cell itself in our calculations
			if !(i == y && j == x) {
				// adds 1 if the current neighbour is alive, and 0 otherwise
				// indexes are designed to handle wrapping around the world
				score += (world[(h+i)%h][(w+j)%w] / 255)
			}
		}
	}
	return score
}

// function to get list of live cells in a given world, works the same as the one in the controller

// function to create a World struct from WorldData sent by the controller, constructs a world from list of
// live cells then sets other metadata based on what it's been given

// struct for rpc calls
type GolWorker struct{}

// rpc function for handling keypresses on client side
// sends keypress down a channel to the main worker loop, then waits for the worker to indicate that
// it's handled the keypress before returning

func (g *GolWorker) KeyPress(req stubs.KeyPress, res *stubs.Report) (err error) {
	keyPresses <- req.Key
	return
}

// RPC call to tell an already running GoL to take another turn.
func (g *GolWorker) TakeTurn(req stubs.BoundaryUpdate, res *stubs.WorldResponse) (err error) {
	// Sends boundaries down a channel
	turnChan <- req
	// Gets a respone back
	response := <-worldResponses

	// Sets liveness value based on whether or not there are any live cells.
	res.Liveness = 1
	if len(response) == 0 {
		res.Liveness = 2
	}

	// Encode cells before updating response object, and update turn.
	res.LiveCells = encodeCells(response)
	res.Turn = req.Turn + 1
	return
}

// RPC call for initialising a worker with new data.
func (g *GolWorker) StartWorker(req stubs.WorldDataBounded, res *stubs.Report) (err error) {
	// Runs a GolRunner, and wait for it to say that setup is done.
	setupDone := make(chan bool)
	go GolRunner(req, setupDone)
	<-setupDone
	return

}

func GolRunner(req stubs.WorldDataBounded, setupDone chan bool) {
	// Setup World data from request
	world := req.Data.World
	top := req.Top
	bottom := req.Bottom

	// Calculate boundary regions for given region.
	topBound := (top - 1 + req.Data.Height) % req.Data.Height
	bottomBound := (bottom) % req.Data.Height

	// Empty list for live cells.
	var liveCells []util.Cell

	// Flag variables.
	halt := false
	close := false

	// Indicate that the worker is ready to run.
	setupDone <- true

	// Main loop for processing turns.
	for !(halt || close) {
		select {
		// If new bounds have been sent in, take a turn.
		case bounds := <-turnChan:

			if bounds.Turn >= req.Data.Turn {
				halt = true
			}

			// Set boundary rows to their new values.
			world[topBound] = bounds.Top
			world[bottomBound] = bounds.Bottom

			// Then calculate a new state on this updated world.
			newState := calculateNextState(world, req.Data.Width, req.Data.Height, top, bottom)

			// Set values from newly calculated state.
			world = newState.World
			liveCells = newState.LiveCells

			// Send live cells back to called method to pass it to broker.
			worldResponses <- liveCells
		// If a key press is received set flags accordingly.
		case key := <-keyPresses:
			switch key {
			case 'q':
				halt = true
			case 'k':
				close = true
			}
		}

	}

	// If the worker has been told to close fully then send a signal to main to do this.
	if close {
		stopRunning <- true
	}
	fmt.Println("Quitting...")
}

// main function, sets up the golengine
func main() {
	// setting up rpc calls
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&GolWorker{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	go rpc.Accept(listener)

	// When the main function is told to stop, wait before doing this so any final messages can be sent.
	<-stopRunning
	time.Sleep(1 * time.Second)
}

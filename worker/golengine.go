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

// channels to handle communication from rpc called functions
var keyPresses chan rune = make(chan rune)
var stopRunning chan bool = make(chan bool)
var ticker chan bool = make(chan bool)
var liveCellChan chan stubs.LiveCellsCount = make(chan stubs.LiveCellsCount)
var keyPressResponses chan stubs.WorldResponse = make(chan stubs.WorldResponse)

// struct to store relevant data about a given world
type World struct {
	Board  [][]byte
	Height int
	Width  int
	Turn   int
}

// main engine of Game of life, calculates the next state of a world in game of life, and returns it
func calculateNextState(world [][]byte, width, height int) [][]byte {

	// makes an empty world to store live cells
	newWorld := make([][]byte, width)
	for i := range world {
		newWorld[i] = make([]byte, height)
	}

	// goes through every cell
	for y, row := range world {
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

			// sets the value of the cell in the new world to this value
			newWorld[y][x] = newStatus
		}
	}

	return newWorld
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

// function to create a World struct from WorldData sent by the controller, constructs a world from list of
// live cells then sets other metadata based on what it's been given
func loadWorld(worldData stubs.WorldData) *World {
	world := make([][]byte, worldData.Height)
	for i := range world {
		world[i] = make([]byte, worldData.Width)
	}
	for _, cell := range worldData.LiveCells {
		world[cell.Y][cell.X] = 255
	}

	return &World{Board: world, Width: worldData.Width, Height: worldData.Height, Turn: 0}
}

func acceptListener(listener *net.Listener) {
	rpc.Accept(*listener)
}

// struct for rpc calls
type GolWorker struct{}

// rpc function for handling keypresses on client side
// sends keypress down a channel to the main worker loop, then waits for the worker to indicate that
// it's handled the keypress before returning
func (g *GolWorker) KeyPressed(req stubs.KeyPress, res *stubs.WorldResponse) (err error) {
	keyPresses <- req.Key
	response := <-keyPressResponses
	res.LiveCells = response.LiveCells
	res.Turn = response.Turn
	return
}

func (g *GolWorker) LiveCellRequest(req stubs.Report, res *stubs.LiveCellsCount) (err error) {
	ticker <- true
	response := <-liveCellChan
	fmt.Println("Response generated")
	res.LiveCells = response.LiveCells
	res.Turn = response.Turn
	return
}

// main rpc function to tell the server to run the GOL based on some initial world data
func (g *GolWorker) ProgressToTurn(req stubs.WorldData, res *stubs.WorldResponse) (err error) {
	// sets up rpc connection, then loads the world and initialises flag variables
	world := loadWorld(req)
	pause := false
	halt := false
	close := false
	// if no turns actually need to be taken, don't even try
	if req.Turn <= world.Turn {
		fmt.Println("Turn already taken")
	} else {
		// ticker to track when status messages need to be sent
		// loop until all worlds are complete
		for world.Turn < req.Turn {
			// if keypresses mean the program should stop then exit the loop
			if close || halt {
				break
			}
			// if processing has not been paused
			if !pause {
				// select between control signals, and a default case
				select {
				// sending a live cells report every 2 seconds
				case <-ticker:
					fmt.Println("Sending live cells")
					liveCellChan <- stubs.LiveCellsCount{LiveCells: len(getLiveCells(world.Board)), Turn: world.Turn}
				// handling keypresses
				case keyPress := <-keyPresses:
					switch keyPress {
					// s needs to make a PGM, so send a response object with current status
					case 's':
						keyPressResponses <- stubs.WorldResponse{LiveCells: getLiveCells(world.Board), Turn: world.Turn}
					// q means that the client has shut, so indicate that the GOL needs to halt
					case 'q':
						halt = true
						keyPressResponses <- stubs.WorldResponse{}
					// k means that the GOL needs to end, and a new PGM needs to be made,
					// update the response object, indicate that it's okay to continue, and that the program needs to close
					case 'k':
						keyPressResponses <- stubs.WorldResponse{LiveCells: getLiveCells(world.Board), Turn: world.Turn}
						close = true
						time.Sleep(1 * time.Second)
						// p means pause, and the client needs to report the turn, so update the turn, indicate
					// that processing has been paused, then indicate that it's okay for the client to continue
					case 'p':
						fmt.Println("Pausing")
						keyPressResponses <- stubs.WorldResponse{LiveCells: make([]util.Cell, 0), Turn: world.Turn}
						pause = true
					}
				// by default take another turn of the GOL, and increment turn
				default:
					world.Board = calculateNextState(world.Board, world.Width, world.Height)
					world.Turn++
				}
				// if the program has been paused
			} else {
				// only need to handle keypresses in this state (I think), so no need for a select
				keyPress := <-keyPresses
				// only need to handle p and k in this state, for k behave as normal, for p unpause by setting pause
				// to false
				switch keyPress {
				case 'k':
					keyPressResponses <- stubs.WorldResponse{LiveCells: getLiveCells(world.Board), Turn: world.Turn}
					close = true
				case 'p':
					fmt.Println("Continuing")
					pause = false
					keyPressResponses <- stubs.WorldResponse{}
				}
			}
		}
	}
	// after all turns are done, or if the program has been told to fully stop, update the response object
	res.LiveCells = getLiveCells(world.Board)
	res.Turn = world.Turn
	// if the program needs to actually shut down, send down a channel to tell main that it needs to stop
	if close {
		stopRunning <- true
	}
	fmt.Println("Quitting...")
	return
}

// main function, sets up the golengine
func main() {
	// setting up rpc calls
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	rpc.Register(&GolWorker{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	go acceptListener(&listener)
	<-stopRunning
}

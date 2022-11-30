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
var turnChan chan stubs.BoundaryUpdate = make(chan stubs.BoundaryUpdate)
var worldResponses chan []util.Cell = make(chan []util.Cell)
var stopRunning chan bool = make(chan bool)
var ticker chan bool = make(chan bool)
var liveCellChan chan stubs.LiveCellsCount = make(chan stubs.LiveCellsCount)
var keyPressResponses chan stubs.WorldResponse = make(chan stubs.WorldResponse)

// struct to store relevant data about a given world
func encodeCells(cells []util.Cell) []util.Cell {
	newCells := make([]util.Cell, 0)
	for _, cell := range cells {
		newCells = append(newCells, util.Cell{X: cell.X + 1, Y: cell.Y + 1})
	}
	return newCells
}

type WorldState struct {
	World     [][]byte
	LiveCells []util.Cell
}

// main engine of Game of life, calculates the next state of a world in game of life, and returns it
func calculateNextState(world [][]byte, width, height, top, bottom int) WorldState {

	// makes an empty world to store live cells
	newWorld := make([][]byte, width)
	for i := range world {
		newWorld[i] = make([]byte, height)
	}

	newLiveCells := make([]util.Cell, 0)

	// goes through every cell
	for y, row := range world {
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

				if newStatus == 255 {
					newLiveCells = append(newLiveCells, util.Cell{X: x, Y: y})
				}

				// sets the value of the cell in the new world to this value
				newWorld[y][x] = newStatus
			}
		}
	}

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

func (g *GolWorker) LiveCellRequest(req stubs.Report, res *stubs.LiveCellsCount) (err error) {
	ticker <- true
	response := <-liveCellChan
	fmt.Println("Response generated")
	res.LiveCells = response.LiveCells
	res.Turn = response.Turn
	return
}

func (g *GolWorker) TakeTurn(req stubs.BoundaryUpdate, res *stubs.WorldResponse) (err error) {
	turnChan <- req
	response := <-worldResponses
	res.Liveness = 1
	if len(response) == 0 {
		res.Liveness = 2
	}
	res.LiveCells = encodeCells(response)
	res.Turn = req.Turn + 1
	return
}

func (g *GolWorker) StartWorker(req stubs.WorldDataBounded, res *stubs.Report) (err error) {
	setupDone := make(chan bool)
	go GolRunner(req, setupDone)
	<-setupDone
	return

}

func GolRunner(req stubs.WorldDataBounded, setupDone chan bool) {
	world := req.Data.World
	top := req.Top
	bottom := req.Bottom
	fmt.Println(top, bottom)
	topBound := (top - 1 + req.Data.Height) % req.Data.Height
	bottomBound := (bottom) % req.Data.Height
	fmt.Println(topBound, bottomBound)

	var liveCells []util.Cell

	halt := false
	close := false

	setupDone <- true

	for !(halt || close) {
		select {
		case bounds := <-turnChan:
			fmt.Println("Taking turn")
			if bounds.Turn >= req.Data.Turn {
				halt = true
			}

			world[topBound] = bounds.Top
			world[bottomBound] = bounds.Bottom
			if req.Data.Height == 16 && bounds.Turn <= 50 {
				fmt.Println(bounds.Turn)
				for y, row := range world {
					fmt.Println(y, row)
				}
			}

			newState := calculateNextState(world, req.Data.Width, req.Data.Height, top, bottom)

			world = newState.World
			liveCells = newState.LiveCells

			worldResponses <- liveCells
		case key := <-keyPresses:
			switch key {
			case 'q':
				halt = true
			case 'k':
				close = true
			}
		}

	}
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
	<-stopRunning
	time.Sleep(1 * time.Second)
}

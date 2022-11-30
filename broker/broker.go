package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"strings"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

// Global channels to pass controller requests to the main loop.
var keyPresses chan rune = make(chan rune)
var stopRunning chan bool = make(chan bool)

// List of rpc clients to keep track of the worker nodes the broker has access to
var workers []*rpc.Client = make([]*rpc.Client, 0)

// Structure to represent the boundary rows (above and below) of a segment.
type Boundary struct {
	Top    int
	Bottom int
}

// Takes cells that have been encoded to remove 0s, and decodes them.
func decodeCells(cells []util.Cell) []util.Cell {
	newCells := make([]util.Cell, 0)
	for _, cell := range cells {
		newCells = append(newCells, util.Cell{X: cell.X - 1, Y: cell.Y - 1})
	}
	return newCells
}

// Generates a list of segment heights for a certain height and number of workers.
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

// Generates a world as a 2D slice from a list of live cells in that world.
func worldFromLiveCells(liveCells []util.Cell, height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}

	for _, cell := range liveCells {
		world[cell.Y][cell.X] = 255
	}
	return world
}

// Gets a list of live cells from a world stored as a 2D slice.
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

// Struct for handling RPC calls.
type GolBroker struct{}

// If a key is pressed on the controller's side this function is called, which passes the key press to the main broker loop
func (g *GolBroker) KeyPress(req stubs.KeyPress, res *stubs.Report) (err error) {
	keyPresses <- req.Key
	return
}

// Main GoL loop, called by controller with relevant parameters, then starts workers to handle processing.
func (g *GolBroker) MainGol(req stubs.WorldData, res *stubs.WorldResponse) (err error) {
	fmt.Println("Gotcalled")
	fmt.Println(req.Threads)
	// Only attempt to run GoL if the broker has enough workers to handle the requested number of threads.
	if req.Threads <= len(workers) {
		// Connects to the controller to set up two way RPC
		controller, _ := rpc.Dial("tcp", req.ClientIP)

		// Slices to store information corresponding to worker nodes in workers (i.e. workers[0] uses heights[0], responses[0], etc.).
		heights := getSegmenttHeights(req.Height, req.Threads)
		responses := make([]stubs.WorldResponse, 0)
		doneChannels := make([]chan *rpc.Call, 0)
		boundaries := make([]Boundary, 0)

		// Used to track the indexes of each worker's segment
		segmentStart := 0

		// Sets up as many workers was requested by the controller.
		for i := 0; i < req.Threads; i++ {
			// Initialise the workers with world and boundary data.
			initialisationData := stubs.WorldDataBounded{Data: req, Top: segmentStart, Bottom: segmentStart + heights[i]}
			err = workers[i].Call(stubs.InitialiseWorker, initialisationData, &stubs.Report{})
			fmt.Println("Initialising worker", i, err)

			// Then sets up responses structures, communication channels, and boundaries for each worker on the #
			// broker side
			responses = append(responses, stubs.WorldResponse{})
			doneChannels = append(doneChannels, make(chan *rpc.Call, 2))

			// boundaries represents indexes of the edge regions that each worker needs to be updated with
			boundaries = append(boundaries, Boundary{Top: (req.Height + segmentStart - 1) % req.Height, Bottom: (segmentStart + heights[i]) % req.Height})

			// Increments segmentstart ready to initialise the next worker
			segmentStart += heights[i]
		}

		// Once workers are set up initialises world data on the broker's side.
		world := req.World
		turn := 0
		liveCells := getLiveCells(world)

		// Flag variables to control pausing, halting, and closing
		pause := false
		halt := false
		close := false

		// Start the ticker for sending live cell data to the controller, and then start taking turns.
		ticker := time.NewTicker(2 * time.Second)
		for turn < req.Turn {
			// End loop early if requested
			if halt || close {
				break
			}

			// If not paused
			if !pause {
				// Select statement to handle various events
				select {
				// If it's been 2 seconds, send a LiveCellReport to the controller.
				case <-ticker.C:
					controller.Call(stubs.LiveCellReport, stubs.LiveCellsCount{LiveCells: len(liveCells), Turn: turn}, &stubs.Report{})
				// If a key has been pressed on the controller, handle it.
				case keyPress := <-keyPresses:
					switch keyPress {
					// s needs to make a PGM, so send a response object with current status
					case 's':
						controller.Call(stubs.KeyPressResponse, stubs.WorldResponse{LiveCells: liveCells, Turn: turn}, &stubs.Report{})
					// q means that the client has shut, so indicate that the GOL needs to halt
					case 'q':
						halt = true
						// Propogates the halt message to all active workers.
						for i := 0; i < req.Threads; i++ {
							workers[i].Call(stubs.WorkerKeyPress, stubs.KeyPress{Key: 'q'}, &stubs.Report{})
						}
					// k means that the GOL needs to end, and a new PGM needs to be made,
					// update the response object, indicate that it's okay to continue, and that the program needs to close
					case 'k':
						controller.Call(stubs.KeyPressResponse, stubs.WorldResponse{LiveCells: liveCells, Turn: turn}, &stubs.Report{})
						close = true
						// Propogates the close message to all active workers.
						for i := 0; i < req.Threads; i++ {
							workers[i].Call(stubs.WorkerKeyPress, stubs.KeyPress{Key: 'k'}, &stubs.Report{})
						}
					// p means pause, and the client needs to report the turn, so send the client the current turn,
					// and set the pause flag to true.
					case 'p':
						controller.Call(stubs.KeyPressResponse, stubs.WorldResponse{LiveCells: liveCells, Turn: turn}, &stubs.Report{})
						pause = true
					}
				// If no events have happened, then take a turn of GoL
				default:
					// Temporary variable to store new live cells
					liveCellsTemp := make([]util.Cell, 0)

					// Tell all active workers to take a turn, providing them with the relevant boundary regions.
					for i := 0; i < req.Threads; i++ {
						workers[i].Go(stubs.TakeTurn, stubs.BoundaryUpdate{Top: world[boundaries[i].Top], Bottom: world[boundaries[i].Bottom], Turn: turn}, &responses[i], doneChannels[i])
					}

					// Then collate the responses from these calls.
					for i := 0; i < req.Threads; i++ {
						// Once the current worker is done.
						<-doneChannels[i]

						// If the worker's segment is alive, receive from it, otherwise just use an empty list as
						// the response.
						var response []util.Cell
						if responses[i].Liveness == 1 {
							response = decodeCells(responses[i].LiveCells)
						} else {
							response = make([]util.Cell, 0)
						}
						// Add the response to the collation of responses.
						liveCellsTemp = append(liveCellsTemp, response...)
					}

					// After receiving from all workers, make a world from their responses, and update liveCells.
					world = worldFromLiveCells(liveCellsTemp, req.Height, req.Width)
					liveCells = liveCellsTemp

					turn++
				}
			} else {
				// only need to handle keypresses in this state, so no need for a select
				keyPress := <-keyPresses
				// only need to handle p and k in this state, for k behave as normal, for p unpause by setting pause
				// to false
				switch keyPress {
				case 'k':
					controller.Call(stubs.KeyPressResponse, stubs.WorldResponse{LiveCells: liveCells, Turn: turn}, &stubs.Report{})
					for i := 0; i < req.Threads; i++ {
						workers[i].Call(stubs.WorkerKeyPress, stubs.KeyPress{Key: 'k'}, &stubs.Report{})
					}
					close = true
				case 'p':
					pause = false
				}
			}
		}
		// After turns have stopped being taken, update the response structure with current world state.
		res.LiveCells = liveCells
		res.Turn = turn

		// Then close connection to the controller.
		controller.Close()

		// If active workers have not already been told to halt, then tell them to.
		if !(close || halt) {
			for i := 0; i < req.Threads; i++ {
				workers[i].Call(stubs.WorkerKeyPress, stubs.KeyPress{Key: 'q'}, &stubs.Report{})
			}
		}

		// If the system has been closed completely, send down a channel to main to make the broker close.
		if close {
			fmt.Println("Closing")
			stopRunning <- true
		}

		fmt.Println("Quitting...")
	}
	return
}

func main() {
	pAddr := flag.String("port", "8050", "Port to listen on")
	// Broker is told what workers to connect to through a list of IPs.
	workerIPs := flag.String("workers", "127.0.0.1:8030", "comma separated (no spaces) of worker IPs")
	flag.Parse()

	// IPs are split into a list.
	ips := strings.Split(*workerIPs, ",")

	// Connects to workers through passed IPs (prints error so we can tell if the connection is successful)
	for _, ip := range ips {
		worker, err := rpc.Dial("tcp", ip)
		fmt.Println(err)
		workers = append(workers, worker)
	}

	// Sets up own RPC server, and starts accepting listeners, then waits to be told to stop running
	rpc.Register(&GolBroker{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	go rpc.Accept(listener)
	<-stopRunning

	// When closing shuts all worker connections on its end.
	for _, worker := range workers {
		worker.Close()
	}

	// Sleep for two second before actually closing so last messages have time to be sent.
	time.Sleep(2 * time.Second)
}

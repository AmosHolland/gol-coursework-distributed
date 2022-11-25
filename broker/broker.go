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

var keyPresses chan rune = make(chan rune)
var workers []*rpc.Client = make([]*rpc.Client, 0)
var stopRunning chan bool = make(chan bool)

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
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
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

type GolBroker struct{}

func (g *GolBroker) RegisterWorker(req stubs.WorkerInfo, res *stubs.Report) (err error) {
	worker, _ := rpc.Dial("tcp", req.WorkerIP)
	workers = append(workers, worker)
	return
}

func (g *GolBroker) KeyPress(req stubs.KeyPress, res *stubs.Report) (err error) {
	keyPresses <- req.Key
	return
}

func (g *GolBroker) MainGol(req stubs.WorldData, res *stubs.WorldResponse) (err error) {
	if req.Threads <= len(workers) {

		controller, _ := rpc.Dial("tcp", req.ClientIP)

		heights := getSegmenttHeights(req.Height, req.Threads)
		segmentStart := 0

		responses := make([]stubs.WorldResponse, 0)
		doneChannels := make([]chan *rpc.Call, 0)
		boundaries := make([]Boundary, 0)

		for i := 0; i < req.Threads; i++ {
			initialisationData := stubs.WorldDataBounded{Data: req, Top: segmentStart, Bottom: segmentStart + heights[i]}
			workers[i].Call(stubs.InitialiseWorker, initialisationData, &stubs.Report{})

			responses = append(responses, stubs.WorldResponse{})
			doneChannels = append(doneChannels, make(chan *rpc.Call, 2))
			boundaries = append(boundaries, Boundary{Top: (req.Height + segmentStart - 1) % req.Height, Bottom: (segmentStart + heights[i]) % req.Height})

			segmentStart += heights[i]
		}

		world := req.World
		turn := 0
		liveCells := getLiveCells(world)
		ticker := time.NewTicker(2 * time.Second)

		pause := false
		halt := false
		close := false

		for turn < req.Turn {

			if halt || close {
				break
			}

			if !pause {
				select {
				case <-ticker.C:
					controller.Call(stubs.LiveCellReport, stubs.LiveCellsCount{LiveCells: len(liveCells), Turn: turn}, &stubs.Report{})
				case keyPress := <-keyPresses:
					fmt.Println(keyPress)
					switch keyPress {
					// s needs to make a PGM, so send a response object with current status
					case 's':
						controller.Call(stubs.KeyPressResponse, stubs.WorldResponse{LiveCells: liveCells, Turn: turn}, &stubs.Report{})
					// q means that the client has shut, so indicate that the GOL needs to halt
					case 'q':
						halt = true
					// k means that the GOL needs to end, and a new PGM needs to be made,
					// update the response object, indicate that it's okay to continue, and that the program needs to close
					case 'k':
						controller.Call(stubs.KeyPressResponse, stubs.WorldResponse{LiveCells: liveCells, Turn: turn}, &stubs.Report{})
						close = true
						// p means pause, and the client needs to report the turn, so update the turn, indicate
					// that processing has been paused, then indicate that it's okay for the client to continue
					case 'p':
						controller.Call(stubs.KeyPressResponse, stubs.WorldResponse{LiveCells: liveCells, Turn: turn}, &stubs.Report{})
						pause = true
					}
				default:
					liveCellsTemp := make([]util.Cell, 0)
					for i := 0; i < req.Threads; i++ {
						workers[i].Go(stubs.TakeTurn, stubs.BoundaryUpdate{Top: world[boundaries[i].Top], Bottom: world[boundaries[i].Bottom]}, &responses[i], doneChannels[i])
					}

					for i := 0; i < req.Threads; i++ {
						<-doneChannels[i]
						liveCellsTemp = append(liveCellsTemp, responses[i].LiveCells...)
					}

					world = worldFromLiveCells(liveCellsTemp, req.Height, req.Width)
					liveCells = liveCellsTemp
				}
			} else {
				// only need to handle keypresses in this state (I think), so no need for a select
				keyPress := <-keyPresses
				// only need to handle p and k in this state, for k behave as normal, for p unpause by setting pause
				// to false
				switch keyPress {
				case 'k':
					controller.Call(stubs.KeyPressResponse, stubs.WorldResponse{LiveCells: liveCells, Turn: turn}, &stubs.Report{})
					close = true
				case 'p':
					pause = false
				}
			}
			turn++
		}
		res.LiveCells = liveCells
		res.Turn = turn

		if close {
			<-stopRunning
		}

		controller.Close()
		fmt.Println("Quitting...")
	}

	return
}

func main() {
	pAddr := flag.String("port", "8050", "Port to listen on")
	flag.Parse()

	rpc.Register(&GolBroker{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	stopped := false
	for {
		select {
		case <-stopRunning:
			stopped = true
		default:
			go rpc.Accept(listener)
		}
		if stopped {
			break
		}
	}
	for _, worker := range workers {
		worker.Close()
	}
}

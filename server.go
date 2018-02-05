package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net"
	"time"
)

const (
	webServerAddr = "127.0.0.1:7255"
	delayMean     = 4.0
	delayStd      = 3.0
)

// A Path contains fields for drawing SVG path elements
type Path struct {
	SVGString string
	Fill      string
	Stroke    string
}

var blackSquare = Path{"M 10 10 h 100 v 100 h -100 v -100", "black", "black"}
var yellowSquare = Path{"M 110 110 h 100 v 100 h -100 v -100", "yellow", "black"}
var redSquare = Path{"M 210 10 h 100 v 100 h -100 v -100", "red", "black"}
var blueSquare = Path{"M 10 210 h 100 v 100 h -100 v -100", "blue", "black"}
var greenSquare = Path{"M 210 210 h 100 v 100 h -100 v -100", "green", "black"}

var commandQueue = []Path{blackSquare, yellowSquare, redSquare, blueSquare, greenSquare}

func main() {

	newBlockCh := make(chan Path) // should contain blocks in the future
	i := 0
	go broadcastNewBlocks(newBlockCh)

	// pretend to be getting new shapes from the Block Chain/Mining Network
	for {
		delay := time.Duration(math.Max(rand.NormFloat64()*delayStd+delayMean, 0.0))
		time.Sleep(delay * time.Second)
		if i < 5 {
			fmt.Printf("Issuing command #%d\n", i)
			newBlockCh <- commandQueue[i]
		}
		i++
	}

}

// Every time a new block is made, this process will broadcast the block to the web-server
func broadcastNewBlocks(ch chan Path) {

	for {
		p := <-ch
		fmt.Println("About to tell the webserver about the new block")
		conn, err := net.Dial("tcp", webServerAddr)
		if err != nil {
			fmt.Println(err)
		}
		msg, err := json.Marshal(p)
		if err != nil {
			fmt.Println("json:", err)
		}
		_, err = conn.Write(msg)
		if err != nil {
			fmt.Println(err)
		}
		conn.Close()
	}

}

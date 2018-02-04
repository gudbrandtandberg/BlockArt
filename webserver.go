/*
 * Webserver serves a static html webpage and pushes drawing-
 * commands to the client over a websocket connection
 */

package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// A Path contains fields for drawing SVG path elements
type Path struct {
	SVGString string
	Fill      string
	Stroke    string
}

type Circle struct {
	cx, cy, r float64
	Fill      string
	Stroke    string
}

type Shape interface {
	Area() float64
}

func (c Circle) Area() float64 {
	return 0.0
}

func (p Path) Area() float64 {
	return 0.0
}

var blackSquare = Path{"M 10 10 h 100 v 100 h -100 v -100", "black", "black"}
var yellowSquare = Path{"M 110 110 h 100 v 100 h -100 v -100", "yellow", "black"}
var redSquare = Path{"M 210 10 h 100 v 100 h -100 v -100", "red", "black"}
var blueSquare = Path{"M 10 210 h 100 v 100 h -100 v -100", "blue", "black"}
var greenSquare = Path{"M 210 210 h 100 v 100 h -100 v -100", "green", "black"}

var commandQueue = []Path{blackSquare, yellowSquare, redSquare, blueSquare, greenSquare}

const (
	delayMean = 4.0
	delayStd  = 3.0
)

type cvsData struct {
	PageTitle string
}

func serve(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	data := cvsData{PageTitle: "BlockArt Drawing Server"}
	tmpl, err := template.ParseFiles("html/index.html")
	tmpl.Execute(w, data)
	if err != nil {
		log.Fatal(err)
	}
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func serveNewShape(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for i := 0; i < len(commandQueue); i++ {
		delay := time.Duration(math.Max(rand.NormFloat64()*delayStd+delayMean, 0.0))
		time.Sleep(delay * time.Second)

		cmd := commandQueue[i]
		msg, err := json.Marshal(cmd)
		if err != nil {
			fmt.Println("json:", err)
		}

		err = c.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

// serve main webpage and issue new drawing commands to the canvas
// over a websocket connection
func main() {
	http.Handle("/draw", http.HandlerFunc(serve))
	http.Handle("/commandserver", http.HandlerFunc(serveNewShape))
	fmt.Println("Starting server...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

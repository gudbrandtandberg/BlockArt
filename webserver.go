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
	"net"
	"net/http"

	"github.com/gorilla/websocket"
)

const (
	webServerAddr = "127.0.0.1:7255"
)

type cvsData struct {
	PageTitle string
	CVSWidth  string
	CVSHeight string
}

func serveIndex(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	data := cvsData{PageTitle: "BlockArt Drawing Server", CVSWidth: "512", CVSHeight: "512"}
	tmpl, err := template.ParseFiles("html/index.html")
	if err != nil {
		fmt.Println(err)
	}
	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println(err)
	}
}

// DrawResponse is the response the webserver sends to the webclient
type DrawResponse struct {
	Status string
}

// Shape stores the draw command that has been issued by a client
type Shape struct {
	SVGString string
	Fill      string
	Stroke    string
	ShapeType string
}

func handleDrawRequest(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var shape Shape
	err := decoder.Decode(&shape)
	if err != nil {
		panic(err)
	}
	req.Body.Close()

	// HERE WE NEED TO SUBMIT BLOCKARTLIB COMMANDS THROUGH AN ARTNODE INTO THE MINER NET

	fmt.Println("Will now submit draw command to a miner!")
	fmt.Println(shape)

	// Assume eveything went OK
	response := DrawResponse{"OK"}
	buff, err := json.Marshal(response)
	if err != nil {
		fmt.Println(err)
	}
	w.Write(buff)
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
var c *websocket.Conn // global ws-conn variable

func registerWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	c = conn
	fmt.Println("Websocket connection to client set up")
}

func broadcastNewBlocks(ch chan []byte) {
	for {
		buffer := <-ch
		err := c.WriteMessage(websocket.TextMessage, buffer)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func listenForNewBlocks(ch chan []byte) {
	l, err := net.Listen("tcp", webServerAddr)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("listening for TCP on webserverAddr:", l.Addr().String())
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println(err)
		}
		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)
		if err != nil {
			fmt.Println(err)
		}
		buffer = buffer[:n]
		ch <- buffer
	}
}

// serve main webpage and listen for / issue new drawing commands to the canvas
func main() {
	newBlockCh := make(chan []byte)
	go listenForNewBlocks(newBlockCh)
	go broadcastNewBlocks(newBlockCh)

	http.Handle("/home", http.HandlerFunc(serveIndex))
	http.Handle("/draw", http.HandlerFunc(handleDrawRequest))
	http.Handle("/registerws", http.HandlerFunc(registerWebsocket))

	fmt.Println("Starting server...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// This stuff should not really live here, but good to have some blockartlib functions for testing
// A Path contains fields for drawing SVG path elements
// type Path struct {
// 	SVGString string
// 	Fill      string
// 	Stroke    string
// }

// // Area is a way of computing area from Path object
// func (p Path) Area() float64 {
// 	parser := blockartlib.NewSVGParser()
// 	components, err := parser.Parse(p.SVGString)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	if p.Fill != "transparent" { // closed path
// 		return blockartlib.ShapeArea(components[0])
// 	}
// 	return blockartlib.LineArea(components) // open path
// }

// // Intersects checks if two paths intersect
// func Intersects(p1, p2 Path) bool {
// 	parser := blockartlib.NewSVGParser()
// 	c1, err := parser.Parse(p1.SVGString)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	c2, err := parser.Parse(p2.SVGString)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	return blockartlib.Intersects(c1, c2)
// }

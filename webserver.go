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

func serve(w http.ResponseWriter, req *http.Request) {
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

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
var c *websocket.Conn // global ws-conn variable

func websocketConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	c = conn
	fmt.Println("Websocket connection to client set up")
}

func broadcastNewBlocks(ch chan Path) {
	for {
		p := <-ch
		buffer, err := json.Marshal(p)
		if err != nil {
			fmt.Println(err)
		}
		err = c.WriteMessage(websocket.TextMessage, buffer)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func listenForNewBlocks(ch chan Path) {
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
		var p Path
		err = json.Unmarshal(buffer[:n], &p)

		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("Received path:", p)
		ch <- p
	}
}

// serve main webpage and listen for / issue new drawing commands to the canvas
func main() {
	newBlockCh := make(chan Path) //doesn't need to be path, could be json really..
	go listenForNewBlocks(newBlockCh)
	go broadcastNewBlocks(newBlockCh)

	http.Handle("/draw", http.HandlerFunc(serve))
	http.Handle("/registerws", http.HandlerFunc(websocketConnect))

	fmt.Println("Starting server...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// This stuff should not really live here, but good to have some blockartlib functions for testing

// A Path contains fields for drawing SVG path elements
type Path struct {
	SVGString string
	Fill      string
	Stroke    string
}

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

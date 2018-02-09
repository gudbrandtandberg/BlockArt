/*
 * Webserver serves a static html webpage and pushes drawing-
 * commands to the client over a websocket connection
 */

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
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
	Key       string
}

func serveIndex(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Serving index")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// This is a bit circular: webserver gives the client an identity,
	// then client shows webserver its identity.. fix registration here.
	curve := elliptic.P384()
	key, _ := ecdsa.GenerateKey(curve, rand.Reader)
	keyBytes, _ := x509.MarshalECPrivateKey(key)
	keyString := hex.EncodeToString(keyBytes)
	data := cvsData{PageTitle: "BlockArt Drawing Server", Key: keyString}

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

// DrawCommand stores the draw command that has been issued by a client
type DrawCommand struct {
	SVGString string
	Fill      string
	Stroke    string
	ShapeType string
	Key       string
	Addr      string
}

func handleDrawRequest(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var cmd DrawCommand
	err := decoder.Decode(&cmd)
	if err != nil {
		panic(err)
	}
	req.Body.Close()

	// HERE WE NEED TO SUBMIT BLOCKARTLIB COMMANDS THROUGH AN ARTNODE INTO THE MINER NET

	fmt.Println("Will now submit draw command to a miner!")
	fmt.Println(cmd)

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

	http.Handle("/", http.FileServer(http.Dir("./html/"))) // for serving 'client.js'
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

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

	"./blockartlib"
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
	keyString, err := encodeKey(*key)
	if err != nil {
		fmt.Println(err)
	}

	data := cvsData{PageTitle: "BlockArt Client App", Key: keyString}

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
	Status       string
	CanvasX      uint32
	CanvasY      uint32
	InkRemaining uint32
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

const valNum = uint8(2)

// Handler for the /draw folder. Receives and parses a draw command,
// opens a canvas, adds a shape, closes the canvas, then writes a
// response to the client. Writes any errors as they appear.
func handleDrawRequest(w http.ResponseWriter, req *http.Request) {
	cmd := parseRequest(req)
	key, err := decodeKey(cmd.Key)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Will now submit draw command to a miner!")
	response := DrawResponse{}
	canvas, settings, err := blockartlib.OpenCanvas(cmd.Addr, *key)
	if err != nil {
		response.Status = err.Error()
		writeResponse(w, response)
		return
	}
	response.CanvasX = settings.CanvasXMax
	response.CanvasY = settings.CanvasYMax

	shapeType := parseShapeType(cmd.ShapeType)
	_, _, _, err = canvas.AddShape(valNum, shapeType, cmd.SVGString, cmd.Fill, cmd.Stroke)
	if err != nil {
		response.Status = err.Error()
		writeResponse(w, response)
		return
	}
	ink, err := canvas.CloseCanvas()
	if err != nil {
		response.Status = err.Error()
		writeResponse(w, response)
		return
	}
	response.InkRemaining = ink
	response.Status = "OK"
	writeResponse(w, response)
}

func parseRequest(req *http.Request) DrawCommand {
	decoder := json.NewDecoder(req.Body)
	var cmd DrawCommand
	err := decoder.Decode(&cmd)
	if err != nil {
		fmt.Println(err)
	}
	req.Body.Close()
	return cmd
}
func writeResponse(w http.ResponseWriter, res DrawResponse) {
	buff, err := json.Marshal(res)
	if err != nil {
		fmt.Println(err)
	}
	w.Write(buff)
}

func decodeKey(hexStr string) (key *ecdsa.PrivateKey, err error) {
	keyBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return key, err
	}
	return x509.ParseECPrivateKey(keyBytes)
}
func encodeKey(key ecdsa.PrivateKey) (string, error) {
	keyBytes, err := x509.MarshalECPrivateKey(&key)
	if err != nil {
		return "", err
	}
	keyString := hex.EncodeToString(keyBytes)
	return keyString, nil
}

func parseShapeType(shapeType string) blockartlib.ShapeType {
	switch shapeType {
	case "Path":
		return blockartlib.PATH
	case "Circle":
		return blockartlib.CIRCLE
	default:
		return blockartlib.PATH
	}
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

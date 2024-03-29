/*
 * Webserver serves a static html webpage and pushes drawing-
 * commands to the client over a websocket connection
 */

package main

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"./blockartlib"
	"strings"
	"net"
	"os"
)

var minerAddr string
// WebServer version of the func in art-app.go
func readMinerAddrKeyWS(minerAddr string) (key string, err error) {
	infos, err := ioutil.ReadDir("keys")
	if err != nil {
		return
	}
	if len(infos) == 0 {
		err = errors.New("There are currently no miners online (according to ./keys/)")
		return
	}
	splits := strings.Split(minerAddr, ":")
	port := splits[len(splits)-1]
	_, err = net.ResolveTCPAddr("tcp", minerAddr)
	keyBytes, err := ioutil.ReadFile("./keys/" + port)
	if err != nil {
		return
	}

	return string(keyBytes), err
}

const (
	webServerAddr = "127.0.0.1:7255"
)

type cvsData struct {
	PageTitle string
	Key       string
	Addr      string
}

func serveIndex(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Serving index")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	key, err := readMinerAddrKeyWS(minerAddr)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	data := cvsData{PageTitle: "BlockArt Client App", Key: key, Addr: minerAddr}

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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	cmd := parseRequest(req)
	var err error = nil
	//key, err := decodeKey(cmd.Key)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Will now submit draw command to a miner!")
	response := DrawResponse{}

	response.CanvasX = settings.CanvasXMax
	response.CanvasY = settings.CanvasYMax

	shapeType := parseShapeType(cmd.ShapeType)
	_, _, _, err = canvas.AddShape(valNum, shapeType, cmd.SVGString, cmd.Fill, cmd.Stroke)
	if err != nil {
		response.Status = err.Error()
		writeResponse(w, response)
		return
	}

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

/*
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
*/

func handleChainRequest(w http.ResponseWriter, r *http.Request) {
	blocks, longestHash := getBlockChain(canvas)

	genesisHash, err := canvas.GetGenesisBlock()

	type data struct {
		Genesis string
		LongestChain string
		Blocks map[string][]string
	}
	d := data{genesisHash, longestHash, blocks}
	resp, err := json.Marshal(d)
	if err != nil {
		fmt.Println(err)
	}
	w.Write(resp)
}

func handleShapesRequest(w http.ResponseWriter, r *http.Request) {
	blocks, _ := getBlockChain(canvas)
	genesis, err := canvas.GetGenesisBlock()
	if err != nil {
		fmt.Println(err)
	}
	longestChain := findLongestChain(canvas, blocks, genesis)
	shapes := make([]string, 0)
	for _, block := range(longestChain.Chain) {
		shapeHashes, err := canvas.GetShapes(block)
		if err != nil {
			fmt.Println(err)
		}
		for _, hash := range(shapeHashes) {
			shape, err := canvas.GetSvgString(hash)
			if err != nil {
				fmt.Println(err)
			}
			shapes = append(shapes, shape)
		}
	}
	resp, err := json.Marshal(shapes)
	w.Write(resp)
}

func getBlockChain(canvas blockartlib.BACanvas) (blocks map[string][]string, cur string) {
	blocks = make(map[string][]string)
	queue := make([]string, 0)
	cur, err := canvas.GetGenesisBlock()
	if err != nil {
		return
	}
	queue = append(queue, cur)
	for len(queue) > 0 {
		cur = queue[0]
		children, err := canvas.GetChildren(cur)
		if err != nil {
			return
		}
		blocks[cur] = children
		queue = append(queue[1:], children...)
	}

	return
}

type Chain struct {
	Length int
	Chain []string
}

func findLongestChain(canvas blockartlib.BACanvas, blocks map[string][]string, start string) (chain Chain) {
	// recursively find the longest chain
	chain.Length = 1
	chain.Chain = make([]string, 0)
	chain.Chain = append(chain.Chain, start)

	children := make([]Chain, 0)
	for _, child := range blocks[start] {
		result := findLongestChain(canvas, blocks, child)
		children = append(children, result)
	}

	// sort the children by chain length
	sort.Slice(children, func(i, j int) bool {return children[i].Length > children[j].Length})
	if len(children) > 0 {
		child := children[0]
		child.Length += 1
		child.Chain = append(chain.Chain, child.Chain...)
		return child
	}

	return
}

var canvas blockartlib.BACanvas
var settings blockartlib.CanvasSettings

// serve main webpage and listen for / issue new drawing commands to the canvas
func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: webserver.go [miner ip:port]")
		os.Exit(1)
	}
	var err error
	minerAddr = os.Args[1]
	//newBlockCh := make(chan []byte)
	//go listenForNewBlocks(newBlockCh)
	//go broadcastNewBlocks(newBlockCh)
	keystring, err := readMinerAddrKeyWS(minerAddr)
	if err != nil {
		log.Println("failed read")
	}
	key, err := decodeKey(keystring)
	if err != nil {
		log.Println("failed decode")
	}

	canvas, settings, err = blockartlib.OpenCanvas(minerAddr, *key)
	if err != nil {
		log.Println("failed opencanvas")
	}

	http.Handle("/", http.FileServer(http.Dir("./html/"))) // for serving 'client.js'
	http.Handle("/home", http.HandlerFunc(serveIndex))
	http.Handle("/draw", http.HandlerFunc(handleDrawRequest))
	// http.Handle("/registerws", http.HandlerFunc(registerWebsocket))
	http.Handle("/blocks", http.HandlerFunc(handleChainRequest))
	http.Handle("/shapes", http.HandlerFunc(handleShapesRequest))

	fmt.Println("Starting server...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

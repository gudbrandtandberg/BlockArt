/*

A trivial application to illustrate how the blockartlib library can be
used from an application in project 1 for UBC CS 416 2017W2.

From spec:
"an application that uses blockartlib and produces an html file as output that contains
 an svg canvas that is the result of the application's distributed activity"

Usage:
go run art-app.go
*/

package main

// Expects blockartlib.go to be in the ./blockartlib/ dir, relative to
// this art-app.go file
import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"net"
	"strings"
	"time"
	"./blockartlib"
	"sort"
)

var examples = map[string]string{
	"diag1":          "M 1 1 L 3 3",
	"diag2":          "M 1 3 L 3 1",
	"diag3":          "M 3 2 L 4 3",
	"diag4":          "M 1 1 L 4 4",
	"cross":          "M 1 1 L 3 3 M 1 3 L 3 1",
	"sq1":            "M 1 1 h 2 v 2 h -2 z",
	"sq2":            "M 2 2 h 2 v 2 h -2 z",
	"sq3":            "M 1.1 1.1 h 1 v 1 h -1 z", //is contained in sq1
	"line1":          "M 1.1 1.1 l 1 1",          //is contained in sq1
	"129":            "M 1 1 l 149 100 h 10 v -10 h 10 v -10 h -10 v 10 h -10 v -10 h 10 v -10 h 100 v 100 h -100 v -50 h -10 v 50 h -10 v -50 l -10 0 z",
	"128":            "M 1 1 l 149 99 h 10 v -10 h 10 v -10 h -10 v 10 h -10 v -10 h 10 v -10 h 100 v 100 h -100 v -50 h -10 v 50 h -10 v -50 l -10 0 z",
	"unitcircle":     "0, 0, 1",
	"half":           "0, 0, 0.5",
	"pathinside":     "M 0 0 l 0.3 0.2 l -0.3 0 z",
	"pathoutside":    "M 2 2 h 10",
	"pathintersects": "L 3 3 l 0 -3 z",
}

func decodeKey(hexStr string) (key *ecdsa.PrivateKey, err error) {
	keyBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return key, err
	}
	return x509.ParseECPrivateKey(keyBytes)
}

func readMinerAddrKey() (minerAddr string, key *ecdsa.PrivateKey, err error) {
	infos, err := ioutil.ReadDir("keys")
	if err != nil {
		return
	}
	if len(infos) == 0 {
		err = errors.New("There are currently no miners online (according to ./keys/)")
		return
	}
	var port string
	for _, fileinfo := range infos {
		if !strings.HasPrefix(fileinfo.Name(), ".") {
			port = fileinfo.Name()
			break
		}
	}
	ip, err := net.ResolveTCPAddr("tcp", "localhost:"+port)
	minerAddr = ip.String()
	keyBytes, err := ioutil.ReadFile("./keys/" + port)
	if err != nil {
		return
	}
	key, err = decodeKey(string(keyBytes))
	return
}

func getBlockChain(canvas blockartlib.BACanvas) (blocks map[string][]string, cur string) {
	blocks = make(map[string][]string)
	queue := make([]string, 0)
	cur, err := canvas.GetGenesisBlock()
	checkError(err)
	queue = append(queue, cur)
	for len(queue) > 0 {
		cur = queue[0]
		children, err := canvas.GetChildren(cur)
		checkError(err)
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

func main() {

	// testParser()
	// return
	//minerAddr := "127.0.0.1:9878"
	// curve := elliptic.P384()
	// privKey, err := ecdsa.GenerateKey(curve, rand.Reader)

	minerAddr, privKey, err := readMinerAddrKey()
	if checkError(err) != nil {
		return
	}

	//Open a canvas.
	canvas, _, err := blockartlib.OpenCanvas(minerAddr, *privKey)
	if checkError(err) != nil {
		return
	}

	validateNum := uint8(2)

	// Add a line.
	shapeHash, _, _, err := canvas.AddShape(validateNum, blockartlib.PATH, "M 0 0 L 0 5", "transparent", "red")
	if checkError(err) != nil {
		return
	}

	// Read the shapeHash
	SVGString, err := canvas.GetSvgString(shapeHash)
	if checkError(err) != nil {
		return
	}
	fmt.Println(SVGString)

	// Get the genesis block
	genesisHash, err := canvas.GetGenesisBlock()
	if checkError(err) != nil {
		return
	}
	fmt.Println(genesisHash)

	// Get the children of the genesis block
	genesisChildren, err := canvas.GetChildren(genesisHash)
	if checkError(err) != nil {
		return
	}
	fmt.Println(genesisChildren)

	// Get the shapes contained in the genesis block
	shapeHashes, err := canvas.GetShapes(genesisHash)
	if checkError(err) != nil {
		return
	}
	fmt.Println(shapeHashes)

	// Delete a shape

	fmt.Println("Will delete", shapeHash)
	_, err = canvas.DeleteShape(validateNum, shapeHash)
	if checkError(err) != nil {
		return
	}

	blocks, last := getBlockChain(canvas)
	for k, v := range(blocks) {
		fmt.Println(k, v)
	}
	fmt.Println("last hash: ", last)

	chain := findLongestChain(canvas, blocks, genesisHash)
	fmt.Println(chain)
	fmt.Println("chain length, ", chain.Length)

	for {
		ink, err := canvas.GetInk()
		if checkError(err) != nil {
			return
		}
		fmt.Println("Ink remaining:", ink)

		time.Sleep(time.Second * 2)
	}
	return

	// Add two lines.
	_, _, _, err = canvas.AddShape(validateNum, blockartlib.PATH, "M 0 0 L 0 5 M 0 1 h 5", "transparent", "red")
	if checkError(err) != nil {
		return
	}

	// Add a square.
	_, _, _, err = canvas.AddShape(validateNum, blockartlib.PATH, "M 0 0 h 5 v 5 h -5 z", "blue", "black")
	if checkError(err) != nil {
		return
	}

	// Add a circle.
	_, _, _, err = canvas.AddShape(validateNum, blockartlib.CIRCLE, "5,5,1", "blue", "black")
	if checkError(err) != nil {
		return
	}

	// Add a 'too long' path.
	_, _, _, err = canvas.AddShape(validateNum, blockartlib.PATH, examples["129"], "blue", "black")
	if checkError(err) != nil {
		return
	}

	// // Add another line.
	// shapeHash2, blockHash2, ink2, err := canvas.AddShape(validateNum, blockartlib.PATH, "M 0 0 L 5 0", "transparent", "blue")
	// if checkError(err) != nil {
	// 	return
	// }

	// // Delete the first line.
	// ink3, err := canvas.DeleteShape(validateNum, shapeHash)
	// if checkError(err) != nil {
	// 	return
	// }

	// // assert ink3 > ink2

	// // Close the canvas.
	// ink4, err := canvas.CloseCanvas()
	// if checkError(err) != nil {
	// 	return
	// }
}

// If error is non-nil, print it out and return it.
func checkError(err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error %s\n", err)
		return err
	}
	return nil
}

func testParser() {
	parser := blockartlib.NewSVGParser()

	shape1, _ := parser.Parse(blockartlib.CIRCLE, examples["unitcircle"], "transparent", "black")
	shape2, _ := parser.Parse(blockartlib.CIRCLE, examples["half"], "transparent", "black")

	fmt.Println(shape1)
	fmt.Println(shape2)

	intersects := blockartlib.Intersects(shape1, shape2)
	fmt.Println("Shapes intersect:", intersects)
	intersects = blockartlib.XMLStringsIntersect(shape1.XMLString(), shape2.XMLString())
	fmt.Println("Shape XML strings intersect:", intersects)

	shape3, _ := parser.Parse(blockartlib.PATH, "M 0 0 l 100 100", "transparent", "black")
	fmt.Println(shape3.XMLString())

	shape4, _ := parser.Parse(blockartlib.CIRCLE, "4, 5, 3", "transparent", "black")
	fmt.Println(shape4.XMLString())

	shape5, _ := parser.ParseXMLString(shape3.XMLString())
	fmt.Println(shape5)

	shape6, _ := parser.ParseXMLString(shape4.XMLString())
	fmt.Println(shape6)

	return
}

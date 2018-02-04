package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

const EPSILON = 10E-6

var commands = map[string]int{"M": 2, "m": 2, "L": 2, "l": 2, "H": 1, "h": 1, "V": 1, "v": 1, "z": 0, "Z": 0}
var moveToCmds = []string{"M", "m", "Z", "z"}
var lineToCmds = []string{"L", "l", "H", "h", "V", "v"}

type Point2d struct {
	x, y float64
}

func (p Point2d) String() string {
	return "(" + strconv.FormatFloat(p.x, 'f', -1, 64) +
		", " + strconv.FormatFloat(p.y, 'f', -1, 64) + ")"
}

type Line2d struct {
	a, b Point2d
}

type Component []Point2d

type Components []Component

type SVGParser struct {
	components Components
}

func NewSVGParser() *SVGParser {
	parser := SVGParser{}
	parser.components = make(Components, 0)
	return &parser
}

func (p *SVGParser) Parse(svgString string) (Components, error) {
	var reader = strings.NewReader(svgString)
	var arg1, arg2 float64
	var lastPoint Point2d // defaults to the origin
	var components = make(Components, 0)
	var component = make(Component, 0)

	for cmd, err := getCommand(reader); err == nil; cmd, err = getCommand(reader) {
		if !isCommand(cmd) {
			return nil, errors.New("The command " + cmd + " is not a valid SVG command")
		}

		if contains(moveToCmds, cmd) && isClosedPath(component) {
			return components, errors.New("Cannot have multiple closed components in same path..")
		}

		if contains(moveToCmds, cmd) && len(component) != 0 {
			if cmd == "m" {
				lastPoint = component[len(component)-1]
			}
			if !(cmd == "Z" || cmd == "z") {
				components = append(components, component)
				component = make(Component, 0)
			}
		}

		if contains(lineToCmds, cmd) && len(component) == 0 {
			component = append(component, lastPoint)
		}

		numArgs := commands[cmd]
		switch numArgs {
		case 0: // cmd is z or Z
			if len(components) > 1 {
				return components, errors.New("Cannot have multiple closed components in same path!!")
			}
			if len(component) < 3 {
				fmt.Println(len(component))
				return components, errors.New("Cannot close path with fewer than three points")
			}
			if len(component) != 0 {
				component = append(component, component[0])
			}

		case 1: // cmd can be H, V, h, v
			arg1, err = getArg(reader)
			if err != nil {
				return nil, err
			}
			switch cmd {
			case "H":
				component = append(component, Point2d{arg1, lastPoint.y})
			case "V":
				component = append(component, Point2d{lastPoint.x, arg1})
			case "h":
				component = append(component, Point2d{lastPoint.x + arg1, lastPoint.y})
			case "v":
				component = append(component, Point2d{lastPoint.x, lastPoint.y + arg1})
			}

		case 2: // cmd can be M, m, L, l
			arg1, arg2, err = getArgs(reader)
			if err != nil {
				return nil, err
			}
			switch cmd {
			case "M":
				component = append(component, Point2d{arg1, arg2})
			case "m":
				component = append(component, Point2d{lastPoint.x + arg1, lastPoint.y + arg2})
			case "L":
				component = append(component, Point2d{arg1, arg2})
			case "l":
				component = append(component, Point2d{lastPoint.x + arg1, lastPoint.y + arg2})
			}
		}
		if len(component) != 0 {
			lastPoint = component[len(component)-1]
		}
	}
	components = append(components, component)
	return components, nil
}

func euclidDist(u, v Point2d) float64 {
	return math.Pow(u.x-v.x, 2) + math.Pow(u.y-v.y, 2)
}

func isClosedPath(path Component) bool {
	if len(path) < 3 {
		return false
	}
	return euclidDist(path[len(path)-1], path[0]) < EPSILON
}

func getCommand(reader *strings.Reader) (string, error) {
	var cmd string
	_, err := fmt.Fscanf(reader, "%s", &cmd)
	if err != nil {
		return "", err
	}
	return cmd, nil
}

func getArg(reader *strings.Reader) (float64, error) {
	var arg float64
	_, err := fmt.Fscanf(reader, "%f", &arg)
	if err != nil {
		return 0.0, err
	}
	return arg, nil
}

func getArgs(reader *strings.Reader) (float64, float64, error) {
	var arg1, arg2 float64
	_, err := fmt.Fscanf(reader, "%f %f", &arg1, &arg2)
	if err != nil {
		return 0.0, 0.0, err
	}
	return arg1, arg2, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func isCommand(cmd string) bool {
	_, exists := commands[cmd]
	return exists
}

func shapeArea(component Component) float64 {

	area := 0.0
	numcomponent := len(component)
	j := numcomponent - 1 // The last vertex is the 'previous' one to the first

	for i := 0; i < numcomponent; i++ {
		area += (component[j].x + component[i].x) * (component[j].y - component[i].y)
		j = i
	}
	return math.Abs(area / 2)
}

func pathArea(components Components) float64 {
	area := 0.0
	for _, line := range components.lines() {
		area += line.length()
	}
	return area
}

func (c Component) lines() (lines []Line2d) {
	for i := 0; i < len(c)-1; i++ {
		lines = append(lines, Line2d{c[i], c[i+1]})
	}
	return lines
}

func (c Components) lines() (lines []Line2d) {
	for _, comp := range c {
		lines = append(lines, comp.lines()...)
	}
	return lines
}

func (l Line2d) length() float64 {
	return math.Sqrt(math.Pow(l.b.x-l.a.x, 2) + math.Pow(l.b.y-l.a.y, 2))
}

func intersects(shape1, shape2 Components) bool {

	// !?!??
	return openIntersects(shape1, shape2)

}

func openIntersects(shape1, shape2 Components) bool {
	for _, line1 := range shape1.lines() {
	Inner:
		for _, line2 := range shape2.lines() {
			a := line1.a
			b := line1.b
			c := line2.a
			d := line2.b
			a11 := b.x - a.x
			a12 := c.x - d.x
			a21 := b.y - a.y
			a22 := c.y - d.y
			b1 := c.x - a.x
			b2 := c.y - a.x
			det := a11*a22 - a21*a12
			if math.Abs(det) < EPSILON {
				continue Inner
			}
			det1 := b1*a22 - b2*a12
			det2 := a11*b2 - a21*b1
			s := det1 / det
			t := det2 / det
			if 0 <= s && s <= 1.0 && 0 <= t && t <= 1.0 {
				return true
			}
		}
	}
	return false
}

func isOpen(c Components) bool {
	return !isClosed(c)
}

func isClosed(c Components) bool {
	if len(c) != 1 {
		return false
	}
	if isClosedPath(c[0]) {
		return true
	}
	return false
}

var examples = map[string]string{
	"a":     "M 2 1 h 1 v 1 h 1 v 1 h -1 v 1 h -1 v -1 h -1 v -1 h 1 z",
	"b":     "M 10 10 l 100 45 H 30 z",
	"c":     "M 37 17 v 15 H 14 V 17 z",
	"d":     "M 0 0 L 2 0 V 1 l 1 1 l -1 1 l -2 -2 z",
	"diag1": "M 1 1 L 3 3",
	"diag2": "M 1 3 L 3 1",
	"diag3": "M 3 2 L 4 3",
	"cross": "M 1 1 L 3 3 M 1 3 L 3 1",
	"sq2":   "M 2 2 h 2 v 2 h -2 z",
	"sq1":   "M 1 1 h 2 v 2 h -2 z",
	"h":     "h 1",
}

func main() {
	var svgString string
	if len(os.Args) > 1 {
		svgString = os.Args[1]
	} else {
		svgString = examples["a"]
	}
	_ = svgString

	parser := NewSVGParser()
	components, _ := parser.Parse(examples["sq1"])
	other, _ := parser.Parse(examples["sq2"])

	fmt.Println(components)
	fmt.Println(other)

	//fmt.Println(shapeArea(components[0]))
	fmt.Println(pathArea(components))
	fmt.Println(shapeArea(components[0]))
	fmt.Println(intersects(components, other))
}

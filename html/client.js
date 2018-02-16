window.onload = function(){
    document.getElementById("canvas").addEventListener("click", clickedCanvas)
    document.addEventListener('keypress', keyPressed)
    scaleCanvas()
    sizeKeyTA()
    getBlockChain()
}
function sizeKeyTA() {
    document.getElementById("keyTA").style.height = document.getElementById("keyTA").scrollHeight+'px';
}
function scaleCanvas() {
    canvas = document.getElementById("canvas")
    ctx = canvas.getContext("2d")
    cvsWidth = canvas.width
    cvsHeight = canvas.height
    ctx.scale(cvsWidth/1024, cvsHeight/1024)

}
function scaleX(x) {
    canvas = document.getElementById("canvas")
    ctx = canvas.getContext("2d")
    cvsWidth = canvas.width
    return x * 1024 / cvsWidth
}
function scaleY(y) {
    canvas = document.getElementById("canvas")
    ctx = canvas.getContext("2d")
    cvsHeight = canvas.height
    return y * 1024 / cvsHeight
}
function keyPressed(event) {
    if (event.keyCode == 13) {
        submitDrawRequest()
    }
}
function clickedCanvas(event) {
    var x = event.clientX
    var y = event.clientY
    canvas = document.getElementById("canvas")
    x -= canvas.offsetLeft;
    y -= canvas.offsetTop;
    x = scaleX(x).toFixed()
    y = scaleY(y).toFixed()
    var command
    var type = document.getElementById("shapetype").value
    if (type === "Path") {
        command = "M " + x + " " + y
    } else {
        command = x + ", " + y + ", "
    }
    var dinput = document.getElementById("dinput")
    var val = dinput.value
    dinput.value = val + " " + command
}
function clearDinput() {
    document.getElementById("dinput").value = ""
}
function receiveBlock(event) {
    var command = JSON.parse(event.data)
    console.log("Received command from webserver")
    drawCommand(command)
}
function setShape(shape) {
    var d = ""
    switch (shape) {
        case 'square':
            d = "h 50 v 50 h -50 z"
            break;
        case 'triangle':
            d = "h 50 l -25 -43 z"
            break;
        case 'smiley': 
            d = "v 18 h 50 v -18 m -33 -12.5 l 0 -18 m 15 0 l 0 18"
            break;
        case 'cross':
            d = "l 50 50 m -50 0 l 50 -50"
            break;
    }
    var dinput = document.getElementById("dinput")
    var val = dinput.value
    dinput.value = val + " " + d
}
function drawCommand(command) {
    d = command.SVGString
    var canvas = document.getElementById('canvas');
    var ctx = canvas.getContext('2d');
    var p = new Path2D(d);
    ctx.fillStyle = command.Fill
    ctx.strokestyle = command.Stroke
    ctx.stroke(p);
    ctx.fill(p)
}
function deleteCommand(command) {

}
function submitDrawRequest() {
    var SVGString = document.getElementById("dinput").value;
    var stroke = document.getElementById("stroke").value;
    var fill = document.getElementById("fill").value;
    var shapetype = document.getElementById("shapetype").value;
    var key = document.getElementById("keyTA").value;
    var addr = document.getElementById("serveraddr").value;
    var request = { "SVGString": SVGString,
                    "Stroke": stroke,
                    "Fill": fill,
                    "ShapeType": shapetype,
                    "Key": key, 
                    "Addr": addr, }
     post(request)
}
function post(shape) {
    fetch("http://localhost:8080/draw",
            {method: "POST",
            body: JSON.stringify(shape)},
        ).then(function(res){ res.text().then(function(data){
                var response = JSON.parse(data)
                if (response["Status"] != "OK") {
                    alert(response["Status"])
                    return
                }
                console.log(response)
                drawInput()
                clearDinput()
            })})
        .catch(function(res){ console.log(res) })
}
function drawInput() {
    var SVGString = document.getElementById("dinput").value;
    var stroke = document.getElementById("stroke").value;
    var fill = document.getElementById("fill").value;
    var shapetype = document.getElementById("shapetype").value;

    var canvas = document.getElementById('canvas');
    var ctx = canvas.getContext('2d');
    ctx.fillStyle = fill
    ctx.strokeStyle = stroke

    if (shapetype == "Path") {    
        var p = new Path2D(SVGString);    
        ctx.stroke(p)
        ctx.fill(p)
    } else if (shapetype == "Circle") {
        ctx.beginPath();
        args = SVGString.split(",")
        cx = parseInt(args[0])
        cy = parseInt(args[1])
        r = parseInt(args[2])
        ctx.arc(cx, cy, r, 0, 2 * Math.PI);                
        ctx.stroke()
        ctx.fill()
    }
}

function getBlockChain() {
    var uri = "http://" + window.location.hostname + ":8080/blocks"
    fetch(uri).then(function(response) {
        return response.json()
    }).then(function(data) {
        var current = data.Genesis;
        var queue = [];
        queue.push({id: current, parentId: ""});
        var flattened = [];
        while (queue.length > 0) {
            var block = queue.shift();
            var children = data.Blocks[block.id];
            if (children != null) {
                for (var i = 0; i < children.length; i++) {
                    queue.push({id: children[i], parentId: block.id});
                }
            }
            flattened.push(block);
        }

        var root = d3.stratify()(flattened);
        console.log(root);
        var tree = d3.tree();
        tree.size([1000, 1000]);
        tree(root);
        console.log(tree)

        // TODO: make this work
    });
}

var svg = d3.select("#tree").append("svg")
    .attr("width", 1000)
    .attr("height", 1000);
var g = svg.append("g");
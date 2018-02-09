window.onload = function(){
    var exampleSocket = new WebSocket("ws://127.0.0.1:8080/registerws")
    exampleSocket.onmessage = receiveBlock
    document.getElementById("theCanvas").addEventListener("click", clickedCanvas)
    document.addEventListener('keypress', keyPressed)
}
function keyPressed(event) {
    if (event.keyCode == 13) {
        submitDrawRequest()
    }
}
function clickedCanvas(event) {
    var x = event.clientX
    var y = event.clientY
    canvas = document.getElementById("theCanvas")
    x -= canvas.offsetLeft;
    y -= canvas.offsetTop;
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
            d = "h 20 v 20 h -20 z"
            break;
        case 'triangle':
            d = "h 20 l -10 -17 z"
            break;
        case 'smiley': 
            d = "v 7 h 20 v -7 m -13 -5 l 0 -7 m 6 0 l 0 7"
            break;
        case 'cross':
            d = "l 20 20 m -20 0 l 20 -20"
            break;
    }
    var dinput = document.getElementById("dinput")
    var val = dinput.value
    dinput.value = val + " " + d
}
function drawCommand(command) {
    d = command.SVGString
    var canvas = document.getElementById('theCanvas');
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
                if (response["Status"] == "OK") {
                    console.log("Status is ok")
                    drawInput()
                }
            })})
        .catch(function(res){ console.log(res) })
}
function drawInput() {
    var SVGString = document.getElementById("dinput").value;
    var stroke = document.getElementById("stroke").value;
    var fill = document.getElementById("fill").value;
    var shapetype = document.getElementById("shapetype").value;

    var canvas = document.getElementById('theCanvas');
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
        ctx.arc(cx, cy, r, 0,2 * Math.PI);                
        ctx.stroke()
        ctx.fill()
    }
}
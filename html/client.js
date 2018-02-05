window.onload = function(){
    var exampleSocket = new WebSocket("ws://127.0.0.1:8080/registerws")
    exampleSocket.onmessage = function (event) {
        var command = JSON.parse(event.data)
        console.log("Received command from webserver")
        drawCommand(command)
        }
}
function drawCommand(command) {
    d = command.SVGString
    var canvas = document.getElementById('myCanvas');
    var ctx = canvas.getContext('2d');
    var p = new Path2D(d);
    ctx.fillStyle = command.Fill
    ctx.strokestyle = command.Stroke
    ctx.stroke(p);
    ctx.fill(p)
}
function submitDrawRequest() {
    var SVGString = document.getElementById("dinput").value;
    var stroke = document.getElementById("stroke").value;
    var fill = document.getElementById("fill").value;
    var shapetype = document.getElementById("shapetype").value;
    console.log(SVGString)
    console.log(stroke)
    console.log(fill)
    console.log(shapetype)
    var shape = {"SVGString": SVGString,
     "Stroke": stroke,
     "Fill": fill,
     "ShapeType": shapetype}
     post(shape)
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

    var canvas = document.getElementById('myCanvas');
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
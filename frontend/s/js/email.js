'use strict';

window.addEventListener('load', function() {
    var domain = location.hostname
    var path = location.pathname
    var email = path.substring(path.indexOf('/c/')+3)
    var ws = new WebSocket('wss://'+domain+'/ws/c/'+email);
    ws.onmessage = function(evt) {
        if (JSON.parse(evt.data).Data.Success) {
            document.getElementsByClassName('info')[0].style.display = 'none'
            document.getElementsByClassName('complete')[0].style.display = 'block'
            setTimeout(function() { window.location.pathname="/" }, 5000)
        } else {
            document.getElementsByClassName('info')[0].style.display = 'none'
            document.getElementsByClassName('error')[0].style.display = 'block'
        }
        ws.close()
    }
    ws.onerror = function() {
        document.getElementsByClassName('info')[0].style.display = 'none'
        document.getElementsByClassName('error')[0].style.display = 'block'
    }
})

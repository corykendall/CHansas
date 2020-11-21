'use strict';

const ctypeUpdatePassword = 14;

window.addEventListener('load', function() {
    var path = location.pathname
    var email = path.substring(path.indexOf('/c/')+3)
    var ws = new WebSocket('wss://'+location.hostname+'/ws/r/'+email);
    ws.onmessage = function(evt) {
        if (JSON.parse(evt.data).Data.Success) {
            document.getElementsByClassName('reset-loader')[0].style.display = 'none'
            document.getElementsByClassName('complete')[0].style.display = 'block'
            setTimeout(function() { window.location.pathname="/" }, 5000)
        } else {
            document.getElementsByClassName('reset-loader')[0].style.display = 'none'
            document.getElementsByClassName('error')[0].style.display = 'block'
        }
        ws.close()
    }
    ws.onerror = function() {
        document.getElementsByClassName('reset-loader')[0].style.display = 'none'
        document.getElementsByClassName('error')[0].style.display = 'block'
    }

    document.getElementsByClassName('reset-button')[0].onclick = function () {
        var p = document.getElementsByClassName('reset-password1')[0].value
        var p2 = document.getElementsByClassName('reset-password2')[0].value
        var localerror = document.getElementsByClassName('localerror')[0]

        if (p != p2) {
            localerror.innerHTML = "Passwords don't match"
            localerror.style.display = 'block'
            return
        }
        if (p.length < 4) {
            localerror.innerHTML = "Password too short"
            localerror.style.display = 'block'
            return
        }
        if (p.length > 32) {
            localerror.innerHTML = "Password too long"
            localerror.style.display = 'block'
            return
        }

        localerror.style.display = 'none'
        document.getElementsByClassName('reset-password1')[0].disabled = true
        document.getElementsByClassName('reset-password2')[0].disabled = true
        document.getElementsByClassName('reset-button')[0].disabled = true
        document.getElementsByClassName('reset-password1')[0].disabled = true
        document.getElementsByClassName('reset-loader')[0].style.display = 'block'
        ws.send('{"CType":'+ctypeUpdatePassword+',"Data":{"Password":"'+p+'"}}')
    }
})

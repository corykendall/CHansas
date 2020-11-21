'use strict';

var ws 
var wsEverActive = false
window.addEventListener('load', function() {
    var path = location.pathname
    ws = new WebSocket('wss://'+location.hostname+'/ws')
    ws.onmessage = function(evt) {
        wsEverActive = true
        printMsg('RESPONSE: ' + evt.data);
        var msg = JSON.parse(evt.data);
        if (msg.SType == stypeYourIdentity) {
            setIdentity(msg.Data)
        } else if (msg.SType == stypeNotifySignup) {
            handleNotifySignup(msg.Data)
        } else if (msg.SType == stypeNotifySignin) {
            handleNotifySignin(msg.Data)
        } else if (msg.SType == stypeNotifyPasswordReset) {
            handleNotifyPasswordReset(msg.Data)
        } else if (msg.SType == stypeNotifyNotification) {
            handleNotifyNotification(msg.Data)
        }
    };
    ws.onclose = function(evt) {
        closeOverlay(wsEverActive)
        ws = null;
    };
    highlightFragment()
})

// Apply a highlight to a single question and answer based on the URL Fragment.
function highlightFragment() {
    var hash = window.location.hash
    if (hash == '') {
        return
    }

    var el = getEl('about-'+hash.substring(1).toLowerCase())
    if (el != null) {
        el.style['background-color'] = 'rgb(40,40,0)'
        el.nextElementSibling.style['background-color'] = 'rgb(60,60,0)'
        el.scrollIntoView(true)
    }
}

function printMsg(msg) {
  console.log(msg)
}

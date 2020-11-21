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
})

function printMsg(msg) {
  console.log(msg)
}

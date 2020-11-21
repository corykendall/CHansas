'use strict';

var ws
var wsEverActive = false

window.addEventListener('load', function() {

    // Request more information
    ws = new WebSocket('wss://'+location.hostname+'/ws/lobby');
    ws.onopen = function(evt) {
        printMsg('OPEN');
    };
    ws.onclose = function(evt) {
        printMsg('CLOSE')
        closeOverlay(wsEverActive)
        ws = null;
    };
    ws.onmessage = function(evt) {
        wsEverActive = true
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
        } else if (msg.SType == stypeNotifyLobby) {
            refreshLobby(msg.Data)
        } else if (msg.SType == stypeNotifyCreateGame) {
            window.location.href = 'https://'+location.hostname+'/g/'+msg.Data.Id
        }
    };
    ws.onerror = function(evt) {
        printMsg('ERROR: ' + evt.data);
    };

    getEl('create-button').onclick = sendCreateGame
});

function refreshLobby(d) {
    getEl('lobby-playerstats-activecount').innerHTML = d.Players
    getEl('lobby-playerstats-observerscount').innerHTML = d.Observers

    var trClass = 'dark'
    var html='<table class="lobbytable"><thead>'+
        '<tr>'+
            '<th>Id</th>'+
            '<th>Status</th>'+
            '<th>Creator</th>'+
            '<th>Running</th>'+
            '<th>Completed</th>'+
            '<th>Players</th>'+
        '</tr></thead>'
    if (d.Games == null || d.Games.length == 0) {
        html+='<tr><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td></tr>'
    } else {
        d.Games.forEach(function(g, i) {
            if (trClass == 'dark') {
                trClass = 'light'
            } else {
                trClass = 'dark'
            }
            var running = '-'
            if (new Date(g.RunningTime) > 100) {
                running = new Date(g.RunningTime).toLocaleString()
            }
            var complete = '-'
            if (new Date(g.CompleteTime) > 100) {
                complete = new Date(g.CompleteTime).toLocaleString()
            }
            html+= '<tr class="'+trClass+'">'+
                '<td><a href="/g/'+g.Id+'">'+g.Id+'</a></td>'+
                '<td>'+gameStatusNames[g.Status]+'</td>'+
                '<td>'+g.Creator.Name+'</td>'+
                '<td>'+running+'</td>'+
                '<td>'+complete+'</td>'+
                '<td>'
            g.Players.forEach(function (p, i2) {
                var color = pcToS(g.Colors[i2])
                var score = g.Scores[i2]
                var name = p.Name || 'Empty'
                html+='<div class="playername-'+color+'">'+name
                if (name != 'Empty') {
                    html+=':&nbsp;&nbsp;&nbsp;'+score
                }
                html+='</div>'
            })
            html+='</td></tr>'
        });
    }
    html+='</table>'
    getEl('lobby').innerHTML=html
}

function printMsg(msg) {
    console.log(msg)
}

function sendCreateGame() {
    if (!ws) {
        return false;
    }
    var msg = '{"CType":'+ctypeCreateGame+',"Data":{}}';
    printMsg('SEND: '+msg);
    ws.send(msg);
    return false;
}


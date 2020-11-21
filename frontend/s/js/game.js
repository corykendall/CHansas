'use strict';

var status
var creator
var iAmCreator
var table
var turnstate
var scores

var playerMe = -1
var gameEl
var boardEl
var playerBoardEls
var turnTimer
var turnTimerPlayer = -1

var pieces
var nonePiece = {PlayerColor: playerColorNone, Shape: shapeNone}
var dragged = {}

var pendingSubactions = []

var ws
var wsEverActive = false
window.addEventListener('load', function() {
    var path = location.pathname
    var gameId = location.pathname.split("/")[2]
    document.title = 'Game: '+gameId;
    ws = new WebSocket('wss://'+location.hostname+'/ws/g/'+gameId);
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
        } else if (msg.SType == stypeNotifyFullGame) {
            handleNotifyFullGame(msg.Data)
        } else if (msg.SType == stypeNotifySitdown) {
            handleNotifySitdown(msg.Data)
        } else if (msg.SType == stypeNotifyStartGame) {
            handleNotifyStartGame(msg.Data)
        } else if (msg.SType == stypeNotifyNextTurn) {
            handleNotifyNextTurn(msg.Data)
        } else if (msg.SType == stypeNotifySubaction) {
            handleNotifySubaction(msg.Data)
        } else if (msg.SType == stypeNotifySubactionError) {
            handleNotifySubactionError(msg.Data)
        } else if (msg.SType == stypeNotifyEndBump) {
            handleNotifyEndBump(msg.Data)
        } else if (msg.SType == stypeNotifyScoringBegin) {
            handleNotifyScoringBegin(msg.Data)
        } else if (msg.SType == stypeNotifyEndgameScoring) {
            handleNotifyEndgameScoring(msg.Data)
        } else if (msg.SType == stypeNotifyComplete) {
            handleNotifyComplete(msg.Data)
        } else {
            printMsg('unhandled stype: '+msg.SType+' data: '+msg.Data)
        }
    };
    ws.onclose = function(evt) {
        closeOverlay(wsEverActive)
        ws = null;
    };

    document.onmousedown = grabPiece

    setInterval(stopwatch, 1000)

    // This is some temp shit
    /*
    dragElement(getEl('locationfinderhelp'))
    dragElement(getEl('locationfinderhelp-disc'))
    document.onkeyup = function (e) {
        if (e.keyCode == 67) {
            var el = getEl('locationfinderhelp')
            console.log('    .ls-1-0-0-0 {top: '+el.style.top+'; left: '+el.style.left+';}    ')
        }
        if (e.keyCode == 68) {
            var el = getEl('locationfinderhelp-disc')
            console.log('DSK .ls-1-0-0-0 {top: '+el.style.top+'; left: '+el.style.left+';}    ')
        }
    }
    */
})

function handleNotifySitdown(d) {
    var i = {Type: identityTypeNone, Name: "", Id: ""}
    if (d.Sitdown) {
        i = d.Identity
    }
    table.PlayerBoards[d.Index].Identity = i

    renderBoard()
    renderPlayerBoardIdentity(d.Index)
    renderPlayerBoardButtons()
}

function renderPlayerBoardIdentity(i) {
    getChild(getEl('playerboard-'+i), 'player-identity').innerHTML =
        table.PlayerBoards[i].Identity.Name
}

function renderPlayerBoardButtons() {
    var sitting = false
    table.PlayerBoards.forEach(function (pb) {
        if (pb.Identity.Id == getIdentity()) {
            sitting = true
        }
    })

    table.PlayerBoards.forEach(function (pb, i) {
        var html = ''
        var el = playerBoardEls[i]
        if (pb.Identity.Id == getIdentity()) {
            if (hasChild(el, 'playerboard-creating')) {
                getChild(el, 'playerboard-creating').remove()
            }
            if (!hasChild(el, 'playerboard-standup')) {
                html+='<button onclick="clickLeave(this)" data-i="'+i+'" '+
                    'class="big-button playerboard-standup">Leave</button>'
            }
        } else if (pb.Identity.Type == identityTypeNone) {
            if (hasChild(el, 'playerboard-standup')) {
                getChild(el, 'playerboard-standup').remove()
            }

            // Redraw because the inner sitdown buttons change when I take another seat.
            if (hasChild(el, 'playerboard-creating')) {
                getChild(el, 'playerboard-creating').remove()
            }
            html += '<div class="playerboard-creating">'
            if (!sitting) {
                html+='<button onclick="clickSitHere(this)" data-i="'+i+'" '+
                    'class="big-button playerboard-sitdown">Sit Here</button>'
            }
            if (iAmCreator) {
                html+='<button onclick="clickAddBot(this)" data-i="'+i+'" '+
                    'class="big-button playerboard-addbot">Add Bot</button>'
            }
            html+='</div>'
        } else if (pb.Identity.Type == identityTypeBot) {
            if (hasChild(el, 'playerboard-creating')) {
                getChild(el, 'playerboard-creating').remove()
            }
            if (iAmCreator) {
                if(!hasChild(el, 'playerboard-standup')) {
                    html+='<button onclick="clickLeaveBot(this)" data-i="'+i+'" '+
                        'class="big-button playerboard-standup">Leave</button>'
                }
            } else {
                if (hasChild(el, 'playerboard-standup')) {
                    getChild(el, 'playerboard-standup').remove()
                }
            }
        } else if (pb.Identity.Type == identityTypeConnection || pb.Identity.Type == identityTypeGuest) {
            if (hasChild(el, 'playerboard-creating')) {
                getChild(el, 'playerboard-creating').remove()
            }
            if (hasChild(el, 'playerboard-standup')) {
                getChild(el, 'playerboard-standup').remove()
            }
        }

        if (html != '') {
            var div = document.createElement('div')
            div.innerHTML=html
            el.appendChild(div.children[0])
        }
    })
}

function handleNotifyStartGame(d) {
    status = gameStatusRunning
    table = d.Table
    renderBoards()
    renderAllPieces()
    table.PlayerBoards.forEach(function (pb, i) {
        if (pb.Identity.Id == getIdentity()) {
            playerMe = i
        }
    })
}

function renderTurnEl(d) {
    turnstate = d
    var turnEl = getChild(playerBoardEls[d.Player], 'playerturn')
    if (!turnEl.classList.contains('currentturn')) {
        var oldTurnEl = getEl('currentturn')
        if (oldTurnEl != null) {
            oldTurnEl.innerHTML = getChild(oldTurnEl, 'player-elapsed').outerHTML
            oldTurnEl.classList.remove('currentturn')
        }
        turnEl.classList.add('currentturn')
    } 
    var elapsedEl = getChild(turnEl, 'player-elapsed')

    var html = '<div class="icon iconaction"></div>'+
        '<div class="player-actions">'+d.ActionsLeft+'</div>'+
        '<div class="player-turntime stopwatch-active">00:00</div>'+
        elapsedEl.outerHTML

    if (d.Type == turnStateTypeBags) {
        html+='<div class="player-turnstate">Bags</div>'
        if (d.BagsLeft > 20) {
            html+='<div class="player-turnstatehelp">(All)</div>'
        } else {
            html+='<div class="player-turnstatehelp">('+d.BagsLeft+')</div>'
        }
    }
    if (d.Type == turnStateTypeMoving) {
        html+='<div class="player-turnstate">Move</div>'+
            '<div class="player-turnstatehelp">('+d.MovesLeft+')</div>'
    }
    if (d.Type == turnStateTypeBumpPaying) {
        html+='<div class="player-turnstate">Bump</div>'+
            '<div class="player-turnstatehelp">Pay'+d.BumpPayingCost+'</div>'
    }

    if (d.Type == turnStateTypeClearing) {
        html+='<div class="player-turnstate">Clear</div>'
        if (d.ClearingCanOffice && d.ClearingAward != awardNone) {
            html+='<div class="player-turnstatehelp">City<br>or<br>'+
                '<div class="'+awardToIcon(d.ClearingAward)+'"></div></div>'
        } else if (d.ClearingCanOffice) {
            html+='<div class="player-turnstatehelp">City</div>'
        } else if (d.ClearingAward != awardNone) {
            html+='<div class="player-turnstatehelp">'+
                '<div class="'+awardToIcon(d.ClearingAward)+'"></div></div>'
        }
    }
    if (d.Type == turnStateTypeBumping) {
        html+='<div class="player-turnstate">Wait</div>'
        var bumpedTurnEl = getEl('bumpedturn')
        if (bumpedTurnEl == null) {
            bumpedTurnEl = getChild(playerBoardEls[d.BumpingPlayer], 'playerturn')
            bumpedTurnEl.classList.add('bumpedturn')
        }
        var bHtml = '<div class="player-turnstate">Bump</div>'+
            getChild(bumpedTurnEl, 'player-elapsed').outerHTML+
            '<div class="player-turnstatehelp">'
        if (!d.BumpingMoved) {
            bHtml+='Move<br>'
        }
        if (d.BumpingReplaces > 0) {
            bHtml+='+'+d.BumpingReplaces
        }
        bumpedTurnEl.innerHTML = bHtml+'</div>'
    } else {
        var bumpedTurnEl = getEl('bumpedturn')
        if (bumpedTurnEl != null) {
            bumpedTurnEl.innerHTML = getChild(bumpedTurnEl, 'player-elapsed').outerHTML
            bumpedTurnEl.classList.remove('bumpedturn')
        }
    }

    turnEl.innerHTML = html

    if (d.Type == turnStateTypeBumping && table.PlayerBoards[d.BumpingPlayer].Identity.Id == getIdentity()) {
        var bumpButtonsEl = document.createElement('div')
        bumpButtonsEl.classList.add('turnbuttons')
        bumpButtonsEl.innerHTML = 
            '<button class="undo-button big-button" onclick="sendUndo()"> Undo </button>'+
            '<button class="endturn-button big-button" onclick="sendEndbump()">Done</button>'
        bumpedTurnEl.appendChild(bumpButtonsEl)
    }

    if (table.PlayerBoards[d.Player].Identity.Id == getIdentity()) {
        var turnButtonsEl = document.createElement('div')
        turnButtonsEl.classList.add('turnbuttons')
        turnButtonsEl.innerHTML = 
            '<button class="undo-button big-button" onclick="sendUndo()"> Undo </button>'+
            '<button class="endturn-button big-button" onclick="sendEndturn()">Done</button>'
        turnEl.appendChild(turnButtonsEl)
    }
}

function renderElapsed(d) {
    d.forEach(function (e, i) {
        getChild(playerBoardEls[i], 'player-elapsed').innerHTML = goDurationToMinutesHours(e)
    })
}

function handleNotifyNextTurn(d) {
    renderTurnEl(d.TurnState)
    renderElapsed(d.Elapsed)
}

function handleNotifySubaction(d) {
    if (pendingSubactions.length > 0) {
        pendingSubactions.pop()
    } else {
        renderSubaction(d.Subaction)
    }

    renderTurnEl(d.TurnState)
    renderScoreDelta(d.Scores)
}

function renderScoreDelta(s) {
    playerBoardEls.forEach(function (pb, i) {
        if (s[i] != 0) {
            var div = document.createElement('div')
            div.classList.add('score-animation')
            div.innerHTML = '+'+s[i]
            var score = s[i]
            var scoreEl = getChild(pb, 'player-score')
            pb.appendChild(div)
            setTimeout(function () {
                div.remove()
                scoreEl.innerHTML = parseInt(scoreEl.innerHTML)+score
            }, 3000)
        }
    })
}

function handleNotifySubactionError(d) {
    d.Type = notificationError
    handleNotifyNotification(d)
    var bad = pendingSubactions.pop()
    renderSubaction({
        Source: bad.Dest,
        Dest: bad.Source,
        Piece: bad.Piece,
        Token: bad.Token
    })
}

function handleNotifyEndBump(d) {
    renderTurnEl(d.TurnState)
    renderElapsed(d.Elapsed)
}

function handleNotifyScoringBegin(d) {
    status = gameStatusScoring
    playerBoardEls.forEach(function (pb, i) {
        var div = document.createElement('div')
        div.classList.add('player-endgame-scoring')
        div.classList.add('pes-'+i)
        div.classList.add('pes-'+pcToS(table.PlayerBoards[i].Color))
        div.innerHTML = 
            '<div class="pes-game">Game:</div><div class="pes-1-value"></div>'+
            '<div class="pes-board">Board:</div><div class="pes-2-value"></div>'+
            '<div class="pes-coellen">Coellen:</div><div class="pes-3-value"></div>'+
            '<div class="pes-control">Control:</div><div class="pes-4-value"></div>'+
            '<div class="pes-network">Network:</div><div class="pes-5-value"></div>'+
            '<div class="pes-total">Total:</div><div class="pes-6-value"></div>'+
            '<div class="pes-place"></div>'
        gameEl.appendChild(div)
    })
}

function handleNotifyEndgameScoring(d) {
    if (d.Type != 7) {
        var pes = getEl('pes-'+d.Player)
        getChild(pes, 'pes-'+d.Type+'-value').innerHTML = d.Score
        if (d.Type == 6) {
            return
        }
        var div = document.createElement('div')
        div.classList.add('finalscore-animation')
        div.innerHTML = '+'+d.Score
        pes.appendChild(div)
        var scoreEl = getChild(playerBoardEls[d.Player], 'player-score')
        var score = d.Score
        var type = d.Type
        setTimeout(function() {
            div.remove()
            if (type != 1) {
                scoreEl.innerHTML = parseInt(scoreEl.innerHTML)+d.Score
            }
        }, 2000)
        return
    }
    var place = d.Score+1
    if (place == 1) {
        place +='st'
    } else if (place == 2) {
        place +='nd'
    } else if (place == 3) {
        place +='rd'
    } else if (place == 4) {
        place +='th'
    } else if (place == 5) {
        place +='th'
    }
    getChild(getEl('pes-'+d.Player), 'pes-place').innerHTML = place
}

function handleNotifyComplete(d) {
    status = gameStatusComplete
}

// This has to look inside groups
function pieceAtLocation(l) {
    var ls = lToS(l)
    if (!lsIsInsideGroup(ls)) {
        return elToP(getEl(ls).children[0])
    }
    return elToP(getEl('piece'+ls.substring(2)))
}

// This should swap, bump, or unbump
function renderSubaction(d) {
    var destP = pieceAtLocation(d.Dest)
    if (destP.PlayerColor == playerColorNone) {

        // Default case
        renderPiece(lToS(d.Source), nonePiece)
        renderPiece(lToS(d.Dest), d.Piece)

        // Unbump
        if (d.Source.Type == locationTypeRoute && d.Source.Subindex == 0) {
            var bumpedLocation = {
                Type: d.Source.Type,
                Id: d.Source.Id,
                Index: d.Source.Index,
                Subindex: 1
            }
            var bumpedP = pieceAtLocation(bumpedLocation)
            if (bumpedP.PlayerColor != playerColorNone) {
                renderPiece(lToS(bumpedLocation), nonePiece)
                renderPiece(lToS(d.Source), bumpedP)
            }
        }
        return
    }

    // TODO: Check moving a bumped piece into antoher route's piece, or another route's bump zone

    if (d.Dest.Type == locationTypeRoute && d.Dest.Subindex == 0) {
        var bumpedLocation = {
            Type: d.Dest.Type,
            Id: d.Dest.Id,
            Index: d.Dest.Index,
            Subindex: 1
        }
        var bumpedP = pieceAtLocation(bumpedLocation)

        // bump
        if (bumpedP.PlayerColor == playerColorNone) {
            renderPiece(lToS(d.Source), nonePiece)
            renderPiece(lToS(d.Dest), d.Piece)
            renderPiece(lToS(bumpedLocation), destP)
            return
        }

        // If we are here, player tried to bump but there is something in the
        // bump zone.  Fallback to a normal swap (this will end up being
        // illegal server side, and we will undo it).
    }

    // swap
    renderPiece(lToS(d.Source), destP)
    renderPiece(lToS(d.Dest), d.Piece)
}

// The second message received from the server (after YourIdentity).  We build
// all initial state now, and everything else is diff stacking.
function handleNotifyFullGame(d) {
    gameEl = getEl('game')
    boardEl = getEl('board')

    status = d.Status
    creator = d.Creator
    iAmCreator = d.Creator.Id == getIdentity()
    table = d.Table
    scores = d.Scores
    renderBoards()
    renderAllPieces()

    if (status == gameStatusRunning) {
        table.PlayerBoards.forEach(function (pb, i) {
            if (pb.Identity.Id == getIdentity()) {
                playerMe = i
            }
        })
        renderTurnEl(d.TurnState)
    }
    renderElapsed(d.Elapsed)

    if (status == gameStatusScoring || status == gameStatusComplete) {
        handleNotifyScoringBegin(d)
        renderFinalScores(d.FinalScores)
    }
}

function renderFinalScores(d) {
    d.forEach(function (ps, i) {
        for (const s in ps) {
            if (s != 7) {
                getChild(getEl('pes-'+i), 'pes-'+s+'-value').innerHTML=ps[s]
            } else {
                var place = ps[s]+1
                if (place == 1) {
                    place +='st'
                } else if (place == 2) {
                    place +='nd'
                } else if (place == 3) {
                    place +='rd'
                } else if (place == 4) {
                    place +='th'
                } else if (place == 5) {
                    place +='th'
                }
                getChild(getEl('pes-'+i), 'pes-place').innerHTML = place
            }
        }
    })
}

function renderBoard() {
    if (status != gameStatusCreating) {
        if (hasChild(boardEl, 'board-creating')) {
            getChild(boardEl, 'board-creating').remove()
        }
        return
    }

    var sitting = false
    var players = 0
    table.PlayerBoards.forEach(function (pb) {
        if (pb.Identity.Id == getIdentity()) {
            sitting = true
        }
        if (pb.Identity.Type != identityTypeNone) {
            players++
        }
    })

    if (iAmCreator) {
        //if (players < 4 || !sitting) {
        if (players < 4) {
            boardEl.innerHTML='<div class="board-creating">'+
                '<button onclick="clickStart()" class="big-button" disabled>Start Game</button>'+
                '<br>Need creator seated and  4+ players...</div>'
        } else {
            boardEl.innerHTML='<div class="board-creating">'+
                '<button onclick="clickStart()" class="big-button">Start Game</button></div>'
        }
    } else {
        boardEl.innerHTML='<div class="board-creating">Waiting for '+creator.Name+' to start...</div>'
    }
}

function renderBoards() {
    renderBoard()

    var key = [1, 2, 2, 3, 4]
    var priviledge = ['white', 'orange', 'purple', 'black']
    var book = [2, 3, 4, 5]
    var action = [2, 3, 3, 4, 4, 5]
    var bag = ['3', '5', '7', 'C']

    var sitting = false
    table.PlayerBoards.forEach(function (pb) {
        if (pb.Identity.Id == getIdentity()) {
            sitting = true
        }
    })

    playerBoardEls = []
    var playerBoardsEl = getEl('playerboards')
    playerBoardsEl.innerHTML = ''
    table.PlayerBoards.forEach(function (pb, i) {
        var c = pcToS(pb.Color)
        var html='<div class="playerboard playerboard-'+i+' playerboard-'+c+'">'+
            '<div class="playerturn playerturn-'+c+'">'+
                '<div class="player-elapsed">00:00</div>'+
            '</div>'+
            '<div class="track key-track"><div class="icon iconkey"></div>'
        key.forEach(function (v) {
            html+='<div class="trackspot">'+v+'</div>'
        })
        html+='</div><div class="track priviledge-track"><div class="icon iconpriviledge"></div>'
        priviledge.forEach(function (v) {
            html+='<div class="trackspot '+v+'-priviledge"></div>'
        })
        html+='</div><div class="track book-track"><div class="icon iconbook"></div>'
        book.forEach(function (v) {
            html+='<div class="trackspot disc-trackspot">'+v+'</div>'
        })
        html+='</div><div class="track action-track"><div class="icon iconaction"></div>'
        action.forEach(function (v) {
            html+='<div class="trackspot">'+v+'</div>'
        })
        html+='</div><div class="track bag-track"><div class="icon iconbag"></div>'
        bag.forEach(function (v) {
            html+='<div class="trackspot">'+v+'</div>'
        })
        html+='</div>'+
            '<div class="player-identity">'+pb.Identity.Name+'</div>'+
            '<div class="track stock-track">Stock:'+
                '<div class="trackspot"></div>'+
                '<div class="trackspot disc-trackspot"></div>'+
                '<div class="stock-cubecount">0</div>'+
                '<div class="stock-disccount">0</div>'+
            '</div>'+
            '<div class="player-supply"></div>'+
            '<div class="player-usedtokens"></div>'+
            '<div class="player-score">'+scores[i]+'</div>'

        if (status == gameStatusCreating) {
            if (pb.Identity.Id == getIdentity()) {
                html+='<button onclick="clickLeave(this)" data-i="'+i+'" '+
                    'class="big-button playerboard-standup">Leave</button>'
            } else if (pb.Identity.Type == identityTypeNone) {
                html += '<div class="playerboard-creating">'
                if (!sitting) {
                    html+='<button onclick="clickSitHere(this)" data-i="'+i+'" '+
                        'class="big-button playerboard-sitdown">Sit Here</button>'
                }
                if (iAmCreator) {
                    html+='<button onclick="clickAddBot(this)" data-i="'+i+'" '+
                        'class="big-button playerboard-addbot">Add Bot</button>'
                }
                html+='</div>'
            } else if (pb.Identity.Type == identityTypeBot && iAmCreator) {
                html+='<button onclick="clickLeaveBot(this)" data-i="'+i+'" '+
                    'class="big-button playerboard-standup">Leave</button>'
            }
        }
        html+='</div>'

        var div = document.createElement('div')
        div.innerHTML = html
        playerBoardEls.push(div.children[0])
        playerBoardsEl.appendChild(div.children[0])
    })
}

function renderAllPieces() {
    Array.from(getEls('location')).forEach(function (el) {
        el.remove()
    })
    buildPieces()
    for (const ls in pieces) {
        renderPiece(ls, pieces[ls])
    }
}

const lsStock = /ls-3-[0-4]-5-[0-9][0-9]?/
const lsSupply = /ls-3-[0-4]-6-[0-9][0-9]?/
function lsIsInsideGroup(ls) {
    return ls.match(lsStock) || ls.match(lsSupply)
}

function lsToGroupLs(ls) {
    var parts = ls.split('-')
    if (ls.match(lsStock)) {
        return 'ls-3-'+parts[2]+'-5'
    } else if (ls.match(lsSupply)) {
        return 'ls-3-'+parts[2]+'-6'
    }
    return ls
}

function lsIsMissingSubindex(ls) {
    var parts = ls.split('-')
    return parts.length == 4
}

function groupLsToLs(ls, subindex) {
    return ls+'-'+subindex
}

// For Groups, this assumes that initial render will render them all in
// subindex order.
function renderPiece(ls, piece) {
    var parts = ls.split('-')
    var discRouteShift = ' '
    if (piece.Shape == shapeDisc && parts[1] == locationTypeRoute) {
        discRouteShift = ' discshift '
    }

    var html = '<div data-subindex='+parts[4]+' class="piece '+
        'piece-'+ls.substring(3)+' '+
        discRouteShift+
        'piece-'+pcToS(piece.PlayerColor)+' '+
        'piece-'+shapeToS(piece.Shape)+'"></div>'

    var group = lsIsInsideGroup(ls)
    if (group) {
        ls = lsToGroupLs(ls)
    }

    if (hasChild(gameEl, ls)) {
        var locationEl = getChild(gameEl, ls)
        if (!group) {
            locationEl.innerHTML = html
        } else {
            if (locationEl.children.length > parts[4]) {
                locationEl.children[parts[4]].outerHTML = html
            } else {
                var div = document.createElement('div')
                div.innerHTML = html
                locationEl.appendChild(div.children[0])
            }
        }
    } else {
        var div = document.createElement('div')
        div.innerHTML = html
        div.classList.add(ls)
        div.classList.add('location')
        gameEl.appendChild(div)
    }
    if (group) {
        renderStockCount(parts[2])
    }
}

function renderStockCount(player) {
    var cubes = 0
    var discs = 0
    var el = getEl('ls-3-'+player+'-5')
    if (el != null) {
        Array.from(el.children).forEach(function (p) {
            if (p.classList.contains('piece-cube')) {
                cubes++
            } else if (p.classList.contains('piece-disc')) {
                discs++
            }
        })
        getChild(playerBoardEls[player], 'stock-cubecount').innerHTML = cubes
        getChild(playerBoardEls[player], 'stock-disccount').innerHTML = discs
    }
}

function buildPieces() {
    pieces = {}
    table.Board.Routes.forEach(function (route, ri) {
        route.Spots.forEach(function (spot, si) {
            pieces[lToS({
                Type: locationTypeRoute,
                Id: route.Id,
                Index: si,
                Subindex: 0
            })] = spot
        })
        route.Bumped.forEach(function (spot, si) {
            pieces[lToS({
                Type: locationTypeRoute,
                Id: route.Id,
                Index: si,
                Subindex: 1
            })] = spot
        })
    })
    table.Board.Cities.forEach(function (city, ci) {
        city.Offices.forEach(function (office, oi) {
            pieces[lToS({
                Type: locationTypeCity,
                Id: city.Id,
                Index: oi,
                Subindex: 0
            })] = office.Piece
        })
        // TODO: place these pixelwise.
        if (city.VirtualOffices != null) {
            city.VirtualOffices.forEach(function (piece, pi) {
                pieces[lToS({
                    Type: locationTypeCity,
                    Id: city.Id,
                    Index: pi,
                    Subindex: 1
                })] = piece
            })
        }
        if (city.Coellen.Spots != null) {
            city.Coellen.Spots.forEach(function (piece, pi) {
                pieces[lToS({
                    Type: locationTypeCity,
                    Id: city.Id,
                    Index: pi,
                    Subindex: 2
                })] = piece
            })
        }
    })
    table.PlayerBoards.forEach(function (pb, pbi) {
        pb.Keys.forEach(function (p, pi) {
            pieces[lToS({
                Type: locationTypePlayer,
                Id: pbi,
                Index: 0,
                Subindex: pi,
            })] = p
        })
        pb.Actions.forEach(function (p, pi) {
            pieces[lToS({
                Type: locationTypePlayer,
                Id: pbi,
                Index: 1,
                Subindex: pi,
            })] = p
        })
        pb.Priviledge.forEach(function (p, pi) {
            pieces[lToS({
                Type: locationTypePlayer,
                Id: pbi,
                Index: 2,
                Subindex: pi,
            })] = p
        })
        pb.Books.forEach(function (p, pi) {
            pieces[lToS({
                Type: locationTypePlayer,
                Id: pbi,
                Index: 3,
                Subindex: pi,
            })] = p
        })
        pb.Bags.forEach(function (p, pi) {
            pieces[lToS({
                Type: locationTypePlayer,
                Id: pbi,
                Index: 4,
                Subindex: pi,
            })] = p
        })
        pb.Stock.forEach(function (p, pi) {
            pieces[lToS({
                Type: locationTypePlayer,
                Id: pbi,
                Index: 5,
                Subindex: pi,
            })] = p
        })
        pb.Supply.forEach(function (p, pi) {
            pieces[lToS({
                Type: locationTypePlayer,
                Id: pbi,
                Index: 6,
                Subindex: pi,
            })] = p
        })
    })
}

function lToS(l) {
    var type =l.Type || '0'
    var id = l.Id || '0'
    var index = l.Index || '0'
    var subindex = l.Subindex || '0'
    return 'ls-'+type+'-'+id+'-'+index+'-'+subindex
}

function sToL(s) {
    pieces = s.split('-')
    return {
        Type: parseInt(pieces[1]),
        Id: parseInt(pieces[2]),
        Index: parseInt(pieces[3]),
        Subindex: parseInt(pieces[4])
    }
}

// Shape to String
function shapeToS(shape) {
    switch (shape) {
        case shapeNone:
            return 'none'
        case shapeCube:
            return 'cube'
        case shapeDisc:
            return 'disc'
        default:
            return 'none'
    }
}

function elToP(el) {
    var p = {
        PlayerColor: playerColorNone,
        Shape: shapeNone
    }
    el.classList.forEach(function (c) {
        if (c == 'piece-none') {
            p.PlayerColor = playerColorNone
        } else if (c == 'piece-yellow') {
            p.PlayerColor = playerColorYellow
        } else if (c == 'piece-green') {
            p.PlayerColor = playerColorGreen
        } else if (c == 'piece-blue') {
            p.PlayerColor = playerColorBlue
        } else if (c == 'piece-purple') {
            p.PlayerColor = playerColorPurple
        } else if (c == 'piece-red') {
            p.PlayerColor = playerColorRed
        } else if (c == 'piece-none') {
            p.Shape = shapeNone
        } else if (c == 'piece-cube') {
            p.Shape = shapeCube
        } else if (c == 'piece-disc') {
            p.Shape = shapeDisc
        }
    })
    return p
}

function awardToIcon(award) {
    if (award == awardNone) {
        return 'icon iconspacer'
    } else if (award == awardDiscs) {
        return 'icon iconbook'
    } else if (award == awardPriviledge) {
        return 'icon iconpriviledge'
    } else if (award == awardBags) {
        return 'icon iconbag'
    } else if (award == awardCoellen) {
        return 'icon iconcoellen'
    } else if (award == awardActions) {
        return 'icon iconaction'
    } else if (award == awardKeys) {
        return 'icon iconkey'
    } else {
        return 'icon iconspacer'
    }
}

function clickLeave(e) {
    sendRequestSitdown(e.dataset.i, false)
}
function clickSitHere(e) {
    sendRequestSitdown(e.dataset.i, true)
}
function sendRequestSitdown(i, s) {
    if (!ws) {
        return
    }
    var msg = '{"CType":'+ctypeRequestSitdown+',"Data":{"Index":'+i+',"Sitdown":'+s+'}}';
    ws.send(msg);
}

function clickAddBot(e) {
    var id = getUnusedBotIdentityId()
    sendRequestAddBot(id, e.dataset.i, true)
}
function clickLeaveBot(e) {
    sendRequestAddBot(table.PlayerBoards[e.dataset.i].Identity.Id, e.dataset.i, false)
}
function sendRequestAddBot(id, i, s) {
    if (!ws) {
        return
    }
    var msg = '{"CType":'+ctypeRequestSitdownBot+',"Data":{"Id":"'+id+'", "Index":'+i+',"Sitdown":'+s+'}}';
    ws.send(msg);
}

function clickStart() {
    if (!ws) {
        return
    }
    var msg = '{"CType":'+ctypeStartGame+',"Data":{}}';
    ws.send(msg);
}

// TODO: This should be moved to the backend unless we're going to build the
// dropdown, but this will change based on how I handle other's bots.  Leaving
// here for now.
var botIds = ['B1', 'B2', 'B3', 'B4', 'B5']
function getUnusedBotIdentityId() {
    for (var i=0; i<5; i++) {
        var exists = false
        table.PlayerBoards.forEach(function (pb) {
            if (botIds[i] == pb.Identity.Id) {
                exists = true
            }
        })
        if (!exists) {
            return botIds[i]
        }
    }
    return botIds[0]
}

function grabPiece(e) {
    e = e || window.event;
    var el = document.elementFromPoint(e.clientX, e.clientY)
    if (el == null || !el.classList.contains('piece')) {
        return
    }

    var sourceLs = ''
    el.parentElement.classList.forEach(function (c) {
        if (c.startsWith('ls-')) {
            sourceLs = c
        }
    })
    if (sourceLs == '') {
        console.log('Error: no ls- class in '+locationEl.classList)
    } else if (lsIsMissingSubindex(sourceLs)) {
        sourceLs+='-'+el.dataset.subindex
    }

    e.preventDefault();
    var startTop = el.offsetTop
    var startLeft = el.offsetLeft
    var halfsize = el.offsetWidth / 2
    el.style.top = (el.offsetTop - (halfsize - e.offsetY)) + "px";
    el.style.left = (el.offsetLeft - (halfsize - e.offsetX)) + "px";
    el.classList.add('grabbing')

    dragged = {
        el: el,
        sourceLs: sourceLs,
        startTop: startTop,
        startLeft: startLeft,
        mouseX: e.clientX,
        mouseY: e.clientY
    };

    document.onmousemove = dragPiece
    document.onmouseup = dropPiece
}

function dragPiece(e) {
    e = e || window.event;
    e.preventDefault();

    var pos1 = dragged.mouseX - e.clientX;
    var pos2 = dragged.mouseY - e.clientY;
    dragged.mouseX = e.clientX;
    dragged.mouseY = e.clientY;
    dragged.el.style.top = (dragged.el.offsetTop - pos2) + "px";
    dragged.el.style.left = (dragged.el.offsetLeft - pos1) + "px";
}

function dropPiece(e) {
    e = e || window.event;
    e.preventDefault();
    document.onmousemove = null
    document.onmouseup = null

    dragged.el.style.top = dragged.startTop + "px";
    dragged.el.style.left = dragged.startLeft + "px";
    dragged.el.classList.remove('grabbing')
    var sourceLs = dragged.sourceLs
    var piece = elToP(dragged.el)
    dragged = {}

    if (status != gameStatusRunning ||
        (turnstate.Type != turnStateTypeBumping && turnstate.Player != playerMe) ||
        (turnstate.Type == turnStateTypeBumping && turnstate.BumpingPlayer != playerMe)) {
        console.log("not your turn")
        return
    }

    var destLs = ''
    var droppedAtEl = document.elementFromPoint(e.clientX, e.clientY)
    if (!droppedAtEl.classList.contains('location') && droppedAtEl.parentElement != null) {
        droppedAtEl = droppedAtEl.parentElement
    }
    droppedAtEl.classList.forEach(function (c) {
        if (c.startsWith('ls-')) {
            destLs = c
        }
    })
    if (destLs == '') {
        console.log('dropped nowhere')
        return
    }
    if (lsIsMissingSubindex(destLs)) {
        for (var i=0;i<droppedAtEl.children.length;i++) {
            if (droppedAtEl.children[i].classList.contains('piece-none')) {
                destLs+='-'+i
                break
            }
        }
    }
    if (sourceLs == destLs) {
        return 
    }
    var subaction = {
        Source: sToL(sourceLs),
        Dest: sToL(destLs),
        Piece: piece
    }
    console.log('subaction: '+JSON.stringify(subaction))
    renderSubaction(subaction)
    pendingSubactions.push(subaction)
    sendDoSubaction(subaction)
}

function sendDoSubaction(s) {
    if (!ws) {
        return
    }
    var msg = '{"CType":'+ctypeDoSubaction+',"Data":'+JSON.stringify(s)+'}';
    ws.send(msg);
}

function sendEndturn() {
    if (!ws) {
        return
    }
    var msg = '{"CType":'+ctypeEndTurn+',"Data":{}}';
    ws.send(msg);
}

function sendEndbump() {
    if (!ws) {
        return
    }
    var msg = '{"CType":'+ctypeEndBump+',"Data":{}}';
    ws.send(msg);
}

function stopwatch() {
    Array.from(getEls('stopwatch-active')).forEach(function (el) {
        var ms = el.innerHTML.split(':')
        ms[1] = parseInt(ms[1])+1
        if (ms[1] == 60) {
            ms[1] = '00'
            ms[0] = parseInt(ms[0])+1
            if (ms[0] < 10) {
                ms[0] = '0'+ms[0]
            }
        } else if (ms[1] < 10) {
            ms[1] = '0'+ms[1]
        }
        el.innerHTML = ms.join(":")
    })
}

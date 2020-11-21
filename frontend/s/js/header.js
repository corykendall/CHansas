'use strict';

const stypeInternalError = 1;
const stypeYourIdentity = 2;
const stypeNotifyLobby = 3;
const stypeNotifySignup = 4;
const stypeNotifySignin = 5;
const stypeNotifyPasswordReset = 6;
const stypeNotifyConfirmEmail = 7;
const stypeNotifyUpdatePassword = 8;
const stypeNotifyNotification = 9;
const stypeHotDeploy = 10;
const stypeNotifyFullGame = 11;
const stypeNotifyCreateGame = 12;
const stypeNotifySitdown = 13;
const stypeNotifyStartGame = 14;
const stypeNotifySubaction = 15;
const stypeNotifyNextTurn = 16;
const stypeNotifyAction = 17;
const stypeNotifySubactionError = 18;
const stypeNotifyEndBump = 19;
const stypeNotifyScoringBegin = 20;
const stypeNotifyEndgameScoring = 21;
const stypeNotifyComplete = 22;

const ctypeRequestSignup = 1
const ctypeRequestSignin = 2
const ctypeRequestPasswordReset = 3
const ctypeUpdatePassword = 4
const ctypeCreateGame = 5
const ctypeRequestSitdown = 6;
const ctypeRequestSitdownBot = 7;
const ctypeStartGame = 8;
const ctypeDoSubaction = 9;
const ctypeEndTurn = 10;
const ctypeEndBump = 11;

const identityTypeNone = 0;
const identityTypeConnection = 1;
const identityTypeGuest = 2;
const identityTypeBot = 3;

const gameStatusCreating = 1;
const gameStatusRunning = 2;
const gameStatusAbandoned = 3;
const gameStatusScoring = 4;
const gameStatusComplete = 5;

var gameStatusNames = {
    1: 'Creating',
    2: 'Running',
    3: 'Abandoned',
    4: 'Scoring',
    5: 'Complete'
}

const turnStateTypeNone = 0;
const turnStateTypeBags = 1;
const turnStateTypeBumpPaying = 2;
const turnStateTypeBumping = 3;
const turnStateTypeMoving = 4;
const turnStateTypeClearing = 5;
const turnStateTypeRemove3 = 6;
const turnStateTypeLevelUp = 7;
const turnStateTypeBonusOffice = 8;
const turnStateTypeSwapOffice = 9;

const notificationError = 0
const notificationWarn = 1
const notificationInfo = 2
const notificationSuccess = 3
const notificationInternalError = 7

var notificationNames = {
    0: 'error',
    1: 'warn',
    2: 'info',
    3: 'success',
    7: 'internalerror'
}

const locationTypeNone = 0
const locationTypeRoute = 1
const locationTypeCity = 2
const locationTypePlayer = 3
const locationTypeCoellen = 4

const stackTypeNone = 0
const stackTypeTower = 1
const stackTypePile = 2

const playerColorNone = 0
const playerColorYellow = 1
const playerColorGreen = 2
const playerColorBlue = 3
const playerColorPurple = 4
const playerColorRed = 5

const shapeNone = 0
const shapeCube = 1
const shapeDisc = 2

const awardNone = 0
const awardDiscs = 1
const awardPriviledge = 2
const awardBags = 3
const awardCoellen = 4
const awardActions = 5
const awardKeys = 6

var hasLocalStorage

window.addEventListener('load', function() {
    getEl('signup-holder').onclick = handleSignupLinkClick
    getEl('signin-holder').onclick = handleSigninLinkClick
    getEl('identity-holder').onclick = handleIdentityClick
    getEl('signup-link').onclick = function () { getEl('signup-holder').click() }
    getEl('signup-button').onclick = handleSignupButtonClick
    getEl('signin-button').onclick = handleSigninButtonClick
    getEl('signin-resetpw-button').onclick = handleResetPasswordClick
    getEl('signout-link').onclick = handleSignoutButtonClick

    try {
        localStorage.setItem('t', 't');
        localStorage.removeItem('t');
        hasLocalStorage = true
    } catch(e) {
        hasLocalStorage = false
    }

    document.addEventListener('mouseup', function(evt) {
        if (!getEl('rightholder').contains(evt.target)) {
            hidetabs()
        }
    })
})

function handleTabClick(name) {
    var el = getEl(name+'-popup')
    var show = el.style.display != 'block'
    hidetabs()
    if (!show) {
        return false
    }
    el.style.display = 'block'
    getEl(name+'-holder').style['background-color'] = '#3C3C3C'
    return true
}

function hidetabs() {
    Array.from(document.getElementsByClassName('header-tabholder'))
        .forEach(function (h, i) {
            h.style['background-color'] = 'unset'
        })
    Array.from(document.getElementsByClassName('header-popup'))
        .forEach(function (p, i) {
            p.style.display = 'none'
        })
}

function handleSignupLinkClick() {
    handleTabClick('signup')
}

function handleSigninLinkClick() {
    handleTabClick('signin')
}

function handleIdentityClick() {
    handleTabClick('identity')
}

function handleSignupButtonClick() {
    var emailEl = getEl('signup-email')
    var usernameEl = getEl('signup-username')
    var password1El = getEl('signup-password1')
    var password2El = getEl('signup-password2')
    var email = emailEl.value
    var username = usernameEl.value
    var password1 = password1El.value
    var password2 = password2El.value

    getEl('signup-error').style.display = 'none'
    getEl('signup-loader').style.display = 'none'

    if (!validateEmail(email)) {
        signupError('Invalid email')
        return
    }

    if (!validateUsername(username)) {
        signupError('Invalid username')
        return
    }

    if (password1 != password2) {
        signupError('Passwords do not match')
        return
    }

    if (password1.length < 4) {
        signupError('Password too short')
        return
    }

    if (password1.length > 32) {
        signupError('Password too long')
        return
    }

    getEl('signup-button').disabled = true
    emailEl.disabled = true
    usernameEl.disabled = true
    password1El.disabled = true
    password2El.disabled = true
    getEl('signup-loader').style.display = 'block'
    sendRequestSignup(email, username, password1)
}

function handleNotifySignup(d) {
    getEl('signup-loader').style.display = 'none'
    if (d.Success) {
        var el = getEl('signup-checkemail')
        el.innerHTML = d.Message
        el.style.display = 'block'
    } else {
        signupError(d.Message)
        getEl('signup-button').disabled = false
        getEl('signup-email').disabled = false
        getEl('signup-username').disabled = false
        getEl('signup-password1').disabled = false
        getEl('signup-password2').disabled = false
    }
}

function handleSigninButtonClick() {
    var emailEl = getEl('signin-email')
    var passwordEl = getEl('signin-password')
    var email = emailEl.value
    var password = passwordEl.value

    if (!validateEmail(email)) {
        signinError('Invalid email')
        return
    }

    if (password.length < 4) {
        signinError('Password too short')
        return
    }

    if (password.length > 32) {
        signinError('Password too long')
        return
    }

    getEl('signin-resetpw-button').disabled = true
    getEl('signin-button').disabled = true
    emailEl.disabled = true
    passwordEl.disabled = true
    getEl('signin-error').style.display = 'none'
    getEl('signin-loader').style.display = 'block'

    sendRequestSignin(email, password)
}

function handleNotifySignin(d) {
    getEl('signin-loader').style.display = 'none'
    if (d.Success) {
        document.cookie='HansaAuthN='+d.Message+'; domain='+location.hostname+'; '+
            'expires=Thu, 18 Dec 2025 12:00:00 UTC; path=/ '
        getEl('signin-success').innerHTML = 'Success!  Reloading the page...'
        getEl('signin-success').style.display = 'block'
        location.reload()
    } else {
        signinError(d.Message)
        getEl('signin-button').disabled = false
        getEl('signin-resetpw-button').disabled = false
        getEl('signin-email').disabled = false
        getEl('signin-password').disabled = false
    }
}

function handleResetPasswordClick() {
    var emailEl = getEl('signin-email')
    var email = emailEl.value

    if (!validateEmail(email)) {
        signinError('Invalid email')
        return
    }

    getEl('signin-resetpw-button').disabled = true
    getEl('signin-button').disabled = true
    emailEl.disabled = true
    getEl('signin-error').style.display = 'none'
    getEl('signin-loader').style.display = 'block'

    sendRequestPasswordReset(email)
}

function handleNotifyPasswordReset(d) {
    getEl('signin-loader').style.display = 'none'
    if (d.Success) {
        getEl('signin-success').innerHTML = 'Password reset email sent'
        getEl('signin-success').style.display = 'block'
    } else {
        signinError("Failure.  Try sign up?")
    }
}

function closeOverlay(wsEverActive) {
    if (wsEverActive) {
        getEl('wsdead-overlay').style.display = 'block'
        setTimeout(function() {
            location.reload(true)
        }, 1000)
    } else {
        getEl('wsdead-overlay-message').innerHTML='Unable to Connect to CHansas'
        getEl('wsdead-overlay').style.display = 'block'
    }
}

function getIdentity() {
    return getEl('identity-id').innerHTML
}

var leagueColor = ['unknown', 'rgb(0,200,255)', 'rgb(255,128,128)']
function setIdentity(d) {
    getEl('identity-id').innerHTML = d.Identity.Id
    getEl('identity-name').innerHTML = d.Identity.Name

    if (d.Identity.Id.charAt(0) == 'P') {
        getEl('signout-link').style.display = 'block';
    } else {
        getEl('signup-link').style.display = 'block';
        getEl('signup-holder').style.display = 'block';
        getEl('signin-holder').style.display = 'block';
    }
}

function handleNotifyNotification(d) {
    var header = d.Header
    var content = d.Content

    var html = '<div class="toast toast-'+notificationNames[d.Type]+'">'+
        '<div class="toast-header"><div class="icon toast-icon"></div>'
    html += header+' '+
        '<div class="icon toast-icon"></div>'+
        '<div class="toast-x">x</div></div>'+
        '<div class="toast-content">'+content+'</div></div>'
    showToast(html)
}

function showToast(html) {
    var toastEl = document.createElement('div')
    toastEl.innerHTML = html
    toastEl = toastEl.firstChild

    var clearToastEl = function () { toastEl.remove() };
    var animDoneToastEl = function () {
        toastEl.classList.add('toast-done')
        setTimeout(clearToastEl, 1000)
    };
    getChild(toastEl, 'toast-x').onclick = clearToastEl
    setTimeout(animDoneToastEl, 8000)
    getEl('toast-container').appendChild(toastEl)
}

function signupError(msg) {
    var el = getEl('signup-error')
    el.innerHTML = msg
    el.style.display = 'block'
}

function signinError(msg) {
    var el = getEl('signin-error')
    el.innerHTML = msg
    el.style.display = 'block'
}

function handleSignoutButtonClick() {
    document.cookie='HansaAuthN=; domain='+location.hostname+'; '+
        'expires=Thu, 18 Dec 2015 12:00:00 UTC; path=/ '
    ws.close()
    getEl('signout-overlay').style.display = 'block'
    location.reload()
}

function sendRequestSignup(email, username, password) {
    if (!ws) {
        return false;
    }
    var msg = '{"CType":'+ctypeRequestSignup+',"Data":{"Email":'+JSON.stringify(email)+',"Username":"'+username+'","Password":'+JSON.stringify(password)+'}}';
    ws.send(msg);
}

function sendRequestSignin(email, password) {
    if (!ws) {
        return false;
    }
    var msg = '{"CType":'+ctypeRequestSignin+',"Data":{"Email":'+JSON.stringify(email)+',"Password":'+JSON.stringify(password)+'}}';
    ws.send(msg);
}

function sendRequestPasswordReset(email) {
    if (!ws) {
        return false;
    }
    var msg = '{"CType":'+ctypeRequestPasswordReset+',"Data":{"Email":'+JSON.stringify(email)+'}}';
    ws.send(msg);
}

function wssend(ctype, data) {
    if (!ws) {
        return false;
    }
    var msg = '{"CType":'+ctype+',"Data":'+data+'}';
    ws.send(msg);
}

const emailRegex = /^(([^<>()\[\]\\.,;:\s@"]+(\.[^<>()\[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
function validateEmail(email) {
    return emailRegex.test(String(email).toLowerCase());
}

const usernameRegex = /^[a-z0-9]+$/i
function validateUsername(username) {
    return username.length >= 4 && username.length <= 16 && usernameRegex.test(String(username))
}

function getEl(c) {
  return getEls(c)[0];
}

function getEls(c) {
  return document.getElementsByClassName(c);
}

function getChild(p, c) {
  return p.getElementsByClassName(c)[0];
}

function hasChild(p, c) {
  return p.getElementsByClassName(c).length > 0;
}

function unfocus() {
    var tmp = document.createElement("input");
    document.body.appendChild(tmp);
    tmp.focus();
    document.body.removeChild(tmp);
}

function displayTime(millis) {
    var time = new Date();
    time.setTime(millis)

    var hours = time.getHours()
    var minutes = time.getMinutes()
    if (hours == 0) {
        hours = 12
    } else if (hours > 12) {
        hours -= 12
    }
    if (minutes < 10) {
        minutes = '0'+minutes
    }
    return hours+':'+minutes
}

function displayDateTime(millis) {
    var time = new Date()
    time.setTime(millis)
    return time.toLocaleString()
}

// Used by getTimer below.
var hansaTimers = {}
setInterval(function () {
    for (var timer in hansaTimers) {
        var el = getEl(timer)
        if (el == undefined) {
            delete hansaTimers[timer]
            continue
        }
        var hms = el.innerHTML.split(':')
        if (hms.length == 3) {
            if (hms[2] != '00') {
                hms[2] = parseInt(hms[2])-1
                if (hms[2] < 10) {
                    hms[2] = '0'+hms[2]
                }
            } else if (hms[1] != '00') {
                hms[2] = '59'
                hms[1] = parseInt(hms[1])-1
                if (hms[1] < 10) {
                    hms[1] = '0'+hms[1]
                }
            } else if (hms[0] != '00') {
                hms[2] = '59';
                hms[1] = '59';
                hms[0] = parseInt(hms[0])-1
                if (hms[0] < 10) {
                    hms[0] = '0'+hms[0]
                }
            } else {
                delete hansaTimers[timer]
                return
            }
        } else if (hms.length == 2) {
            if (hms[1] != '00') {
                hms[1] = parseInt(hms[1])-1
                if (hms[1] < 10) {
                    hms[1] = '0'+hms[1]
                }
            } else if (hms[0] != '00') {
                hms[1] = '59';
                hms[0] = parseInt(hms[0])-1
                if (hms[0] < 10) {
                    hms[0] = '0'+hms[0]
                }
            } else {
                delete hansaTimers[timer]
                return
            }
        } else if (hms.length == 1) {
            if (hms[0] != '0') {
                hms[0] = parseInt(hms[0])-1
            } else {
                delete hansaTimers[timer]
                return
            }
        }
        el.innerHTML = hms.join(":")
    }
}, 1000);

// Give this an id and a date, and you get back a div with
// hours:minutes:seconds content, and a unique class.  The class is a key in
// the above hansaTimers, and every second a setTimeout runs to update the
// timer.  You can remove to key from hansaTimers directly to stop the timer,
// or it will stop automatically if it can't find the div or the counter gets
// to 00:00:00.  Accuracy can be 3 (hours:minutes:seconds), 2 (minutes:seconds)
// or 1 (seconds)
function getTimer(id, countdownToDate, accuracy) {
    var diff = countdownToDate - new Date();
    if (diff < 1) {
        if (accuracy == 3) {
            return '<div class="'+id+'">00:00:00</div>'
        } else if (accuracy == 2) {
            return '<div class="'+id+'">00:00</div>'
        } else if (accuracy == 1) {
            return '<div class="'+id+'">0</div>'
        }
    }
    var hms
    if (accuracy == 3) {
        hms = [
            Math.floor(diff/3600000),
            Math.floor(diff/60000)%60,
            Math.floor(diff/1000)%60]

        if (hms[0] < 10) {
            hms[0] = '0'+hms[0]
        }
        if (hms[1] < 10) {
            hms[1] = '0'+hms[1]
        }
        if (hms[2] < 10) {
            hms[2] = '0'+hms[2]
        }
    } else if (accuracy == 2) {
        hms = [
            Math.floor(diff/60000),
            Math.floor(diff/1000)%60]

        if (hms[0] < 10) {
            hms[0] = '0'+hms[0]
        }
        if (hms[1] < 10) {
            hms[1] = '0'+hms[1]
        }
    } else if (accuracy == 1) {
        hms = [Math.floor(diff/1000)]
    }

    hansaTimers[id] = true
    return '<div class="'+id+'">'+hms.join(':')+'</div>'
}

function removeTimer(id) {
    delete hansaTimers[id]
}

// 19 -> 7 PM
function hoursToPretty(hours) {
    if (hours == 0) {
        return '12 AM'
    } else if (hours < 12) {
        return ''+hours+' AM'
    } else if (hours == 12) {
        return '12 PM'
    }
    return ''+(hours-12)+' PM'
}

// 19, 4 -> 7:04 PM
function hoursMinutesToPretty(hours, minutes) {
    var am = 'AM'
    if (hours >= 12) {
        am = 'PM'
    }
    if (hours == 0) {
        hours = 12
    } else if (hours > 12) {
        hours = hours - 12
    } 
    if (minutes < 10) {
        minutes = '0'+minutes
    }
    return ''+hours+':'+minutes+' '+am
}

function goDurationToMinutesHours(d) {
    var seconds = d / 1000000000
    seconds = Math.trunc(seconds)
    var secondsPart = seconds % 60
    var minutesPart = (seconds-secondsPart) / 60
    if (secondsPart < 10) {
        secondsPart = '0'+secondsPart
    }
    if (minutesPart < 10) {
        minutesPart = '0'+minutesPart
    }
    return minutesPart+':'+secondsPart
}

// Player Color to String
function pcToS(pc) {
    switch (pc) {
        case playerColorNone:
            return 'none'
        case playerColorYellow:
            return 'yellow'
        case playerColorGreen:
            return 'green'
        case playerColorBlue:
            return 'blue'
        case playerColorPurple:
            return 'purple'
        case playerColorRed:
            return 'red'
        default:
            return 'none'
    }
}



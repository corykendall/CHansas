package server

import (
    "fmt"
	"net/http"
    "os"
    "runtime/debug"
    "strconv"
    "strings"
    "github.com/gorilla/mux"
    "github.com/gorilla/websocket"
    "local/hansa/bot"
    "local/hansa/client"
    "local/hansa/crypto"
    "local/hansa/database"
    "local/hansa/email"
    "local/hansa/lobby"
    "local/hansa/log"
    "local/hansa/message"
    "local/hansa/simple"
    "local/hansa/user"
)

type Server struct {
    config simple.Config
    upgrader websocket.Upgrader
    db *database.DB
    uh *user.Handler
    bm *bot.Manager
    lobby *lobby.Lobby
    broadcaster *Broadcaster
    ip simple.IpChecker
}

func New(config simple.Config) *Server {
    log.Info("New: running")
    initDone := make(chan struct{}, 100)

    db := database.New(config)
    err := db.Run(initDone)
    if err != nil {
        log.Stop("server New panic", err)
        panic(err)
    }

    broadcaster := NewBroadcaster()
    ip := NewIpChecker()

    em := email.NewEmailer(config)
    go em.Run(initDone)

    uh := user.NewHandler(db, em, config)
    go uh.Run(initDone)
    broadcaster.uh = uh

    bm := bot.NewManager()

    lobby := lobby.New(config, uh, db, bm, ip, broadcaster);
    go lobby.Run(initDone)
    broadcaster.lobby = lobby

    log.Info("New: Waiting for initDone on created resources")
    for i:=0;i<4;i++ {
        <-initDone
    }
    log.Info("New: Done")

    return &Server{
        config: config,
        upgrader: websocket.Upgrader{},
        db: db,
        uh: uh,
        bm: bm,
        lobby: lobby,
        broadcaster: broadcaster,
        ip: ip,
    }
}

func (s *Server) Run() {
    r := mux.NewRouter()
    r.Use(authNClosure(s.db, s.config))
    if s.config.Name != "prod" {
        r.Use(blockIps(s.config))
    }
    r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        s.dispatch(w, r)
    })
	http.Handle("/", r)

    addr := fmt.Sprintf("0.0.0.0:%d", s.config.ServerPort)
    log.Debug("Listening on %s", addr)
	log.Fatal("ListenAndServe return: %s", http.ListenAndServe(addr, nil))
}

func (s *Server) dispatch(w http.ResponseWriter, r  *http.Request) {
    p := r.URL.Path
    if p[0] != '/' {
        s.clientError(w, "URL Path doesn't begin with '/': %s", p)
        return
    }
    pe := strings.Split(p, "/")
    pe = pe[1:]
    if pe[0] == "" {
        s.clientError(w, "URL Path has no routes: %s", p)
        return
    }
    switch pe[0] {
        case "ws":
            s.handleWs(w, r, pe)
        case "a":
            s.handleAdmin(w, pe, r)
        default:
            s.clientError(w, "URL Path has no routes: %s", p)
    }
}

// If this goroutine (for the web request) panics, we will terminate the
// websocket.  For that reason, this go routine should register the websocket
// in some other component and perhaps send an initial message, but then
// complete quickly.
func (s *Server) handleWs(w http.ResponseWriter, r  *http.Request, pe []string) {
    pe = pe[1:]

    // Identity should be set in our AuthN handler.
    rawIdentity := r.Context().Value("Identity")
    if rawIdentity == nil {
        s.clientError(w, "No Identity in handler")
        return
    }
    identity, ok := s.castIdentity(rawIdentity)
    if !ok {
        s.clientError(w, "'Identity' in Context not a simple.Identity")
        return
    }

	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
        s.clientError(w, "Can't upgrade websocket: %e", err)
		return
	}
    c := client.NewWebClient(identity, ws, s.ip, getIP(r))
    // We only report error if we are in panic
    defer func() {
        if r := recover(); r!=nil {
            s.cServerError(c, fmt.Sprintf("Panic in websocket goroutine: %s\n%s",
                r,
                string(debug.Stack())))
        }
    }()
    go c.Run()

    if len(pe) == 0 {
        s.sendIdentity(c, identity)
        s.handleGenericWebsocket(c)
        return
    }

    switch pe[0] {
        case "lobby":
            s.sendIdentity(c, identity)
            s.handleLobby(c)
        case "g": 
            s.sendIdentity(c, identity)
            s.handleGame(c, pe)
        case "c": 
            s.handleConfirmEmail(c, pe)
        case "r": 
            s.handleUpdatePassword(c, pe)
        default:
            s.wsClientError(ws, "URL Path has no routes: %s", pe)
    }
}

func (s *Server) sendIdentity(c client.Client, i simple.Identity) {
    c.Send(message.NewYourIdentity(i))
}

// Pages like about or release need a websocket to handle identity information
// in the header, but they don't need a websocket for the page itself.  They
// land here.
func (s *Server) handleGenericWebsocket(c *client.WebClient) {
    s.uh.Join(c)
}

func (s *Server) handleLobby(c *client.WebClient) {
    s.lobby.Register(c)
}

func (s *Server) handleGame(c *client.WebClient, pe []string) {
    if len(pe) == 1 {
        s.cServerError(c, "URL Path has no routes: %s", pe)
        return;
    }
    gId, err := strconv.Atoi(pe[1])
    if err != nil {
        s.cServerError(c, "GameId is not a number: %s", pe[1])
        return;
    }

    s.lobby.RegisterGame(c, gId)
}

func (s *Server) handleConfirmEmail(c *client.WebClient, pe []string) {
    if len(pe) == 1 {
        c.Send(message.NewNotifyConfirmEmail(false))
        return
    }
    enc := pe[1]
    pe = pe[2:]
    for _, pee := range pe {
        enc = fmt.Sprintf("%s/%s", enc, pee)
    }

    email, err := crypto.DecryptBase64(enc, s.config.ConfigKeys["email"])
    if err != nil {
        log.Debug("(ClientError) Bad Confirm Email '%s' caused: %s", enc, err)
        c.Send(message.NewNotifyConfirmEmail(false))
        return
    }

    log.Debug("Confirming email: %s", email)
    err = s.db.ConfirmEmail(email)
    if err != nil {
        log.Error("Confirming email failed in db: %s", err)
        c.Send(message.NewNotifyConfirmEmail(false))
    }
    c.Send(message.NewNotifyConfirmEmail(true))
}

func (s *Server) handleUpdatePassword(c *client.WebClient, pe []string) {
    if len(pe) == 1 {
        c.Send(message.NewNotifyUpdatePassword(false))
        return
    }
    enc := pe[1]
    pe = pe[2:]
    for _, pee := range pe {
        enc = fmt.Sprintf("%s/%s", enc, pee)
    }

    email, err := crypto.DecryptBase64(enc, s.config.ConfigKeys["email"])
    if err != nil {
        log.Debug("(ClientError) Bad Update Password Email '%s' caused: %s", enc, err)
        c.Send(message.NewNotifyUpdatePassword(false))
        return
    }

    log.Debug("UpdatePassword conn opened, waiting for new password for: %s", email)

    // Note: This hangs the goroutine here, which is ok afaik
    m, ok := <-c.Read()
    if !ok {
        log.Debug("UpdatePassword conn closed before getting new password: %s", email)
        return
    } else if m.CType != message.UpdatePassword {
        log.Debug("UpdatePassword conn recieved bad CType: %s", m.CType)
        c.Send(message.NewNotifyUpdatePassword(false))
        return
    }
    s.uh.HandleUpdatePasswordForEmail(email, c, m.Data.(message.UpdatePasswordData))
}

func (s *Server) handleAdmin(w http.ResponseWriter, pe []string, r *http.Request) {
    pe = pe[1:]

    if len(pe) == 0 {
        log.Error("Admin function called with no route (raw '/a')")
        s.adminError(w)
        return
    }

    switch pe[0] {
    case "hotdeploy":
        s.handleHotDeploy(w)
    default:
        log.Error("URL Path has no routes: /a/%s", pe)
        s.adminError(w)
    }
}

func (s *Server) handleHotDeploy(w http.ResponseWriter) {
    log.Info("HotDeploy admin request recieved (/a/hotdeploy)")
    m := message.NewNotifyNotification(
        message.NotificationInfo,
        "CHansas Update",
        "CHansas will update after this turn.  Expected update time is 15 seconds.  Stay here to be reseated automatically.")
    s.broadcaster.Broadcast(message.Broadcast{"", m})

/*
    // TODO: Hotdeploy out to running games.
    //wg := s.ts.HotDeploy()

    log.Info("Waiting for all games to pause...")
    wg.Wait()

    log.Info("Hotdeploy prepared: all games paused.  "+
        "Wait 5 seconds for final messages to be delivered")
    time.Sleep(5*time.Second)
    */

    log.Fatal("Hotdeploy prepared: Shutting down.")
    s.adminSuccess(w)
    log.Stop("HotDeploy", nil)
    os.Exit(0)
}

func getIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

func (s *Server) castIdentity(raw interface{}) (i simple.Identity, b bool) {
    b = true
    i = raw.(simple.Identity)
    defer func() {
        if r := recover(); r!=nil {
            b = false
        }
    }()
    return i, b
}

func (s *Server) clientError(w http.ResponseWriter, m string, fs ...interface{}) {
    m = fmt.Sprintf("(ClientError) %s", m)
    log.Info(m, fs)
    http.Error(w, m, 400)
}

func (s *Server) adminSuccess(w http.ResponseWriter) {
    w.Write([]byte("OK"))
}

func (s *Server) adminError(w http.ResponseWriter) {
    http.Error(w, "FAIL", 400)
}

func (s *Server) wsClientError(ws *websocket.Conn, m string, f ...interface{}) {
    m = fmt.Sprintf(fmt.Sprintf("(ClientError) %s", m), f...)
    log.Info(m)

    // We don't care about errors, we are closing anyway.
    // TODO: Wrap message for server SType and respect on browser
    _ = ws.WriteMessage(websocket.TextMessage, []byte(m))
    ws.Close()
}

func (s *Server) cServerError(client *client.WebClient, e string, fargs ...interface{}) {
    log.Error(e, fargs...)
    m := message.NewInternalError("Internal Error")
    client.Send(m)
}

package user

import (
    "fmt"
    "reflect"
    "encoding/base64"
    "golang.org/x/crypto/bcrypt"
    "local/hansa/client"
    "local/hansa/crypto"
    "local/hansa/database"
    "local/hansa/email"
    "local/hansa/log"
    "local/hansa/message"
    "local/hansa/simple"
)

type Handler struct {
    config simple.Config
    db *database.DB
    emailer *email.Emailer
    clients map[simple.Identity]*client.MultiWebClient
    join chan *client.WebClient
    broadcast chan message.Broadcast
}

func NewHandler(db *database.DB, em *email.Emailer, config simple.Config) *Handler {
    return &Handler{
        db: db,
        emailer: em,
        config: config,
        clients: make(map[simple.Identity]*client.MultiWebClient),
        join: make(chan *client.WebClient, 10),
        broadcast: make(chan message.Broadcast, 10),
    }
}

func (h *Handler) Run(initDone chan struct{}) {
    defer h.panicking()
    rcase := func(c reflect.Value) reflect.SelectCase {
        return reflect.SelectCase{
            Dir: reflect.SelectRecv,
            Chan: c,
        }
    }

    initDone <- struct{}{}

    for {
        cases := []reflect.SelectCase{
            rcase(reflect.ValueOf(h.join)),
            rcase(reflect.ValueOf(h.broadcast)),
        }
        order := []simple.Identity{}
        for i, c := range h.clients {
            order = append(order, i)
            cases = append(cases, rcase(reflect.ValueOf(c.Read())))
        }

        chosen, value, ok := reflect.Select(cases)

        switch chosen {
        case 0:
            h.handleJoin(value.Interface().(*client.WebClient))
        case 1:
            h.handleBroadcast(value.Interface().(message.Broadcast))
        default:
            i := order[chosen-2]
            if !ok {
                h.handleLeave(i)
            } else {
                h.Handle(h.clients[i], value.Interface().(message.Client))
            }
        }
    }
}

func (h *Handler) Join(c *client.WebClient) {
    h.join <-c
}

func (h *Handler) Handle(c client.Client, m message.Client) {
    switch m.CType {
    case message.RequestSignup:
        h.handleRequestSignup(c, m.Data.(message.RequestSignupData))
    case message.RequestSignin:
        h.handleRequestSignin(c, m.Data.(message.RequestSigninData))
    case message.RequestPasswordReset:
        h.handleRequestPasswordReset(c, m.Data.(message.RequestPasswordResetData))
    case message.UpdatePassword:
        h.handleUpdatePassword(c, m.Data.(message.UpdatePasswordData))
    }
}

func (h *Handler) Broadcast(b message.Broadcast) {
    h.broadcast <-b
}

func (h *Handler) handleJoin(c *client.WebClient) {
    if mc, ok := h.clients[c.Identity()]; ok {
        mc.Consume(c)
    } else {
        h.clients[c.Identity()] = client.NewMultiWebClient(c)
        go h.clients[c.Identity()].Run()
    }
}

func (h *Handler) handleBroadcast(b message.Broadcast) {
    if b.Id == "" {
        for _, c := range h.clients {
            c.Send(b.M)
        }
        return
    }
    for i, c := range h.clients {
        if i.Id == b.Id {
            c.Send(b.M)
        }
        break
    }
}

func (h *Handler) handleLeave(i simple.Identity) {
    delete(h.clients, i)
}

func (h *Handler) handleRequestSignup(c client.Client, d message.RequestSignupData) {

    if ok, msg := h.validatePassword(d.Password); !ok {
        c.Send(message.NewNotifySignup(false, msg))
    }
    storablePassword, err := h.hashpw(c, d.Password)
    if err != nil {
        c.Send(message.NewNotifySignup(false, "Internal Error validating password"))
    }

    // TODO more validation lol?

    err, dberr := h.db.Signup(d.Email, d.Username, storablePassword)
    if dberr != nil {
        c.Send(message.NewInternalError("Database Issue, goodbye"))
    } else if err != nil {
        if err.Error() == "email" {
            h.sendPasswordResetEmail(d.Email)
            c.Send(message.NewNotifySignup(false,
                "Known email address: password reset email sent"))
        } else if err.Error() == "username" {
            c.Send(message.NewNotifySignup(false,
                "Username taken, choose another"))
        } else {
            c.Send(message.NewInternalError("Database Issue, goodbye"))
        }
    } else {
        h.sendConfirmEmail(d.Email)
        c.Send(message.NewNotifySignup(true, "Check your email to complete signup"))
    }
}

func (h *Handler) handleRequestSignin(c client.Client, d message.RequestSigninData) {
    id, err, dberr := h.db.Signin(d.Email, []byte(d.Password))
    if dberr != nil {
        c.Send(message.NewInternalError("Database Issue, goodbye"))
    } else if err != nil {
        if err.Error() == "email" {
            c.Send(message.NewNotifySignin(false, "Unknown email"))
        } else if err.Error() == "verified" {
            h.sendConfirmEmail(d.Email)
            c.Send(message.NewNotifySignin(false, "Check your email to complete signup"))
        } else if err.Error() == "password" {
            c.Send(message.NewNotifySignin(false, "Wrong password"))
        } else {
            c.Send(message.NewInternalError("Database Issue, goodbye"))
        }
    } else {
        c.Send(message.NewNotifySignin(true, crypto.NewCookieValue(id, h.config)))
    }
}

func (h *Handler) handleRequestPasswordReset(c client.Client, d message.RequestPasswordResetData) {
    exist, err := h.db.EmailExists(d.Email)
    if err != nil {
        h.errorf(c, "Unable to select from db: %s", err)
        exist = false
    }
    if exist {
        h.sendPasswordResetEmail(d.Email)
    }
    c.Send(message.NewNotifyPasswordReset(exist))
}

func (h *Handler) handleUpdatePassword(c client.Client, d message.UpdatePasswordData) {
    i := c.Identity()
    if i.Type != simple.IdentityTypeConnection {
        h.debugf(c, "(ClientError) %s attempted to update password", i)
        c.Send(message.NewNotifyUpdatePassword(false))
    }
    h.handleUpdatePasswordForIdentity(i, c, d)
}

// In this case, we don't care about the client's identity because it's
// probably a guest, this person came into server through a password reset
// email and then webpage.
func (h *Handler) HandleUpdatePasswordForEmail(email string, c client.Client, d message.UpdatePasswordData) {
    i, ok := h.db.GetIdentityFromEmail(email)
    if !ok {
        c.Send(message.NewNotifyUpdatePassword(false))
    }
    h.handleUpdatePasswordForIdentity(i, c, d)
}

func (h *Handler) handleUpdatePasswordForIdentity(i simple.Identity, c client.Client, d message.UpdatePasswordData) {
    if ok, _ := h.validatePassword(d.Password); !ok {
        c.Send(message.NewNotifyUpdatePassword(false))
    }
    storablePassword, err := h.hashpw(c, d.Password)
    if err == nil {
        err = h.db.SetPassword(i.Id, storablePassword)
    }
    c.Send(message.NewNotifyUpdatePassword(err == nil))
}

func (h *Handler) validatePassword(pw string) (bool, string) {
    if len(pw) < 4 {
        return false, "Password too short"
    } else if len(pw) > 32 {
        return false, "Password too long"
    }
    return true, ""
}

func (h *Handler) hashpw(c client.Client, pw string) ([]byte, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
    if err != nil {
        h.errorf(c, "bcrypt error hashing password: %s", err)
        return []byte{}, err
    }
    return hash, nil
}

func (h *Handler) sendPasswordResetEmail(email string) {
    key := h.config.ConfigKeys["email"]
    encryptedEmail := crypto.Encrypt(email, key)
    encodedEmail := base64.URLEncoding.EncodeToString(encryptedEmail)
    link := fmt.Sprintf("https://%s/r/%s", h.config.ServerDNS, encodedEmail)

    h.emailer.Send(email, "CHansas: Reset Password",
        fmt.Sprintf("Navigate to this website to reset your password: %s", link),
        fmt.Sprintf("Click <a href=\"%s\">here</a> to reset your password.", link))
}

func (h *Handler) sendConfirmEmail(email string) {
    key := h.config.ConfigKeys["email"]
    encryptedEmail := crypto.Encrypt(email, key)
    encodedEmail := base64.URLEncoding.EncodeToString(encryptedEmail)
    link := fmt.Sprintf("https://%s/c/%s", h.config.ServerDNS, encodedEmail)

    h.emailer.Send(email, "CHansas: Confirm Email Address",
        fmt.Sprintf("Navigate to this website to confirm your email address: %s", link),
        fmt.Sprintf("Click <a href=\"%s\">here</a> to confirm your email address.", link))
}

func (h *Handler) panicking() {
    if r := recover(); r != nil {
        log.Stop("UserHandler panic", r)
    }
}

func (h *Handler) debugf(c client.Client, msg string, fargs ...interface{}) {
    log.Debug(fmt.Sprintf("(%s) %s", c.Identity().Id, msg), fargs...)
}

func (h *Handler) errorf(c client.Client, msg string, fargs ...interface{}) {
    log.Error(fmt.Sprintf("(%s) %s", c.Identity().Id, msg), fargs...)
}

func (h *Handler) fatalf(msg string, fargs ...interface{}) {
    log.Fatal(msg, fargs...)
}

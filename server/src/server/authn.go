package server

import (
    "context"
    "fmt"
    "net/http"
    "local/hansa/crypto"
    "local/hansa/log"
    "local/hansa/database"
    "local/hansa/simple"
)

func authNClosure(db *database.DB, config simple.Config) func(http.Handler) http.Handler  {
    playerCookieName := "HansaAuthN"
    guestCookieName := "HansaAuthNGuest"
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
            ip := getIP(r)
            path := r.URL.Path

            // Path /a is blocked at the nginx level from calls off this box.
            // This is a crontab calling in to perform some timed maintanence.
            if path[0:2] == "/a" {
                next.ServeHTTP(w, r)
                return
            }

            // Ensure cookie exists, and use playerCookie over guestCookie.
            playerCookie :=  ""
            guestCookie := ""
            for _, c := range r.Cookies() {
                if c.Name == guestCookieName {
                    guestCookie = c.Value
                } else if c.Name == playerCookieName {
                    playerCookie = c.Value
                }
            }
            cookie := playerCookie
            if cookie == "" {
                cookie = guestCookie
            }

            if cookie == "" {
                log.Debug("Access: NoCookie %s (%s)", ip, path)
                w.WriteHeader(http.StatusForbidden)
                w.Write([]byte(fmt.Sprintf("Missing cookie '%s' or '%s'",
                    playerCookieName, guestCookieName)))
                return
            }

            // Check cookie validity
            id, ok := crypto.ReadCookie(cookie, ip, path, config)
            if !ok {
                w.WriteHeader(http.StatusForbidden)
                w.Write([]byte("Bad cookie"))
                return
            }

            // Set Identity in Context
            identity, ok := db.GetIdentity(id)
            if !ok {
                w.WriteHeader(http.StatusForbidden)
                w.Write([]byte(fmt.Sprintf("InternalError Get Identity '%s'", id)))
                return
            }
            next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(),
                "Identity", identity,
            )))
        })
    }
}


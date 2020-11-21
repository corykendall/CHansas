package server

import (
	"net/http"
    "local/hansa/log"
    "local/hansa/simple"
)

func blockIps(config simple.Config) func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
            ip := getIP(r)
            path := r.URL.Path
            if ip != string(config.ConfigKeys["myip"]) && path[0:3] != "/a/" {
                log.Debug("Access: BlockedIP %s (%s)", ip, path)
                w.WriteHeader(http.StatusForbidden)
                w.Write([]byte("403"))
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

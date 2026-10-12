package api

import (
	"errors"
	"net"
	"net/http"
	"strings"
)

func IsLoopbackHost(host string) bool {
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

func localOnly(next http.Handler) http.Handler {
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, http.StatusForbidden, errors.New("cross-origin request rejected"))
	}))
	protected := protection.Handler(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if name, _, err := net.SplitHostPort(host); err == nil {
			host = name
		} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = host[1 : len(host)-1]
		}
		if !IsLoopbackHost(host) {
			writeAPIError(w, http.StatusForbidden, errors.New("only localhost and loopback IP hosts are allowed"))
			return
		}
		protected.ServeHTTP(w, r)
	})
}

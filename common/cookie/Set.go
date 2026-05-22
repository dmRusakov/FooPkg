package cookie

import (
	"net/http"
	"time"
)

// Set sets a cookie with the given parameters.
func Set(w http.ResponseWriter, key, value string, expireTime time.Time, isHttpOnly bool) {
	cookie := &http.Cookie{
		Name:     key,
		Value:    value,
		Expires:  expireTime,
		HttpOnly: isHttpOnly,
		Path:     "/",
	}
	http.SetCookie(w, cookie)
}

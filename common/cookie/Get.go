package cookie

import (
	"net/http"
)

// Get retrieves the value of a cookie with the given key from the request.
func Get(r *http.Request, key string) (string, error) {
	cookie, err := r.Cookie(key)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

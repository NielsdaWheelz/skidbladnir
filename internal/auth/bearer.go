package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"strings"
)

const bearerBytes = 32

var bearerEncoding = base64.RawURLEncoding.Strict()

type Credential struct{ raw [bearerBytes]byte }

func (c Credential) CanonicalBearer() string { return bearerEncoding.EncodeToString(c.raw[:]) }
func (c Credential) Verify(authorization string) bool {
	raw, ok := strings.CutPrefix(authorization, "Bearer ")
	presented, e := ParseCredential(raw)
	valid := 0
	if ok && e == nil {
		valid = 1
	}
	return subtle.ConstantTimeCompare(c.raw[:], presented.raw[:])&valid == 1
}

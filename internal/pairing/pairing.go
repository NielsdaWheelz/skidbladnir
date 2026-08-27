package pairing

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

const (
	inviteLifetime    = 5 * time.Minute
	tokenBytes        = 32
	tokenTextSize     = 43
	tokenDigestDomain = "skidbladnir.pairing-invite.v1\x00"
)

var (
	tokenEncoding = base64.RawURLEncoding.Strict()
	errRejected   = errors.New("pairing invitation is invalid, expired, or already used")
)

type Token struct {
	raw [tokenBytes]byte
}

func ParseToken(value string) (Token, error) {
	var token Token
	if len(value) != tokenTextSize {
		return token, errRejected
	}
	decoded, err := tokenEncoding.DecodeString(value)
	if err != nil || len(decoded) != tokenBytes {
		return token, errRejected
	}
	copy(token.raw[:], decoded)
	if token.String() != value {
		return Token{}, errRejected
	}
	return token, nil
}

func (token Token) String() string {
	return tokenEncoding.EncodeToString(token.raw[:])
}

type Invitation struct {
	Token     Token
	ExpiresAt time.Time
}

type Slot struct {
	mutex  sync.Mutex
	invite *activeInvitation
}

type activeInvitation struct {
	tokenVerifier [sha256.Size]byte
	bearerDigest  [sha256.Size]byte
	machine       machine.Handle
	expiresAt     time.Time
}

func NewSlot() *Slot {
	return &Slot{}
}

func (slot *Slot) Create(now time.Time, handle machine.Handle, credential auth.Credential) (Invitation, error) {
	if handle.String() == "" || now.IsZero() {
		return Invitation{}, errors.New("pairing invitation input is invalid")
	}
	raw := [tokenBytes]byte{}
	if _, err := rand.Read(raw[:]); err != nil {
		return Invitation{}, fmt.Errorf("mint pairing invitation: %w", err)
	}
	token := Token{raw: raw}
	expiresAt := now.Add(inviteLifetime)
	invite := &activeInvitation{
		tokenVerifier: tokenVerifier(token),
		bearerDigest:  credential.Digest(),
		machine:       handle,
		expiresAt:     expiresAt,
	}
	slot.mutex.Lock()
	slot.invite = invite
	slot.mutex.Unlock()
	return Invitation{Token: token, ExpiresAt: expiresAt.UTC()}, nil
}

func (slot *Slot) Redeem(now time.Time, encodedToken string, handle machine.Handle, credential auth.Credential) error {
	token, err := ParseToken(encodedToken)
	if err != nil || handle.String() == "" || now.IsZero() {
		return errRejected
	}
	presentedToken := tokenVerifier(token)
	presentedBearer := credential.Digest()
	slot.mutex.Lock()
	defer slot.mutex.Unlock()
	if slot.invite == nil {
		return errRejected
	}
	invite := slot.invite
	if !now.Before(invite.expiresAt) {
		slot.invite = nil
		return errRejected
	}
	if subtle.ConstantTimeCompare(invite.bearerDigest[:], presentedBearer[:]) != 1 {
		slot.invite = nil
		return errRejected
	}
	if subtle.ConstantTimeCompare(invite.tokenVerifier[:], presentedToken[:]) != 1 || invite.machine != handle {
		return errRejected
	}
	slot.invite = nil
	return nil
}

func tokenVerifier(token Token) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(tokenDigestDomain)) // justify-ignore-error: hash.Hash.Write never returns an error.
	_, _ = hash.Write(token.raw[:])              // justify-ignore-error: hash.Hash.Write never returns an error.
	var verifier [sha256.Size]byte
	copy(verifier[:], hash.Sum(nil))
	return verifier
}

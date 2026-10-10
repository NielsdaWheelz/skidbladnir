package notifier

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
)

type Publisher struct {
	origin         string
	credentialPath string
	http           *http.Client
}

func NewPublisher(origin, credentialPath string) (*Publisher, error) {
	token, err := readPrivate(credentialPath, 257)
	if err != nil || !validPublisherToken(token) {
		return nil, attention.ErrUnavailable
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Publisher{origin: origin, credentialPath: credentialPath, http: &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func validPublisherToken(data []byte) bool {
	if len(data) < 2 || len(data) > 257 || data[len(data)-1] != '\n' {
		return false
	}
	for _, value := range data[:len(data)-1] {
		if value < 33 || value > 126 {
			return false
		}
	}
	return true
}
func (publisher *Publisher) Publish(ctx context.Context, subscription attention.Subscription, hint attention.Hint) error {
	if !subscription.Valid(publisher.origin) {
		return attention.ErrUnavailable
	}
	encoded, err := json.Marshal(hint)
	if err != nil || len(encoded) > attention.MaximumHintBytes {
		return attention.ErrUnavailable
	}
	encrypted, err := encrypt(subscription, encoded)
	if err != nil {
		return err
	}
	credential, err := readPrivate(publisher.credentialPath, 257)
	if err != nil || !validPublisherToken(credential) {
		return attention.ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, subscription.Endpoint, bytes.NewReader(encrypted))
	if err != nil {
		return attention.ErrUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSuffix(string(credential), "\n"))
	request.Header.Set("Content-Encoding", "aes128gcm")
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("X-UnifiedPush", "1")
	response, err := publisher.http.Do(request)
	if err != nil {
		return attention.ErrUnavailable
	}
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 8193))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return attention.ErrUnavailable
	}
	return nil
}

// encrypt emits exactly one bounded RFC8291 aes128gcm record.
func encrypt(subscription attention.Subscription, message []byte) ([]byte, error) {
	if len(message) > attention.MaximumHintBytes {
		return nil, attention.ErrUnavailable
	}
	receiver, auth, err := subscription.PublicKeys()
	if err != nil {
		return nil, err
	}
	sender, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	salt := [16]byte{}
	if _, err := rand.Read(salt[:]); err != nil {
		return nil, attention.ErrUnavailable
	}
	return encryptRecord(sender, receiver, auth, salt, message)
}
func encryptRecord(sender *ecdh.PrivateKey, receiver *ecdh.PublicKey, auth []byte, salt [16]byte, message []byte) ([]byte, error) {
	shared, err := sender.ECDH(receiver)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	senderPublic := sender.PublicKey().Bytes()
	info := append([]byte("WebPush: info\x00"), receiver.Bytes()...)
	info = append(info, senderPublic...)
	ikm, err := hkdf.Key(sha256.New, shared, auth, string(info), 32)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	key, err := hkdf.Key(sha256.New, ikm, salt[:], "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt[:], "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, attention.ErrUnavailable
	}
	result := make([]byte, 21, 512)
	copy(result, salt[:])
	binary.BigEndian.PutUint32(result[16:20], 4096)
	result[20] = 65
	result = append(result, senderPublic...)
	plaintext := make([]byte, len(message)+1)
	copy(plaintext, message)
	plaintext[len(message)] = 2
	return gcm.Seal(result, nonce, plaintext, nil), nil
}

package account

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	totpIssuer = "LegacyStore"
	totpPeriod = 30
	totpDigits = 6
)

func generateTOTPSecret() (string, error) {
	raw, err := randomBytes(20)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func totpURL(email, secret string) string {
	label := urlEncode(totpIssuer + ":" + email)
	return "otpauth://totp/" + label + "?secret=" + secret + "&issuer=" + urlEncode(totpIssuer) + "&period=30&digits=6"
}

func verifyTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}
	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return false
	}
	counter := now.Unix() / totpPeriod
	for drift := int64(-1); drift <= 1; drift++ {
		if hotp(secretBytes, uint64(counter+drift)) == code {
			return true
		}
	}
	return false
}

func hotp(secret []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, secret)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)
	mod := int(math.Pow10(totpDigits))
	return fmt.Sprintf("%0"+strconv.Itoa(totpDigits)+"d", value%mod)
}

func urlEncode(value string) string {
	replacer := strings.NewReplacer(" ", "%20", "@", "%40", ":", "%3A", "+", "%2B")
	return replacer.Replace(value)
}

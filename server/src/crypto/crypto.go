package crypto

import(
    "fmt"
    "time"
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
	"net/http"
    "io"
    "strconv"
    "local/hansa/log"
    "local/hansa/simple"
)

func Encrypt(s string, k []byte) []byte {
    block, err := aes.NewCipher(k)
	if err != nil { panic(err.Error()) }
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil { panic(err.Error()) }
	aesgcm, err := cipher.NewGCM(block)
	if err != nil { panic(err.Error()) }
    ciphertext := aesgcm.Seal(nil, nonce, []byte(s), nil)
    return append(nonce[:], ciphertext[:]...)
}

func Decrypt(c []byte, k []byte) (string, error) {
    nonce := c[:12]
    ciphertext := c[12:]

    block, err := aes.NewCipher(k)
	if err != nil { panic(err.Error()) }
	aesgcm, err := cipher.NewGCM(block)
	if err != nil { panic(err.Error()) }
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
        return "", err
	}
    return string(plaintext), nil
}

func DecryptBase64(c string, k []byte) (string, error) {
    decoded, err := base64.URLEncoding.DecodeString(c)
    if err != nil {
        return "", err
    }

    decrypted, err := Decrypt(decoded, k)
    if err != nil {
        return "", err
    }

    return string(decrypted), nil
}

var realCookieName string = "HansaAuthN"
func NewCookie(id string, config simple.Config) *http.Cookie {

    return &http.Cookie{
        Name: realCookieName,
        Value: NewCookieValue(id, config),
        Expires: time.Now().AddDate(5,0,0),
        Path: "/",
        Secure: true,
        HttpOnly: true,
        SameSite: http.SameSiteStrictMode,
    }
    //http.SetCookie(w, cookie)
}

func NewCookieValue(id string, config simple.Config) string {
    // From P123 to P000000123
    idNum, _ := strconv.Atoi(id[1:])
    id = fmt.Sprintf("%c%09d", id[0], idNum)

    cookieValue := fmt.Sprintf("%s%s", time.Now().Format("20060102"), id)
    encryptedCookieValue := Encrypt(cookieValue, config.ConfigKeys["cookie"])
    encodedCookieValue := base64.URLEncoding.EncodeToString(encryptedCookieValue)
    return string(encodedCookieValue)
}


func ReadCookie(raw string, ip string, path string, config simple.Config) (string, bool) {
    decoded, err := base64.URLEncoding.DecodeString(raw)
    if err != nil {
        log.Info("(ClientError) Access: BadCookieDecode %s (%s) %s", ip, path, raw)
        return "", false
    }

    decrypted, err := Decrypt(decoded, config.ConfigKeys["cookie"])
    if err != nil {
        log.Info("(ClientError) Access: BadCookieDecrypt %s (%s) %s", ip, path, raw)
        return "", false
    }

    // TODO: Currently we ignore the date it was generated, but this could be
    // used for forced rollover or versioning, and is tamper-proof unlike
    // cookie's Expire attribute
    casted := string(decrypted)

    cType := casted[8]
    if cType != 'G' && cType != 'P' {
        log.Info("(ClientError) Access: BadCookieType %s (%s) %s", ip, path, raw)
        return "", false
    }

    cNum, err := strconv.Atoi(casted[9:])
    if err != nil {
        log.Info("(ClientError) Access: BadCookieNumber %s (%s) %s", ip, path, raw)
        return "", false
    }
    id := fmt.Sprintf("%c%d", cType, cNum)
    log.Debug("Access: (%s) %s (%s)", id, ip, path)
    return id, true
}

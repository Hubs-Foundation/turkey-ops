package utils

import (
	crand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"net/http"
	"strings"
	"time"
)

func NewUUID() []byte {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(crand.Reader, uuid)
	if n != len(uuid) || err != nil {
		Logger.Panic("NewUUID err, something's not right")
		return uuid
	}
	// variant bits;
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// v4 pseudo-random;
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return uuid
	// return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

func CreateNewSession() *http.Cookie {
	Logger.Println("######################## create new sessions: ########################")
	id := base64.RawURLEncoding.EncodeToString(NewUUID())
	cookie := &http.Cookie{
		Name:  SessionTokenName,
		Value: id,
	}
	CACHE.Put(
		id,
		&CacheBoxSessData{},
		time.Hour*1,
	)
	Logger.Println("######################## new sessions created ########################")
	return cookie
}

func AddCacheData(c *http.Cookie) *CacheBoxSessData {
	CACHE.Put(
		c.Value,
		&CacheBoxSessData{},
		time.Hour*1,
	)
	return CACHE.Load(c.Value)
}

// func ShortMiniteUniqueID() string {
// 	timeStr := time.Now().UTC().Format("200601021504")
// 	timeInt, _ := strconv.ParseInt(timeStr, 10, 64)
// 	timeHex := strconv.FormatInt(int64(timeInt), 36)
// 	return timeHex
// }

// PwDGen -- will use all printable chars except space and delete (why is this a printable char?)
// so ascii code 33 - 126
func PwdGen(length int) string {
	var seed int64
	binary.Read(crand.Reader, binary.BigEndian, &seed)
	mr := mrand.New(mrand.NewSource(seed))

	var pwd string
	for i := 0; i < length-2; i++ {
		roll := mr.Intn(95) + 32
		pwd = pwd + string(byte(roll))
	}
	troubleChars := []string{`/`, `"`, `@`, ` `, `\`, `<`, `>`, `:`, `{`, `}`, "'", "`"}
	for _, c := range troubleChars {
		replace := string(byte(mr.Intn(26) + 65))
		pwd = strings.Replace(pwd, c, replace, -1)
	}
	return "~" + pwd + "~"
}

func StackNameGen() string {
	var seed int64
	binary.Read(crand.Reader, binary.BigEndian, &seed)
	mr := mrand.New(mrand.NewSource(seed))
	return ShortAdjs[mr.Intn(len(ShortAdjs))] + ShortNouns[mr.Intn(len(ShortNouns))]
}

var ShortAdjs = []string{
	"bad", "big", "dim", "dry", "fat", "fit", "fun", "hot", "icy", "mad", "odd",
	"raw", "red", "sad", "shy", "tan", "wet", "new", "old", "rad",
}
var ShortNouns = []string{
	"air", "ant", "art", "axe", "act", "ale", "ape", "arm", "ash", "awl", "amp",
	"bag", "bay", "bat", "bun", "box", "bed", "bee", "bow",
	"cab", "cam", "can", "car", "cat", "cup", "cod", "cog", "cow",
	"dam", "den", "dew", "dog", "ear", "eye", "eal", "ice", "ion", "key", "pie", "sea", "tea",
}

func ParseJsonReqBody(reqBody io.ReadCloser) (map[string]string, error) {

	bytes, err := ioutil.ReadAll(reqBody)
	if err != nil {
		return nil, err
	}

	inputmap := make(map[string]string)
	err = json.Unmarshal(bytes, &inputmap)
	if err != nil {
		return nil, err
	}
	return inputmap, err
}

func GetSession(Cookie func(string) (*http.Cookie, error)) *CacheBoxSessData {
	cookie, _ := Cookie(SessionTokenName)
	sess := CACHE.Load(cookie.Value)
	if sess == nil {
		Logger.Println("WARNING @ GetSession: session not found")
	}
	return sess
}

func GetMimeType(fileExtension string) string {
	if val, ok := mimeMap[fileExtension]; ok {
		return val
	} else {
		return ""
	}
}

var mimeMap = map[string]string{
	".html": "text/html",
	".htm":  "text/html",
	".css":  "text/css",
}

/////////////////////////////////////////////////

/////////////////////////////////////////////////

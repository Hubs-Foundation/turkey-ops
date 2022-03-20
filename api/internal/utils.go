package internal

import (
	"bufio"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"io/ioutil"
	"math/rand"
	mrand "math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger
var Atom zap.AtomicLevel

func InitLogger() {
	Atom = zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "t"
	encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout("060102.03:04:05MST") //wanted to use time.Kitchen so much
	encoderCfg.CallerKey = "c"
	encoderCfg.FunctionKey = "f"
	encoderCfg.MessageKey = "m"
	// encoderCfg.FunctionKey = "f"
	logger = zap.New(zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.Lock(os.Stdout), Atom), zap.AddCaller())

	defer logger.Sync()

	Atom.SetLevel(zap.DebugLevel)
}

func GetLogger() *zap.Logger {
	return logger
}

func NewUUID() []byte {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(crand.Reader, uuid)
	if n != len(uuid) || err != nil {
		logger.Panic("NewUUID err, something's not right")
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
	logger.Debug("######################## create new sessions: ########################")
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
	logger.Debug("######################## new sessions created ########################")
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

// PwDGen -- will use all printable chars except space and delete (why is delete a printable char?)
// aka. ascii code 33 - 126
func PwdGen(length int, seed int64) string {
	// var seed int64
	// binary.Read(crand.Reader, binary.BigEndian, &seed)
	// mr := mrand.New(mrand.NewSource(seed))
	// var pwd string
	// for i := 0; i < length-2; i++ {
	// 	roll := mr.Intn(95) + 32
	// 	pwd = pwd + string(byte(roll))
	// }
	// troubleChars := []string{`/`, `"`, `@`, ` `, `\`, `<`, `>`, `:`, `{`, `}`, "'", "`", "|", "%"}
	// for _, c := range troubleChars {
	// 	replace := string(byte(mr.Intn(26) + 65)) //ascii 65~90 aka. A~Z
	// 	pwd = strings.Replace(pwd, c, replace, -1)
	// }
	rand.Seed(seed)

	pwd := make([]rune, length)
	var dict = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	dictLen := len(dict)
	for i := range pwd {
		pwd[i] = dict[rand.Intn(dictLen)]
	}
	return "~" + string(pwd) + "~"
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
		logger.Debug("WARNING @ GetSession: session not found")
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

// Copy the src file to dst. Any existing file will be overwritten and will not
// copy file attributes.
func Copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func RunCmd_sync(name string, arg ...string) error {

	cmd := exec.Command(name, arg...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, _ := cmd.StderrPipe()

	err = cmd.Start()
	GetLogger().Debug("started: " + cmd.String())
	if err != nil {
		return err
	}

	// print the output of the subprocess
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		m := scanner.Text()
		GetLogger().Debug(m)
	}

	scanner_err := bufio.NewScanner(stderr)
	for scanner_err.Scan() {
		m := scanner_err.Text()
		GetLogger().Error(m)
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}

func RunCmd_async(name string, updates chan string, arg ...string) error {

	cmd := exec.Command(name, arg...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, _ := cmd.StderrPipe()

	err = cmd.Start()
	GetLogger().Debug("started: " + cmd.String())
	if err != nil {
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			m := scanner.Text()
			GetLogger().Debug(m)
			updates <- m
		}

		scanner_err := bufio.NewScanner(stderr)
		for scanner_err.Scan() {
			m := scanner_err.Text()
			GetLogger().Error(m)
			updates <- m
		}

		close(updates)
	}()

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}

func RootDomain(fullDomain string) string {
	fdArr := strings.Split(fullDomain, ".")
	len := len(fdArr)
	if len < 2 {
		return ""
	}
	return fdArr[len-2] + "." + fdArr[len-1]
}

/////////////////////////////////////////////////

/////////////////////////////////////////////////

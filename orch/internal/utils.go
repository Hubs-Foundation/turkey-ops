package internal

import (
	"bufio"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger
var AtomLvl zap.AtomicLevel

func InitLogger() {
	AtomLvl = zap.NewAtomicLevel()
	if os.Getenv("LOG_LEVEL") == "warn" {
		AtomLvl.SetLevel(zap.WarnLevel)
	} else if os.Getenv("LOG_LEVEL") == "debug" {
		AtomLvl.SetLevel(zap.DebugLevel)
	} else {
		AtomLvl.SetLevel(zap.InfoLevel)
	}

	// encoderCfg := zap.NewProductionEncoderConfig()
	// encoderCfg.TimeKey = "t"
	// encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout("060102.03:04:05MST") //wanted to use time.Kitchen so much
	// encoderCfg.CallerKey = "c"
	// encoderCfg.FunctionKey = "f"
	// encoderCfg.MessageKey = "m"
	// // encoderCfg.FunctionKey = "f"

	zapCfg := &zap.Config{
		Level:    zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			FunctionKey:   "func",
			TimeKey:       "time",
			LevelKey:      "severity",
			NameKey:       "logger",
			CallerKey:     "caller",
			MessageKey:    "message",
			StacktraceKey: "stacktrace",
			LineEnding:    zapcore.DefaultLineEnding,
			EncodeLevel: func(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
				switch l {
				case zapcore.DebugLevel:
					enc.AppendString("DEBUG")
				case zapcore.InfoLevel:
					enc.AppendString("INFO")
				case zapcore.WarnLevel:
					enc.AppendString("WARNING")
				case zapcore.ErrorLevel:
					enc.AppendString("ERROR")
				case zapcore.DPanicLevel:
					enc.AppendString("CRITICAL")
				case zapcore.PanicLevel:
					enc.AppendString("ALERT")
				case zapcore.FatalLevel:
					enc.AppendString("EMERGENCY")
				}
			},
			EncodeTime:     zapcore.RFC3339TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// Logger = zap.New(zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.Lock(os.Stdout), AtomLvl), zap.AddCaller())
	// err := errors.New("-_-")
	var err error
	Logger, err = zapCfg.Build(zap.AddCaller())
	if err != nil {
		panic(err)
	}

	defer Logger.Sync()

}

func GetLogger() *zap.Logger {
	return Logger
}

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
	Logger.Debug("######################## create new sessions: ########################")
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
	Logger.Debug("######################## new sessions created ########################")
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

// PwDGen -- see dict
func PwdGen(length int, seed int64, padding string) string {
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
	mrand.Seed(seed)

	pwd := make([]rune, length)
	var dict = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	dictLen := len(dict)
	for i := range pwd {
		pwd[i] = dict[mrand.Intn(dictLen)]
	}

	return padding + string(pwd) + ReverseString(padding)
}
func ReverseString(s string) (result string) {
	for _, v := range s {
		result = string(v) + result
	}
	return
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
	if cookie == nil {
		return nil
	}
	sess := CACHE.Load(cookie.Value)
	if sess == nil {
		Logger.Debug("WARNING @ GetSession: session not found")
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

func RunCmd_sync(name string, arg ...string) (error, []string) {

	cmd := exec.Command(name, arg...)
	var out []string

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err, nil
	}
	stderr, _ := cmd.StderrPipe()

	err = cmd.Start()
	GetLogger().Info("RunCmd_sync: " + cmd.String())
	if err != nil {
		return err, nil
	}

	// collect outputs of the subprocess
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		m := scanner.Text()
		GetLogger().Info(m)
		out = append(out, m)
	}

	scanner_err := bufio.NewScanner(stderr)
	for scanner_err.Scan() {
		m := scanner_err.Text()
		GetLogger().Error(m)
		out = append(out, m)
	}

	err = cmd.Wait()
	if err != nil {
		return err, nil
	}

	return nil, out
}

func FindRootDomain(fullDomain string) string {
	fdArr := strings.Split(fullDomain, ".")
	len := len(fdArr)
	if len < 2 {
		return ""
	}
	return fdArr[len-2] + "." + fdArr[len-1]
}

func IsValidDomainName(domain string) bool {
	RegExp := regexp.MustCompile(`^(([a-zA-Z]{1})|([a-zA-Z]{1}[a-zA-Z]{1})|([a-zA-Z]{1}[0-9]{1})|([0-9]{1}[a-zA-Z]{1})|([a-zA-Z0-9][a-zA-Z0-9-_]{1,61}[a-zA-Z0-9]))\.([a-zA-Z]{2,6}|[a-zA-Z0-9-]{2,30}\.[a-zA-Z]{2,3})$`)
	return RegExp.MatchString(domain)
}

func RetryHttpReq(client *http.Client, request *http.Request, maxRetry time.Duration) (*http.Response, time.Duration, error) {

	stepWait := 8 * time.Second

	timeout := time.Now().Add(maxRetry)
	tStart := time.Now()
	resp, err := client.Do(request)
	for err != nil || resp.StatusCode > 299 {
		time.Sleep(stepWait)
		ttl := time.Until(timeout)
		if ttl < 0 {
			return nil, time.Since(tStart), fmt.Errorf("timeout waiting for %v", request.URL)
		}

		if err != nil {
			Logger.Sugar().Debugf("retrying for %v, ttl: %v, reason: err -- %v", request.URL, ttl, err.Error())
		}
		if resp != nil {
			Logger.Sugar().Debugf("retrying for %v, ttl: %v, reson: resp -- %v", request.URL, ttl, resp.StatusCode)
		}

		resp, err = client.Do(request)
	}
	Logger.Sugar().Debugf("T-ready: %v for %v", time.Since(tStart), request.URL)

	return resp, time.Since(tStart), nil
}

/////////////////////////////////////////////////

/////////////////////////////////////////////////

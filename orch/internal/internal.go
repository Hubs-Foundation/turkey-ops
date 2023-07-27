package internal

const CONST_DEFAULT_TIME_FORMAT = "060102-0304"
const CONST_SESSION_TOKEN_NAME = "session_token"

// var Logger = log.New(os.Stdout, "http: ", log.LstdFlags)
var CACHE CacheBox
var TokenBook *tokenBook
var TrcCmBook *trcCmBook
var TrcCacheBook *trcCacheBook

func InitSingletons() {
	CACHE = NewCacheBox()
	TokenBook = NewTokenBook(5)
	TrcCmBook = NewTrcCmBook()
	TrcCacheBook = NewTrcCacheBook("trcCacheBook_" + Cfg.Domain)
}

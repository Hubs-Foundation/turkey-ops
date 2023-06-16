package internal

const CONST_DEFAULT_TIME_FORMAT = "060102-0304"
const CONST_CONST_SESSION_TOKEN_NAME = "session_token"

// var Logger = log.New(os.Stdout, "http: ", log.LstdFlags)
var CACHE = NewCacheBox()
var TokenBook = NewTokenBook(5)

// type RootPageData struct {
// 	ShowStackInfo bool
// 	Stacks        []StackInfo
// }

// type UserData struct {
// 	AwsCreds    AwsCreds          `json:"aws_stuff"`
// 	Cfoverrides map[string]string `json:"cf_overrides"`
// 	Stacks      []StackInfo       `json:"stacks"`
// }

// type AwsCreds struct {
// 	Key    string `json:"key"`
// 	Secret string `json:"secret"`
// 	Region string `json:"region"`
// 	// CERTarn           string `json:"CERTarn"`
// 	// S3bucket          string `json:"S3bucket,omitempty"`
// 	// LambdaBucket      string `json:"LambdaBucket,omitempty"`
// 	// ContainerRegistry string `json:"ContainerRegistry,omitempty"`
// }

// type StackInfo struct {
// 	StackName  string    `json:"stack_name"`
// 	TimeStart  time.Time `json:"time_start"`
// 	StackLink  string
// 	LastStatus string
// }

// // func newUserData() UserData {
// // 	userData := UserData{
// // 		AwsCreds: AwsCreds{},
// // 		Stacks:   []StackInfo{StackInfo{}}}
// // 	return userData
// // }

// type stackInfoData struct {
// 	showStackInfo bool
// 	Stacks        []StackInfo
// }

// func (u *UserData)populate(in map[string]string){
// 	u.AwsCreds.Key=in["awsKey"]
// 	u.AwsCreds.Secret=in["awsSecret"]
// 	u.AwsCreds.Region=in["awsRegion"]
// }

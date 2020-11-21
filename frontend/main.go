package main

import (
    "bytes"
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "io"
    "io/ioutil"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"
    "golang.org/x/net/html"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/awserr"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/gorilla/mux"
)

var stack string
var ddb *dynamodb.DynamoDB
var pages = map[string][]byte{}

// This is loaded from server.cfg
var ConfigKeys = map[string][]byte{}

func main() {
    InitLog("/home/ec2-user/hansa/logs", DebugLevel)
    Debug("Started")
    stack = LoadStackName("/home/ec2-user/hansa/server.cfg")
    Debug(fmt.Sprintf("Using stack '%s'", stack))

    initPages()

    // Not guarded by AuthN
    fs := http.FileServer(http.Dir("./s"))
    http.Handle("/s/", http.StripPrefix("/s/", fs))
    http.HandleFunc("/favicon.ico", faviconHandler)

    // Guarded by AuthN
    r := mux.NewRouter()
    if stack != "prod" {
        r.Use(blockIps)
    }
    r.Use(authN)

    makeFunc := func(f string) (func (w http.ResponseWriter, r *http.Request)) {
        return func (w http.ResponseWriter, r *http.Request) {
            servePage(f, w, r)
        }
    }

    r.HandleFunc("/", makeFunc("lobby.html"))
    r.PathPrefix("/c/").HandlerFunc(makeFunc("email.html"))
    r.PathPrefix("/r/").HandlerFunc(makeFunc("password.html"))
    r.PathPrefix("/hansa").HandlerFunc(makeFunc("about.html"))
    r.PathPrefix("/release").HandlerFunc(makeFunc("release.html"))
    r.HandleFunc("/g/{gameid}", makeFunc("game.html"))

    // DynamoDB Setup for GuestID generation
    sess := session.Must(session.NewSessionWithOptions(session.Options{
        SharedConfigState: session.SharedConfigEnable,
    }))
    ddb = dynamodb.New(sess)
  
    http.Handle("/", r)
    Debug("Listening on :10000...")
    err := http.ListenAndServe(":10000", nil)
    if err != nil {
        Error(err.Error())
    }
}

func initPages() {
    files, err := ioutil.ReadDir("html")
    if err != nil {
        Fatal(err.Error())
        panic(err)
    }
    for _, f := range files {
        file, err := ioutil.ReadFile("html/"+f.Name())
        if err != nil {
            Fatal(err.Error())
            panic(err)
        }
        pages[f.Name()] = file
        Debug("Loaded page from disk: %s", f.Name())
    }

    // Build the release page by merging release and releasefragment
    rBytes, ok := pages["release.html"]
    if !ok {
        Fatal("don't have release.html")
        panic("don't have release.html")
    }
    rNode, err := html.Parse(bytes.NewReader(rBytes))
    if err != nil {
        Fatal(err.Error())
        panic(err)
    }
    rContent, ok := pages["releasefragment.html"]
    if !ok {
        Fatal("don't have releasefragment.html")
        panic("don't have releasefragment.html")
    }
    rContentNode, err := html.ParseFragment(bytes.NewReader(rContent), nil)
    if err != nil {
        Fatal(err.Error())
        panic(err)
    }

    var dfs func(*html.Node) *html.Node
    dfs = func(x *html.Node) *html.Node {
        if len(x.Attr) > 0 &&
            x.Attr[0].Key == "class" &&
            x.Attr[0].Val == "release" {
                return x
        }
        if x.FirstChild != nil {
            y := dfs(x.FirstChild)
            if y != nil {
                return y
            }
        }
        if x.NextSibling != nil {
            return dfs(x.NextSibling)
        }
        return nil
    }
    rDivNode := dfs(rNode)
    if rDivNode == nil {
        Fatal("no class=release in release.html")
        panic("no class=release in release.html")
    }

    // Strip off Html and body tags from rContentNode
    rCNode := rContentNode[0].FirstChild.NextSibling.FirstChild
    rCNode.Parent = nil
    rCNode.NextSibling = nil
    rCNode.PrevSibling = nil

    rDivNode.AppendChild(rCNode)

    var buf bytes.Buffer
    html.Render(&buf, rNode)
    pages["release.html"] = buf.Bytes()
}

func servePage(f string, w http.ResponseWriter, r *http.Request) {
    lobby, ok := pages[f]
    if !ok {
        Error("Can't servePage (doesn't exist): %s", f)
    }
    fmt.Fprintf(w, string(lobby))
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "s/img/favicon.png")
}

func blockIps(next http.Handler) http.Handler {
    myip := string(ConfigKeys["myip"])
    return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
        ip := GetIP(r)
        path := r.URL.Path
        if ip != myip {
            Debug(fmt.Sprintf("Access: BlockedIP %s (%s)", ip, path))
            w.WriteHeader(http.StatusForbidden)
            w.Write([]byte("403"))
            return
        }
        next.ServeHTTP(w, r)
    })
}

var testCookieName string = "HansaAuthNAccept"
var guestCookieName string = "HansaAuthNGuest"
var playerCookieName string = "HansaAuthN"

func authN(next http.Handler) http.Handler {
    return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
        ip := GetIP(r)
        path := r.URL.Path

        // There are 5 cases.
        //  * User has no cookies:  Respond with a test cookie and a cookie page
        //  * User has only test cookie: Respond with actual page and guest cookie
        //  * User has guest cookie and no player cookie: respond with real page
        //  * User has player cookie and no guest cookie: respond with real page
        //  * User has player cookie and guest cookie: respond with real page, use player cookie
        testCookie := ""
        guestCookie := ""
        playerCookie := ""

        for _, c := range r.Cookies() {
            if c.Name == testCookieName {
                testCookie = c.Value
            } else if c.Name == guestCookieName {
                guestCookie = c.Value
            } else if c.Name == playerCookieName {
                playerCookie = c.Value
            }
        }
        realCookie := playerCookie
        if realCookie == "" {
            realCookie = guestCookie
        }

        if realCookie == "" && testCookie != "true" {
            Debug(fmt.Sprintf("Access: NoCookie %s (%s)", ip, path))
            
            cookie := &http.Cookie{
                Name: testCookieName,
                Value: "false",
                Expires: time.Now().Add(30 * time.Minute),
            }
            http.SetCookie(w, cookie)

            // We don't serve the real page with no cookie at all.  They have
            // 30 minutes to click ok.
            cookiepage, _ := ioutil.ReadFile("html/cookies.html")
            fmt.Fprintf(w, string(cookiepage))
            return 
        }

        // Whether they have no real cookie or a bad one, give them a new valid
        // guest one and let them through.
        if realCookie == "" {
            Debug(fmt.Sprintf("Access: CookieAccept %s (%s)", ip, path))
            if !addNewCookie(w) {
                Error(fmt.Sprintf("Generate new Cookie Fail"))
                // For now just give them this page, they can try again :(
                cookiepage, _ := ioutil.ReadFile("html/cookies.html")
                fmt.Fprintf(w, string(cookiepage))
                return
            }
            removeTestCookie(w)
            next.ServeHTTP(w, r)
            return
        }
            
        if (!checkCookie(realCookie, ip, path)) {
            Warn(fmt.Sprintf("Warn: BadCookie %s (%s)", ip, path))
            if !addNewCookie(w) {
                Error(fmt.Sprintf("Error: Generate new Cookie Fail"))
                // For now just give them this page, they can try again :(
                cookiepage, _ := ioutil.ReadFile("html/cookies.html")
                fmt.Fprintf(w, string(cookiepage))
                return
            }
            removeTestCookie(w)
            next.ServeHTTP(w, r)
            return
        }

        // Let them through.  Note that this frontend is still just serving
        // static files, but this saves cookie creation work from the backend.
        next.ServeHTTP(w, r)
    })
}

func addNewCookie(w http.ResponseWriter) bool {
    id, ok := getNewGuestId()
    if !ok {
        return false
    }

    cookieValue := fmt.Sprintf("%s%s%09d", time.Now().Format("20060102"), "G", id)
    encryptedCookieValue := encrypt(cookieValue)
    encodedCookieValue := base64.URLEncoding.EncodeToString(encryptedCookieValue)
    cookie := &http.Cookie{
        Name: guestCookieName,
        Value: string(encodedCookieValue),
        Expires: time.Now().AddDate(5,0,0),
        Path: "/",
    }
    http.SetCookie(w, cookie)
    return true
}

func removeTestCookie(w http.ResponseWriter) {
    http.SetCookie(w, &http.Cookie{
        Name: testCookieName,
        Value: "",
        Expires: time.Now().AddDate(0,0,-1),
    })
}

// TODO: There should be an in memory cache here so we're not doing a decrypt
// on each hit.
func checkCookie(encodedCookieValue string, ip string, path string) bool {
    Trace(fmt.Sprintf("Checking cookie value: %s", encodedCookieValue))
    encryptedCookieValue, err := base64.URLEncoding.DecodeString(encodedCookieValue)
    if err != nil {
        return false
        Debug(fmt.Sprintf("Cookie decode fail."))
    }

    cookieValue, err := decrypt(encryptedCookieValue)
    if err != nil {
        return false
        Debug(fmt.Sprintf("Cookie decrypt fail."))
    }

    cType := cookieValue[8]
    if cType != 'G' && cType != 'P' {
        Debug(fmt.Sprintf("Cookie CType fail"))
        return false
    }

    cNum, err := strconv.Atoi(cookieValue[9:])
    if err != nil {
        Debug(fmt.Sprintf("strconv fail: '%s'", cookieValue[9:]))
        return false
    }
    Debug(fmt.Sprintf("Access: (%c%d) %s (%s)", cType, cNum, ip, path))

    return true
}


// This prepends the 12 byte nonce to the encrypted data.
func encrypt(s string) []byte {
    key := ConfigKeys["cookie"]
    block, err := aes.NewCipher(key)
	if err != nil { panic(err.Error()) }
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil { panic(err.Error()) }
	aesgcm, err := cipher.NewGCM(block)
	if err != nil { panic(err.Error()) }
    ciphertext := aesgcm.Seal(nil, nonce, []byte(s), nil)
    return append(nonce[:], ciphertext[:]...)
}
func decrypt(c []byte) (string, error) {
    key := ConfigKeys["cookie"]
    nonce := c[:12]
    ciphertext := c[12:]

    block, err := aes.NewCipher(key)
	if err != nil { panic(err.Error()) }
	aesgcm, err := cipher.NewGCM(block)
	if err != nil { panic(err.Error()) }
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
        return "", err
	}
    return string(plaintext), nil
}

func getNewGuestId() (int, bool) {
    result, err := ddb.UpdateItem(&dynamodb.UpdateItemInput{
        ReturnValues:     aws.String("UPDATED_OLD"),
        TableName:        aws.String(fmt.Sprintf("%s-Hansa-Counters", stack)),
        UpdateExpression: aws.String("SET V = V + :i"),
        Key:              map[string]*dynamodb.AttributeValue{"H":{S: aws.String("G")}},
        ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{":i":{N:aws.String("1")}},
    })
    if err != nil {
        formatDDBError(err)
        return 0, false
    }
    
    id, err := strconv.Atoi(*result.Attributes["V"].N)
    if err != nil {
        return 0, false
    }
    return id, true
}

func formatDDBError(err error) {
    if aerr, ok := err.(awserr.Error); ok {
        switch aerr.Code() {
        case dynamodb.ErrCodeConditionalCheckFailedException:
            Error(fmt.Sprintln(dynamodb.ErrCodeConditionalCheckFailedException, aerr.Error()))
        case dynamodb.ErrCodeProvisionedThroughputExceededException:
            Error(fmt.Sprintln(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error()))
        case dynamodb.ErrCodeResourceNotFoundException:
            Error(fmt.Sprintln(dynamodb.ErrCodeResourceNotFoundException, aerr.Error()))
        case dynamodb.ErrCodeItemCollectionSizeLimitExceededException:
            Error(fmt.Sprintln(dynamodb.ErrCodeItemCollectionSizeLimitExceededException, aerr.Error()))
        case dynamodb.ErrCodeTransactionConflictException:
            Error(fmt.Sprintln(dynamodb.ErrCodeTransactionConflictException, aerr.Error()))
        case dynamodb.ErrCodeRequestLimitExceeded:
            Error(fmt.Sprintln(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error()))
        case dynamodb.ErrCodeInternalServerError:
            Error(fmt.Sprintln(dynamodb.ErrCodeInternalServerError, aerr.Error()))
        default:
            Error(fmt.Sprintln(aerr.Error()))
        }
    } else {
        // Print the error, cast err to awserr.Error to get the Code and
        // Message from an error.
        Error(fmt.Sprintln(err.Error()))
    }
}

func GetIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

func LoadStackName(filename string) string {
    configBytes, err := ioutil.ReadFile(filename)
    if err != nil {
        now := time.Now().Format("2006-01-02T15:04:05.000Z")
        fmt.Printf("%s: LoadConfig err reading '%s', goodbye: %s\n", now, filename, err)
        os.Exit(1)
    }
    configVars := strings.TrimSpace(string(configBytes))
    for _, cfg := range strings.Split(configVars, "\n") {
        parts := strings.Split(cfg, "=")
        if parts[0] == "email" || parts[0] == "cookie" {
            ConfigKeys[parts[0]] = decodeString(parts[1])
        } else {
            ConfigKeys[parts[0]] = []byte(parts[1])
        }
    }

    stackName, ok := ConfigKeys["stack"]
    now := time.Now().Format("2006-01-02T15:04:05.000Z")
    if !ok {
        fmt.Printf("%s: LoadConfig found no 'stack' in config.  goodbye.", now)
        os.Exit(1)
    }
    fmt.Printf("%s: Server Started with stack: '%s'\n", now, stackName)

    return string(stackName)
}

func decodeString(s string) (r []byte) {
    r, _ = hex.DecodeString(s)
    return
}

type LogLevel int
const (
    TraceLevel LogLevel = iota
    DebugLevel
    InfoLevel
    WarnLevel
    ErrorLevel
    FatalLevel
)
var logDirectory string
var serverlog *os.File
var serverchan chan string
var days <-chan time.Time // produces a Time on the day
var firstRollover bool
var done chan struct{}
var level LogLevel

func InitLog(ld string, l LogLevel) {
    logDirectory = ld
    serverlog = openLog()

    level = l
    serverchan = make(chan string, 10)
    done = make(chan struct{}, 1)

    firstRollover = true
    firstTick := make(chan time.Time, 1)
    days = func() <-chan time.Time { return firstTick }()
    go firstDayTick(firstTick)

    go runServerlog()
}

func StopLog() {
    close(serverchan)
    <-done
}

func Fatal(msg string, fargs ...interface{}) {
    if level <= FatalLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[FATAL] %s", msg), fargs...)
    }
}

func Error(msg string, fargs ...interface{}) {
    if level <= ErrorLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[ERROR] %s", msg), fargs...)
    }
}

func Warn(msg string, fargs ...interface{}) {
    if level <= WarnLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[WARN] %s", msg), fargs...)
    }
}

func Info(msg string, fargs ...interface{}) {
    if level <= InfoLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[INFO] %s", msg), fargs...)
    }
}

func Debug(msg string, fargs ...interface{}) {
    if level <= DebugLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[DEBUG] %s", msg), fargs...)
    }
}

func Trace(msg string, fargs ...interface{}) {
    if level <= TraceLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[TRACE] %s", msg), fargs...)
    }
}

func runServerlog() {
    for {
        select {
        case m, ok := <-serverchan:
            if ok {
                log(serverlog, m)
            } else {
                serverlog.Sync()
                close(done)
                return
            }
        case <-days:
            if firstRollover {
                days = time.NewTicker(3600 * time.Second).C
                firstRollover = false
            }
            rollLog()
        }
    }
}

func firstDayTick(x chan time.Time) {
    now := time.Now()
    dayEnd := time.Date(now.Year(), now.Month(), now.Day(),
        0, 0, 0, 0, now.Location())
    dayEnd = dayEnd.Add(24 * time.Hour)
    sleepTime := dayEnd.Sub(now)
    Debug("Log rollover initialization: now: '%s' dayEnd: '%s', sleepTime: '%s'",
        now, dayEnd, sleepTime)
    time.Sleep(sleepTime)
    x <-time.Now()
}

func rollLog() {
    serverlog.Close()
    serverlog = openLog()
}

func openLog() *os.File {
    f, err := os.OpenFile(
        fmt.Sprintf("%s/frontend.log.%s", logDirectory, logSuffix()),
        os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        panic(fmt.Sprintf("Unable to open serverlog in %s for writing: %s",
            logDirectory, err))
    }
    return f
}

// date +'%Y%m%d'
func logSuffix() string {
    return time.Now().Format("20060102")
}

func log(logfile *os.File, msg string) {
    logfile.WriteString(fmt.Sprintf("%s %s\n",
        time.Now().Format("15:04:05.000Z"),
        strings.ReplaceAll(msg, "\n", "\\n")))
}

package simple

import (
    "encoding/hex"
    "fmt"
    "os"
    "strings"
    "time"
    "io/ioutil"
    "github.com/aws/aws-sdk-go/aws/session"
)

type Config struct {
    Name string
    LogDirectory string
    ServerPort int
    ServerDNS string
    RdsUser string
    RdsHost string
    RdsPort string
    RdsName string
    S3Bucket string
    S3Folder string
    EmailSender string
    AwsRole string
    ConfigKeys map[string][]byte
    Session *session.Session // Configured elsewhere.
}

var configs map[string]Config = map[string]Config {
    "beta": Config{
        Name: "beta",
        LogDirectory: "/home/ec2-user/hansa/logs",
        ServerPort: 9000,
        ServerDNS: "hansa.coryandjill.com",
        RdsUser: "hansaiam",
        RdsHost: "beta-cpoker-database.cxodejzp6kpa.us-west-2.rds.amazonaws.com",
        RdsPort: "5432",
        RdsName: "hansabeta",
        S3Bucket: "chansas",
        S3Folder: "beta",
        EmailSender: "CHansas Dev<no-reply@hansa.coryandjill.com>",
        AwsRole: "arn:aws:iam::042697826304:role/cpokerHostsS3Access",
        ConfigKeys: map[string][]byte{},
    },
    "prod": Config{
        Name: "prod",
        LogDirectory: "/home/ec2-user/hansa/logs",
        ServerPort: 9000,
        ServerDNS: "chansas.com",
        RdsUser: "hansaiam",
        RdsHost: "beta-cpoker-database.cxodejzp6kpa.us-west-2.rds.amazonaws.com",
        RdsPort: "5432",
        RdsName: "hansaprod",
        S3Bucket: "chansas",
        S3Folder: "prod",
        EmailSender: "CHansas<no-reply@chansas.com>",
        AwsRole: "arn:aws:iam::042697826304:role/cpokerHostsS3Access",
        ConfigKeys: map[string][]byte{},
    },
}

func LoadConfig(filename string) Config {
    configBytes, err := ioutil.ReadFile(filename)

    now := time.Now().Format("2006-01-02T15:04:05.000Z")
    fmt.Printf("\n\n\n%s: Server Start\n", now)
    if err != nil {
        fmt.Printf("%s: LoadConfig err reading '%s', goodbye: %s\n", now, filename, err)
        os.Exit(1)
    }

    stackName := ""
    configVars := strings.TrimSpace(string(configBytes))
    for _, cfg := range strings.Split(configVars, "\n") {
        parts := strings.Split(cfg, "=")
        if parts[0] == "stack" {
            stackName = parts[1]
            break
        }
    }
    if stackName == "" {
        now := time.Now().Format("2006-01-02T15:04:05.000Z")
        fmt.Printf("%s: LoadConfig found no 'stack' in config.  goodbye.", now)
        os.Exit(1)
    }

    stack, ok := configs[stackName]
    if !ok {
        now := time.Now().Format("2006-01-02T15:04:05.000Z")
        fmt.Printf("%s: LoadConfig config unknown stack '%s' set in '%s', goodbye.\n", now, stack, filename)
        os.Exit(1)
    }

    for _, cfg := range strings.Split(configVars, "\n") {
        parts := strings.Split(cfg, "=")
        if parts[0] == "email" || parts[0] == "cookie" {
            stack.ConfigKeys[parts[0]] = DecodeString(parts[1])
        } else {
            stack.ConfigKeys[parts[0]] = []byte(parts[1])
        }
    }

    now = time.Now().Format("2006-01-02T15:04:05.000Z")
    fmt.Printf("%s: LoadConfig '%s'\n", now, stackName)
    return stack
}

func DecodeString(s string) (r []byte) {
    r, _ = hex.DecodeString(s)
    return
}

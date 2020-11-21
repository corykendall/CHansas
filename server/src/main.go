package main

import (
    "time"
    "math/rand"
    "syscall"
    "github.com/aws/aws-sdk-go/aws/session"
    "local/hansa/server"
    "local/hansa/log"
    "local/hansa/simple"
)

func main() {
    t := time.Now().UTC().UnixNano()
    rand.Seed(t)

    config := simple.LoadConfig("/home/ec2-user/hansa/server.cfg")
    config.Session = awsSession()

    logOverrides := make(map[string]log.LogLevel)
    pgid, _ := syscall.Getpgid(syscall.Getpid())
    log.Init(config.LogDirectory, log.DebugLevel, logOverrides)

    log.Info("********************************************************************")
    log.Info("*                                                                  *")
    log.Info("*                           Server Start                           *")
    log.Info("*                                                                  *")
    log.Info("********************************************************************")
    log.Info("Log Initialized")
    log.Debug("Seed: %d", t)
    log.Debug("Pgid: %d", pgid)

    server.New(config).Run()
}

func awsSession() *session.Session {
  return session.Must(session.NewSessionWithOptions(session.Options{
      SharedConfigState: session.SharedConfigEnable,
  }))
}

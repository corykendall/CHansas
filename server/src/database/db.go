package database

import(
    "database/sql"
    "fmt"
    "sync"
	"github.com/jmoiron/sqlx"
    "github.com/aws/aws-sdk-go/aws/awserr"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "local/hansa/simple"
    "local/hansa/log"
)

type DB struct {
    config simple.Config
    ddb *dynamodb.DynamoDB
    c *sqlx.DB
    counterMux sync.Mutex
    counters map[string]*counter
}

func New(config simple.Config) *DB {
    return &DB{
        config: config,
        ddb: dynamodb.New(config.Session),
        c: nil,
        counterMux: sync.Mutex{},
        counters: make(map[string]*counter),
    }
}

func (db *DB) Run(initDone chan struct{}) error {
    conn, err := db.RdsConnect()
    if err != nil {
        db.errorf("Unable to connect to rdb: %s", err)
        return err
    }
    db.c = conn
    db.infof("Connected to rdb: %s", db.config.RdsHost)

    initDone <- struct{}{}
    return nil
}

func (db *DB) exec(sql string, args ...interface{}) (sql.Result, error) {
    result, err := db.c.Exec(sql, args...)
    return result, err
}

func (db *DB) uidUsernameToIdentity(uid string, username *string) simple.Identity {
    if username != nil {
        return simple.NewConnectionIdentity(uid, *username)
    } else if uid[0] == 'G' {
        return simple.NewGuestIdentity(uid)
    } else if uid[0] == 'B' {
        r, _ := botidentities[uid]
        return r
    }
    panic(fmt.Sprintf("Can't convert uid=%s username=nil to Identity", uid))
}

func (db *DB) formatDDBError(err error) string {
    if aerr, ok := err.(awserr.Error); ok {
        switch aerr.Code() {
        case dynamodb.ErrCodeConditionalCheckFailedException:
            return fmt.Sprintln(dynamodb.ErrCodeConditionalCheckFailedException, aerr.Error())
        case dynamodb.ErrCodeProvisionedThroughputExceededException:
            return fmt.Sprintln(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
        case dynamodb.ErrCodeResourceNotFoundException:
            return fmt.Sprintln(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
        case dynamodb.ErrCodeItemCollectionSizeLimitExceededException:
            return fmt.Sprintln(dynamodb.ErrCodeItemCollectionSizeLimitExceededException, aerr.Error())
        case dynamodb.ErrCodeTransactionConflictException:
            return fmt.Sprintln(dynamodb.ErrCodeTransactionConflictException, aerr.Error())
        case dynamodb.ErrCodeRequestLimitExceeded:
            return fmt.Sprintln(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
        case dynamodb.ErrCodeInternalServerError:
            return fmt.Sprintln(dynamodb.ErrCodeInternalServerError, aerr.Error())
        default:
            return fmt.Sprintln(aerr.Error())
        }
    } else {
        // Print the error, cast err to awserr.Error to get the Code and
        // Message from an error.
        return fmt.Sprintln(err.Error())
    }
}

func min (x, y int) int {
    if x < y {
        return x
    }
    return y
}

func (db *DB) tracef(msg string, fargs ...interface{}) {
    log.Trace(msg, fargs...)
}

func (db *DB) debugf(msg string, fargs ...interface{}) {
    log.Debug(msg, fargs...)
}

func (db *DB) infof(msg string, fargs ...interface{}) {
    log.Info(msg, fargs...)
}

func (db *DB) warnf(msg string, fargs ...interface{}) {
    log.Warn(msg, fargs...)
}
func (db *DB) errorf(msg string, fargs ...interface{}) {
    log.Error(msg, fargs...)
}

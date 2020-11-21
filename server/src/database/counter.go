package database

import(
    "fmt"
    "strconv"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/service/dynamodb"
)

type counter struct {
    id int
    maxid int
}

func (db *DB) GetHandId() (id int, err error) {
    id, err = db.getCounter("hand", "H", 10)
    return
}

func (db *DB) getNewPlayerId() (id int, err error) {
    id, err = db.getCounter("player", "P", 1)
    return
}

func (db *DB) GetNewGameId() (id int, err error) {
    id, err = db.getCounter("game", "M", 1)
    return
}

func (db *DB) getCounter(name string, ddbname string, reserve int) (id int, err error) {
    db.counterMux.Lock()
    defer func() { db.counterMux.Unlock() }()

    c, ok := db.counters[name]
    if ok && c.id < c.maxid {
        id = c.id
        c.id++
        return 
    }
    if !ok {
        c = &counter{id:0, maxid:0}
        db.counters[name] = c
    }

    result, err := db.ddb.UpdateItem(&dynamodb.UpdateItemInput{
        ReturnValues: aws.String("UPDATED_OLD"),
        TableName: aws.String(fmt.Sprintf("%s-Hansa-Counters", db.config.Name)),
        UpdateExpression: aws.String("SET V = V + :i"),
        Key: map[string]*dynamodb.AttributeValue{"H":{S: aws.String(ddbname)}},
        ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
            ":i":{N:aws.String(fmt.Sprintf("%d", reserve))}},
    })
    if err != nil {
        db.errorf(db.formatDDBError(err))
        return
    }

    id, err = strconv.Atoi(*result.Attributes["V"].N)
    if err != nil {
        db.errorf("Error parsing counter from DDB: %s", err)
        return
    }
    c.id = id+1
    c.maxid = id+reserve
    return
}

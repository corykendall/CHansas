package database

import (
    "errors"
    "fmt"
    "strings"
    "golang.org/x/crypto/bcrypt"
    "local/hansa/simple"
    "local/hansa/log"
)

type Login struct {
    Email string `db:"email"`
    Username string `db:"username"`
    Password []byte  `db:"password"`
    Id string `db:"id"`
    Verified bool `db:"verified"`
}

func (db *DB) Signup(email string, username string, pw []byte) (error, error) {
    email = strings.ToLower(email)

    exist, err := db.EmailExists(email)
    if err != nil {
        db.errorf("Unable to select from db: %s", err)
        return nil, err
    }
    if exist {
        return errors.New("email"), nil
    }

    err = db.c.QueryRow(
        "select exists(select 1 from login where username=$1) as \"exists\"",
        username).Scan(&exist)
    if err != nil {
        db.errorf("Unable to select from db: %s", err)
        return nil, err
    }
    if exist {
        return errors.New("username"), nil
    }

    // Now we're pretty confident, let's generate them a player id.
    id, err := db.getNewPlayerId()
    if err != nil {
        db.errorf("Unable to getNewPlayerId() from ddb: %s", err)
        return nil, err
    }
    idString := fmt.Sprintf("P%d", id)
    db.infof("Reserved new PlayerId: %s", idString)

    _, err = db.c.Exec("insert into login values ($1, $2, $3, $4, false)",
        email, username, pw, idString)
    if err != nil {
        db.errorf("Unable to insert into login table in db: %s", err)
        return nil, err
    }
    return nil, nil
}

func (db *DB) Signin(email string, pw []byte) (id string, err error, dberr error) {
    email = strings.ToLower(email)
    log.Debug("Signin attempt for %s", email)

    rows, dberr := db.c.Queryx("select * from login where email=$1", email)
    if dberr != nil {
        db.errorf("Unable to select from db: %s", dberr)
        return
    }

    var l Login
    count := 0
    for rows.Next() {
        dberr = rows.StructScan(&l)
        count++
        if dberr != nil {
            db.errorf("Error scanning into Login: %s", dberr)
            return
        }
    }
    if count == 0 || l.Email != email {
        log.Debug("Signin attempt for %s: unknown email", email)
        err = errors.New("email")
        return
    }

    if notok := bcrypt.CompareHashAndPassword(l.Password, pw); notok != nil {
        log.Debug("Signin attempt for %s: bad password", email)
        err = errors.New("password")
        return
    }

    if !l.Verified {
        log.Debug("Signin attempt for %s: not verified", email)
        err = errors.New("verified")
        return
    }

    id = l.Id
    log.Debug("Signin attempt for %s: success (%s)", email, id)
    return
}

var botidentities map[string]simple.Identity = map[string]simple.Identity{
    "B1": simple.NewBotIdentity("B1", "Bob"),
}

func (db *DB) GetIdentity(id string) (simple.Identity, bool) {
    if id[0] == 'G' {
        return simple.NewIdentity(
            id, fmt.Sprintf("Guest%s", id[1:]), simple.IdentityTypeGuest), true
    } else if id[0] == 'B' {
        v, ok := botidentities[id]
        return v, ok
    } else if id[0] == 'P' {
        rows, dberr := db.c.Queryx("select * from login where id=$1", id)
        if dberr != nil {
            db.errorf("Unable to select from db: %s", dberr)
            return simple.EmptyIdentity, false
        }

        var l Login
        for rows.Next() {
            dberr = rows.StructScan(&l)
            if dberr != nil {
                db.errorf("Error Scanning into Login: %s", dberr)
                return simple.EmptyIdentity, false
            }
        }

        return simple.NewIdentity(
            l.Id, l.Username, simple.IdentityTypeConnection), true
    }

    db.errorf("Unknown Identity type: '%c'", id[0])
    return simple.EmptyIdentity, false
}

func (db *DB) GetIdentityFromEmail(email string) (simple.Identity, bool) {
    email = strings.ToLower(email)
    rows, dberr := db.c.Queryx("select * from login where email=$1", email)
    if dberr != nil {
        db.errorf("Unable to select from db: %s", dberr)
        return simple.EmptyIdentity, false
    }

    var l Login
    for rows.Next() {
        dberr = rows.StructScan(&l)
        if dberr != nil {
            db.errorf("Error Scanning into Login: %s", dberr)
            return simple.EmptyIdentity, false
        }
    }

    return simple.NewIdentity(
        l.Id, l.Username, simple.IdentityTypeConnection), true
}

func (db *DB) GetIdentityForUsername(username string) simple.Identity {
    rows, err := db.c.Query("select id from login where username=$1", username)
    if err != nil {
        db.errorf("GetIdentityForUsername: Unable to select from login: %s", err)
        return simple.EmptyIdentity
    }

    var uid string
    for rows.Next() {
        err = rows.Scan(&uid)
        if err != nil {
            db.errorf("GetIdentityForUsername: Unable to scan from login: %s", err)
            return simple.EmptyIdentity
        }
    }

    if uid == "" {
        return simple.EmptyIdentity
    }

    return simple.NewConnectionIdentity(uid, username)
}


func (db *DB) ConfirmEmail(email string) error {
    email = strings.ToLower(email)
    _, err := db.c.Exec("update login set verified=true where email=$1", email)
    return err
}

func (db *DB) SetPassword(id string, pw []byte) error {
    db.debugf("Updating password for %s", id)
    _, err := db.c.Exec("update login set password=$1 where id=$2", pw, id)
    return err
}

func (db *DB) EmailExists(email string) (exist bool, err error){
    email = strings.ToLower(email)
    err = db.c.QueryRow(
        "select exists(select 1 from login where email=$1) as \"exists\"",
        email).Scan(&exist)
    return
}


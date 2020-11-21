package database

// This is pulled from here with very few modifications:
// https://github.com/califlower/golang-aws-rds-iam-postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net"
	"strings"
	"time"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/aws/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/rds/rdsutils"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/jackc/pgx/v4/stdlib"
	"github.com/jmoiron/sqlx"
	"golang.org/x/xerrors"
    "local/hansa/log"
    "local/hansa/simple"
)

type Rds struct {
    config simple.Config
}

// Just call this to get a new sqlx to postgres rdb.
func (db *DB) RdsConnect() (*sqlx.DB, error) {
	conn := sql.OpenDB(&Rds{config: db.config})
    conn.SetMaxOpenConns(20)
	err := conn.Ping()
	if err != nil {
		return nil, xerrors.Errorf("could not ping db: %w", err)
	}

	return sqlx.NewDb(conn, "pgx"), nil
}

// If not set, database can hang for an extremely long time trying to open a
// new connection
const databaseConnectionTimeoutMilliseconds = 5000

// We call this to get a new IAM token
func getAuthToken(region, cname, port, user, arn string) (string, error) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return "", xerrors.Errorf("could not connect to db using iam auth: %w", err)
	}

	cfg.Region = region
	credProvider := stscreds.NewAssumeRoleProvider(sts.New(cfg), arn)
	signer := v4.NewSigner(credProvider)

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), databaseConnectionTimeoutMilliseconds*time.Millisecond)
	defer cancel()

	authToken, _ := rdsutils.BuildAuthToken(ctxWithTimeout,
		fmt.Sprintf("%s:%s", cname, port),
		region, user, signer)
	return authToken, err
}

// golang database/sql will call this
func (rds *Rds) Connect(ctx context.Context) (driver.Conn, error) {
	connectionString, err := rds.getConnectionString()
	if err != nil {
		return nil, xerrors.Errorf("could not get connection string: %w", err)
	}

	pgxConnector := &stdlib.Driver{}
	connector, err := pgxConnector.OpenConnector(connectionString)
	if err != nil {
		return nil, err
	}

	return connector.Connect(context.Background())
}

// golang database/sql will call this
func (rds *Rds) Driver() driver.Driver {
	return rds
}

// golang database/sql will not call this, but we have to implement
func (rds *Rds) Open(name string) (driver.Conn, error) {
	return nil, xerrors.New("driver open method unsupported")
}

// golang database/sql will call this
func (rds *Rds) getConnectionString() (string, error) {
	cnameUntrimmed, err := net.LookupCNAME(rds.config.RdsHost)
	if err != nil {
		log.Error("could not lookup cname during iam auth: %v", err)
		return "", xerrors.Errorf("could not lookup cname during iam auth: %w", err)
	}

	//Trim the trailing dot from the cname
	cname := strings.TrimRight(cnameUntrimmed, ".")
	splitCname := strings.Split(cname, ".")

	if len(splitCname) != 6 {
		return "", xerrors.New(fmt.Sprintf("cname not in AWS format, cname:%s ", cname))
	}

	region := splitCname[2]
	authToken, err := getAuthToken(region, cname, rds.config.RdsPort, rds.config.RdsUser, rds.config.AwsRole)
	if err != nil {
		return "", xerrors.Errorf("could not build auth token: %w", err)
	}

	var postgresConnection strings.Builder
	postgresConnection.WriteString(
		fmt.Sprintf("user=%s dbname=%s sslmode=%s port=%s host=%s password=%s",
			rds.config.RdsUser,
			rds.config.RdsName,
			"require",
			rds.config.RdsPort,
			cname,
			authToken))

	return postgresConnection.String(), nil
}


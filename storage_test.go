package dynamodbstorage

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/certmagic"
)

const TestTableName = "CertMagicTest"
const DisableSSL = true

func initDb() error {
	storage := Storage{
		TableName:     TestTableName,
		AwsEndpoint:   os.Getenv("AWS_ENDPOINT"),
		AwsRegion:     os.Getenv("AWS_DEFAULT_REGION"),
		AwsDisableSSL: true,
	}
	sess, err := session.NewSession(&aws.Config{
		Endpoint:   &storage.AwsEndpoint,
		Region:     &storage.AwsRegion,
		DisableSSL: &storage.AwsDisableSSL,
	})
	if err != nil {
		return err
	}

	svc := dynamodb.New(sess)

	// attempt to delete table in case already exists
	deleteTable := &dynamodb.DeleteTableInput{
		TableName: aws.String(storage.TableName),
	}
	_, err = svc.DeleteTable(deleteTable)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeResourceNotFoundException:
				// this is fine
			default:
				return aerr
			}
		} else {
			return err
		}
	}

	// create table
	createTable := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("PrimaryKey"),
				AttributeType: aws.String("S"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("PrimaryKey"),
				KeyType:       aws.String("HASH"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(3),
			WriteCapacityUnits: aws.Int64(3),
		},
		TableName: aws.String(storage.TableName),
	}
	_, err = svc.CreateTable(createTable)
	return err
}

func TestDynamoDBStorage_initConfg(t *testing.T) {
	type fields struct {
		TableName     string
		KeyPrefix     string
		ColumnName    string
		AwsEndpoint   string
		AwsRegion     string
		AwsDisableSSL bool
	}
	tests := []struct {
		name     string
		fields   fields
		wantErr  bool
		expected *Storage
	}{
		{
			name:     "defaults - should error with empty table",
			fields:   fields{},
			wantErr:  true,
			expected: &Storage{},
		},
		{
			name: "defaults - provide only table name",
			fields: fields{
				TableName: "Testing123",
			},
			wantErr: false,
			expected: &Storage{
				TableName:             "Testing123",
				LockTimeout:           defaultLockTimeoutMinutes,
				LockFreshnessInterval: defaultLockFreshnessInterval,
				LockPollingInterval:   defaultLockPollingInterval,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Storage{
				TableName:     tt.fields.TableName,
				AwsEndpoint:   tt.fields.AwsEndpoint,
				AwsRegion:     tt.fields.AwsRegion,
				AwsDisableSSL: tt.fields.AwsDisableSSL,
			}
			if err := s.initConfig(); (err != nil) != tt.wantErr {
				t.Errorf("initConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// unset complicated objects not requiring tests
			s.Dynamo = tt.expected.Dynamo
			s.Table = tt.expected.Table
			if !reflect.DeepEqual(tt.expected, s) {
				t.Errorf("Expected does not match actual: %+v != %+v", tt.expected, s)
			}
		})
	}
}

func TestDynamoDBStorage_Store(t *testing.T) {
	err := initDb()
	if err != nil {
		t.Error(err)
		return
	}

	type fields struct {
		TableName     string
		KeyPrefix     string
		ColumnName    string
		AwsEndpoint   string
		AwsRegion     string
		AwsDisableSSL bool
	}
	type args struct {
		key   string
		value []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "simple key/value store",
			fields: fields{
				TableName:     TestTableName,
				AwsEndpoint:   os.Getenv("AWS_ENDPOINT"),
				AwsRegion:     os.Getenv("AWS_DEFAULT_REGION"),
				AwsDisableSSL: DisableSSL,
			},
			args: args{
				key:   "simple-key",
				value: []byte("value"),
			},
			wantErr: false,
		},
		{
			name: "empty key should error",
			fields: fields{
				TableName:     TestTableName,
				AwsEndpoint:   os.Getenv("AWS_ENDPOINT"),
				AwsRegion:     os.Getenv("AWS_DEFAULT_REGION"),
				AwsDisableSSL: DisableSSL,
			},
			args: args{
				key:   "",
				value: []byte("value"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Storage{
				TableName:     tt.fields.TableName,
				AwsEndpoint:   tt.fields.AwsEndpoint,
				AwsRegion:     tt.fields.AwsRegion,
				AwsDisableSSL: tt.fields.AwsDisableSSL,
			}
			err := s.Store(tt.args.key, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Store() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				loaded, err := s.Load(tt.args.key)
				if err != nil {
					t.Errorf("failed to load after store: %s", err.Error())
					return
				}
				if string(loaded) != string(tt.args.value) {
					t.Errorf("Load() returned value other than expected. Expected: %s, Actual: %s", string(tt.args.value), string(loaded))
					return
				}
			}
		})
	}
}

func TestDynamoDBStorage_List(t *testing.T) {
	err := initDb()
	if err != nil {
		t.Error(err)
		return
	}

	storage := Storage{
		TableName:     TestTableName,
		AwsEndpoint:   os.Getenv("AWS_ENDPOINT"),
		AwsRegion:     os.Getenv("AWS_DEFAULT_REGION"),
		AwsDisableSSL: DisableSSL,
	}

	prefix := "domain"

	fixturesWithPrefix := map[string]string{
		"domain1": "cert1",
		"domain2": "cert2",
		"domain3": "cert3",
	}
	for k, v := range fixturesWithPrefix {
		err := storage.Store(k, []byte(v))
		if err != nil {
			t.Errorf("failed to store fixture %s, error: %s", k, err.Error())
			return
		}
	}

	fixtures := map[string]string{
		"notinlist":        "cert4",
		"anothernotinlist": "cert5",
	}
	for k, v := range fixtures {
		err := storage.Store(k, []byte(v))
		if err != nil {
			t.Errorf("failed to store fixture %s, error: %s", k, err.Error())
			return
		}
	}

	foundKeys, err := storage.List(prefix, false)
	if err != nil {
		t.Errorf("failed to list keys: %s", err.Error())
		return
	}

	if len(foundKeys) != len(fixturesWithPrefix) {
		t.Errorf("did not get back expected number of keys, expected: %v, got: %v",
			len(fixturesWithPrefix), len(foundKeys))
		return
	}

	noKeysFound, err := storage.List("invalid", false)
	if err != nil {
		t.Errorf("unable to list keys with invalid prefix: %s", err.Error())
		return
	}

	if len(noKeysFound) != 0 {
		t.Errorf("should not have found any keys but found %v key", len(noKeysFound))
		return
	}
}

func TestDynamoDBStorage_Stat(t *testing.T) {
	err := initDb()
	if err != nil {
		t.Error(err)
		return
	}

	storage := Storage{
		TableName:     TestTableName,
		AwsEndpoint:   os.Getenv("AWS_ENDPOINT"),
		AwsRegion:     os.Getenv("AWS_DEFAULT_REGION"),
		AwsDisableSSL: DisableSSL,
	}

	err = storage.Store("key", []byte("value"))
	if err != nil {
		t.Errorf("failed to store fixture key/value: %s", err.Error())
		return
	}

	stat, err := storage.Stat("key")
	if err != nil {
		t.Errorf("failed to stat item: %s", err.Error())
		return
	}

	if stat.Key != "key" {
		t.Errorf("stat key does not match expected. got: %s", stat.Key)
		return
	}
	if stat.Size != int64(len("value")) {
		t.Errorf("stat size does not match expected. got: %v", stat.Size)
		return
	}
	if time.Since(stat.Modified) > 5*time.Second {
		t.Errorf("stat modified time is not within 3 seoncds. got: %s", stat.Modified)
		return
	}
}

func TestDynamoDBStorage_Delete(t *testing.T) {
	err := initDb()
	if err != nil {
		t.Error(err)
		return
	}

	storage := Storage{
		TableName:     TestTableName,
		AwsEndpoint:   os.Getenv("AWS_ENDPOINT"),
		AwsRegion:     os.Getenv("AWS_DEFAULT_REGION"),
		AwsDisableSSL: DisableSSL,
	}

	err = storage.Store("key", []byte("value"))
	if err != nil {
		t.Errorf("failed to store fixture key/value: %s", err.Error())
		return
	}

	value, err := storage.Load("key")
	if err != nil {
		t.Errorf("unable to load key that was just stored: %s", err.Error())
		return
	}

	if string(value) != "value" {
		t.Errorf("value returned does not match expected. expected: %s, got: %s", "value", string(value))
		return
	}

	err = storage.Delete("key")
	if err != nil {
		t.Errorf("unable to delete key: %s", err.Error())
		return
	}

	if storage.Exists("key") {
		t.Errorf("key still exists after delete")
		return
	}

}

func TestDynamoDBStorage_Lock(t *testing.T) {
	err := initDb()
	if err != nil {
		t.Error(err)
		return
	}

	lockTimeout := 1 * time.Second

	storage := Storage{
		TableName:     TestTableName,
		AwsEndpoint:   os.Getenv("AWS_ENDPOINT"),
		AwsRegion:     os.Getenv("AWS_DEFAULT_REGION"),
		AwsDisableSSL: DisableSSL,
		LockTimeout:   caddy.Duration(lockTimeout),
	}

	// create lock
	key := "test1"
	err = storage.Lock(context.TODO(), key)
	if err != nil {
		t.Errorf("error creating lock: %s", err.Error())
	}

	// try to create lock again, it should take about 1-2 seconds to return
	before := time.Now()
	err = storage.Lock(context.TODO(), key)
	if err != nil {
		t.Errorf("error creating lock second time: %s", err.Error())
	}
	if time.Since(before) < lockTimeout {
		t.Errorf("creating second lock finished quicker than it shoud, in %v seconds", time.Since(before).Seconds())
	}

	// try to unlock a key that doesn't exist
	err = storage.Unlock("doesntexist")
	if err != nil {
		t.Errorf("got error unlocking non-existant key")
	}
}

func TestDynamoDBStorage_LoadErrNotExist(t *testing.T) {
	err := initDb()
	if err != nil {
		t.Error(err)
		return
	}

	storage := Storage{
		TableName:     TestTableName,
		AwsEndpoint:   os.Getenv("AWS_ENDPOINT"),
		AwsRegion:     os.Getenv("AWS_DEFAULT_REGION"),
		AwsDisableSSL: DisableSSL,
	}

	_, err = storage.Load("notarealkey")
	_, isNotErrNotExist := err.(certmagic.ErrNotExist)
	if !isNotErrNotExist {
		t.Errorf("err was not a ErrNotExist, got: %s", err.Error())
	}
}

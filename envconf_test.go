package envconf

import (
	"reflect"
	"testing"
)

type ServerConfig struct {
	Addr          string
	ID            int32
	Key           string `env:"-"`
	Tags          []string
	EnableTLS     bool   `env:"enable_tls"`
	KeyFile       string `env:"key_file"`
	CertFile      string `env:"cert_file"`
	StorageConfig `env:"storage"`
}

type StorageConfig struct {
	Hosts    []string `env:"hosts"`
	MaxConns int      `env:"max_conns"`
	Timeouts StorageTimeouts
}

type StorageTimeouts struct {
	Send    int64
	Receive int64 `env:"Recv"`
}

var environ = []string{
	"SERVER_ADDR= :8080",
	"SERVER_ID=123",
	"SERVER_KEY=uQ/OEEc0kFCthCHm9iyorw==",
	"SERVER_ENABLE_TLS=1",
	"SERVER_KEY_FILE=./server.key",
	"SERVER_Cert_File=./server.crt",
	`SERVER_TAGS=ren,stimpy,hapi\, hapi\, joi\, joi`,
	"SERVER_STORAGE_HOSTS=[::1]:6160,127.0.0.1:6160,:6160",
	"server_storage_max_conns=500",
	"server_storage_Timeouts_Send=5\t",
	"server_storage_timeouts_recv=10",
	"USER",
}

var expected = &ServerConfig{
	Addr:      ":8080",
	ID:        123,
	Tags:      []string{"ren", "stimpy", "hapi, hapi, joi, joi"},
	EnableTLS: true,
	KeyFile:   "./server.key",
	CertFile:  "./server.crt",
	StorageConfig: StorageConfig{
		Hosts:    []string{"[::1]:6160", "127.0.0.1:6160", ":6160"},
		MaxConns: 500,
		Timeouts: StorageTimeouts{
			Send:    5,
			Receive: 10,
		},
	},
}

type getTest struct {
	name  string
	key   string
	value string
	ok    bool
}

var getTests = []getTest{
	{"lowercase name", "server_id", "123", true},
	{"uppercase name", "SERVER_STORAGE_TIMEOUTS_RECV", "10", true},
	{"mixed-case name", "Server_Cert_File", "./server.crt", true},
	{"leading whitespace in value", "server_addr", ":8080", true},
	{"trailing whitespace in value", "server_storage_timeouts_send", "5", true},
	{`"=" in value`, "server_key", "uQ/OEEc0kFCthCHm9iyorw==", true},
	{"nonexistent variable", "PATH", "", false},
	{"missing variable value", "USER", "", false},
}

func TestGet(t *testing.T) {
	env := New(environ)
	for _, test := range getTests {
		value, ok := env.Get(test.key)
		if value != test.value {
			t.Errorf("On test %v, unexpected value: got %#v; want %#v", test.name, value, test.value)
		}
		if ok != test.ok {
			t.Errorf("On test %v, unexpected existence test: got %#v; want %#v", test.name, ok, test.ok)
		}
	}
}

func TestDecode(t *testing.T) {
	env := New(environ)
	actual := new(ServerConfig)
	if err := env.Decode("server", "_", actual); err != nil {
		t.Errorf("Error decoding environment: %#v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Unexpected result: got %#v; want %#v", actual, expected)
	}
}

func TestDecodeStrict(t *testing.T) {
	env := New(environ)
	actual := new(ServerConfig)
	if err := env.DecodeStrict("server", "_", actual, nil); err == nil {
		t.Errorf("Expected error decoding environment, got %#v", err)
	}
	ignoreEnv := map[string]interface{}{"server_key": true}
	if err := env.DecodeStrict("server", "_", actual, ignoreEnv); err != nil {
		t.Errorf("Unexpected error decoding environment: %#v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Unexpected result: got %#v; want %#v", actual, expected)
	}
}

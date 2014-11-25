package envconf

import (
	"reflect"
	"testing"
	"time"
)

type ServerConfig struct {
	Addr          string
	ID            int32
	Key           string `env:"-"`
	LogID         []byte `env:"LOG_ID"`
	Tags          []string
	EnableTLS     bool   `env:"enable_tls"`
	KeyFile       string `env:"key_file"`
	CertFile      string `env:"cert_file"`
	StorageConfig `env:"storage"`
}

type emptyInterface struct {
	Required   interface{}
	Ignored    interface{} `env:"-"`
	unexported interface{}
}

type engine struct {
	next, prev *engine
}

type StorageConfig struct {
	engines    *engine
	Hosts      []string `env:"hosts"`
	conns      int
	MaxConns   int           `env:"max_conns"`
	RetryDelay time.Duration `env:"retry_delay"`
	Timeouts   StorageTimeouts
}

type StorageTimeouts struct {
	*Channels
	Send    int64
	Receive int64 `env:"Recv"`
}

type Channels struct {
	Registered string `env:"REG"`
	Active     string
	Deleted    string `env:"del"`
}

var hyphenEnv = []string{
	"ADDR= :8080",
	"ID=123",
	"KEY=uQ/OEEc0kFCthCHm9iyorw==",
	"log_id=UbH4XJytSLSMBOYdyR9+7w==",
	"ENABLE_TLS=1",
	"KEY_FILE=./server.key",
	"Cert_File=./server.crt",
	`TAGS=ren,stimpy,hapi\, hapi\, joi\, joi`,
	"STORAGE-HOSTS=[::1]:6160,127.0.0.1:6160,:6160",
	"storage-max_conns=500",
	"STORAGE-CONNS=10",
	"STORAGE-RETRY_DELAY=5s",
	"storage-Timeouts-Send=5\t",
	"storage-timeouts-recv=10",
	"STORAGE-TIMEOUTS-REG=3h",
	"Storage-Timeouts-Active=72h",
	"storage-timeouts-del=24h",
	"USER",
}

var environ = []string{
	"SERVER_ADDR= :8080",
	"SERVER_ID=123",
	"SERVER_KEY=uQ/OEEc0kFCthCHm9iyorw==",
	"server_log_id=UbH4XJytSLSMBOYdyR9+7w==",
	"SERVER_ENABLE_TLS=1",
	"SERVER_KEY_FILE=./server.key",
	"SERVER_Cert_File=./server.crt",
	`SERVER_TAGS=ren,stimpy,hapi\, hapi\, joi\, joi`,
	"SERVER_STORAGE_HOSTS=[::1]:6160,127.0.0.1:6160,:6160",
	"server_storage_max_conns=500",
	"SERVER_STORAGE_CONNS=10",
	"SERVER_STORAGE_RETRY_DELAY=5s",
	"server_storage_Timeouts_Send=5\t",
	"server_storage_timeouts_recv=10",
	"SERVER_STORAGE_TIMEOUTS_REG=3h",
	"Server_Storage_Timeouts_Active=72h",
	"server_storage_timeouts_del=24h",
	"USER",
}

var expected = &ServerConfig{
	Addr: ":8080",
	ID:   123,
	LogID: []byte{0x51, 0xb1, 0xf8, 0x5c, 0x9c, 0xad, 0x48, 0xb4, 0x8c,
		0x04, 0xe6, 0x1d, 0xc9, 0x1f, 0x7e, 0xef},
	Tags:      []string{"ren", "stimpy", "hapi, hapi, joi, joi"},
	EnableTLS: true,
	KeyFile:   "./server.key",
	CertFile:  "./server.crt",
	StorageConfig: StorageConfig{
		Hosts:      []string{"[::1]:6160", "127.0.0.1:6160", ":6160"},
		MaxConns:   500,
		RetryDelay: 5 * time.Second,
		Timeouts: StorageTimeouts{
			Channels: &Channels{
				Registered: "3h",
				Active:     "72h",
				Deleted:    "24h",
			},
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
			t.Errorf("On test %v, unexpected value: got %#v; want %#v",
				test.name, value, test.value)
		}
		if ok != test.ok {
			t.Errorf("On test %v, unexpected existence test: got %#v; want %#v",
				test.name, ok, test.ok)
		}
	}
}

func TestDecodeNil(t *testing.T) {
	var (
		nilConfig interface{}
		srvConfig *ServerConfig
	)
	env := New(environ)
	if err := env.Decode("server", "_", nilConfig); err == nil {
		t.Errorf("Expected invalid value error decoding into invalid value %#v",
			nilConfig)
	}
	if err := env.Decode("server", "_", &srvConfig); err != nil {
		t.Errorf("Error decoding into nil pointer: %s", err)
	}
	if !reflect.DeepEqual(srvConfig, expected) {
		t.Errorf("Unexpected result decoding into nil pointer: got %#v; want %#v",
			srvConfig, expected)
	}
}

func TestDecodeEmptyInterface(t *testing.T) {
	var empty *emptyInterface
	env := New([]string{
		"required=1",
		"ignored=hello",
		"unexported=123",
	})
	if err := env.Decode("", "", &empty); err == nil {
		t.Errorf("Expected unsupported type error decoding into %#v", empty)
	}
}

func TestDecodeCustomSep(t *testing.T) {
	env := New(hyphenEnv)
	actual := new(ServerConfig)
	if err := env.Decode("", "-", actual); err != nil {
		t.Errorf("Error decoding hyphenated environment variables: %s", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Unexpected result: got %#v; want %#v", actual, expected)
	}
}

func TestDecode(t *testing.T) {
	env := New(environ)
	actual := new(ServerConfig)
	if err := env.Decode("server", "_", actual); err != nil {
		t.Errorf("Error decoding environment: %s", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Unexpected result: got %#v; want %#v", actual, expected)
	}
}

func TestDecodeStrict(t *testing.T) {
	env := New(environ)
	actual := new(ServerConfig)
	if err := env.DecodeStrict("server", "_", actual, nil); err == nil {
		t.Errorf("Expected field error decoding environment")
	}
	ignoreEnv := map[string]interface{}{"server_key": true}
	if err := env.DecodeStrict("server", "_", actual, ignoreEnv); err != nil {
		t.Errorf("Unexpected error decoding environment: %s", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Unexpected result: got %#v; want %#v", actual, expected)
	}
}

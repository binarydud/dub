package main

import (
	"fmt"
	"flag"
	"log"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"github.com/BurntSushi/toml"
	ba "github.com/binarydud/dub/backends"
)

const (
	colon   = ":"
	XRealIP = "X-Real-IP"
)
//Config Types
type Config struct {
	FrontendMap map[string]frontends `toml:"frontends"`
	BackendMap map[string]backends `toml:"backends"`
}

func (config *Config) BuildBackends(backends []string) ba.Backends {
	//for each key, get the backed details and put into roundrobin
	//backendList := make([]string, 0, len(backends))
	backendMap := make(map[string]string)
	for _,value := range backends {
		if backendValue, ok := config.BackendMap[value]; ok {
			backendMap[value] = backendValue.Host
		}
	}
	return ba.NewRoundRobin(backendMap)
}

func (config *Config) BuildFrontends() []Frontend {
	frontends := make([]Frontend, 0, len(config.FrontendMap))
	for serverName, server := range config.FrontendMap {
		backends := config.BuildBackends(server.Backends)
		frontends = append(frontends, Frontend{Name: serverName, Bind: server.Bind, Backends: backends})
	}
	return frontends
}
type frontends struct {
	Bind string
	Strategy string
	Backends []string
}

type backends struct {
	Host string
	Path string
}

func RealIP(req *http.Request) string {
    host, _, _ := net.SplitHostPort(req.RemoteAddr)
    return host
}

// Program Struct
type NoBackend struct{}

type Frontend struct {
	Name string
	Bind string
	Strategy string
	Backends ba.Backends
}
func (f *Frontend) Start() error {
	proxy := &Proxy{
		&httputil.ReverseProxy{Director: func(req *http.Request) {
			backend := f.Backends.Choose()
			if backend == nil {
				log.Printf("no backend for client %s", req.RemoteAddr)
				panic(NoBackend{})
			}
			log.Printf("serving to backend %s", backend.Name())
			req.URL.Scheme = "http"
			req.URL.Host = backend.Host()
			req.Header.Add(XRealIP, RealIP(req))
		}},
	}
	log.Printf("listening on %s, balancing", f.Bind)
	return http.ListenAndServe(f.Bind, proxy)
}
type Proxy struct {
    *httputil.ReverseProxy
}

func (p *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    defer func() {
        if err := recover(); err != nil {
            switch err.(type) {
            case NoBackend:
                rw.WriteHeader(503)
                req.Body.Close()
            default:
                panic(err)
            }
        }
    }()
    p.ReverseProxy.ServeHTTP(rw, req)
}



func main() {
	var config_file = flag.String("config", ".dub.toml", "dub config file path")
	flag.Parse()
	var config Config
	bs, err := ioutil.ReadFile(*config_file)
	fmt.Println(string(bs))
	if err != nil {
		panic(err)
	}
	if _, err := toml.Decode(string(bs), &config); err != nil {
		fmt.Println(err)
		return
	}
	count := 0
	frontends := config.BuildFrontends()
	exit_chan := make(chan int)
	for _, frontend := range frontends {
		go func(fe *Frontend) {
			fmt.Println("Starting Go Routine")
			if err := fe.Start(); err != nil{
				log.Printf("Starting frontend %s failed: %v", fe.Name, err)
			}
			exit_chan <- 1
		}(&frontend)
	}
	count++
	for i := 0; i < count; i++ {
		<-exit_chan
	}
}

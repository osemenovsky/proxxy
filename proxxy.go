package proxxy

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"
)

func closeConn(conn net.Conn) {
	err := conn.Close()
	if err != nil {
		log.Printf("Failed to close connection: %s", err)
	}
}

type Proxy struct {
	Url		string
	Username 	string
	Password	string
}

func (p *Proxy) GetAuthString() (string, error){
	if p.Username == "" || p.Password == "" {
		return "", fmt.Errorf("this proxy doesn't support or doesn't require auth")
	}
	auth := fmt.Sprintf("%s:%s", p.Username, p.Password)
	authString := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	return authString, nil
}
func (p *Proxy) ProxyRequest(w http.ResponseWriter, r http.Request)  {
	log.Printf("Proxying request")
}

type ProxyManager struct {
	proxies []Proxy
	cursor  int
	mux 	sync.Mutex
}

func (pm *ProxyManager) AddProxy(p Proxy) {
	pm.proxies = append(pm.proxies, p)
}

func (pm *ProxyManager) NextProxy() *Proxy {
	// Basic implementation
	pm.mux.Lock()
	proxy := pm.proxies[pm.cursor]

	pm.cursor++
	if pm.cursor >= len(pm.proxies) {
		pm.cursor = 0
	}

	pm.mux.Unlock()

	return &proxy
}

func GetAllProxies (proxyList string) []Proxy {
	// This is a temporary version of the function.
	proxies := []Proxy{}

	// Add all the fineproxy proxies from the list file if present
	if proxyList != "" {
		file, err := os.Open(proxyList)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			proxy := Proxy{Url: scanner.Text()}
			proxies = append(proxies, proxy)
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

	return proxies
}

func NewProxyManager (settings *Settings) *ProxyManager {
	proxyManager := new(ProxyManager)

	proxies := GetAllProxies(settings.ProxyList)
	for _, proxy := range proxies {
		proxyManager.AddProxy(proxy)
	}
	return proxyManager
}


type ProxyMux struct {
	Settings			Settings
	ProxyManager		ProxyManager
}

func (mux *ProxyMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Checking if authentication is required and authenticating client
	authHeader, err := mux.Settings.GetAuthHeader()
	requestAuthHeader := r.Header.Get("Proxy-Authorization")

	if requestAuthHeader != authHeader {
		w.WriteHeader(http.StatusForbidden)
	}
	// Removing original proxy-authorization header if present
	r.Header.Del("Proxy-Authorization")

	// Getting next proxy from ProxyManager
	proxy := *mux.ProxyManager.NextProxy()
	authString, err := proxy.GetAuthString()
	// If proxy is configured to use authorization add required header
	// Do nothing otherwise
	if err == nil {
		r.Header.Add("Proxy-Authorization", authString)
	}

	// Connect to chosen proxy
	// todo: Add either retries or healthchecks to proxy list (not critical)
	log.Printf("Got %s request to %s, forwarding to %s", r.Method, r.URL, proxy.Url)
	remoteConn, err := net.Dial("tcp", proxy.Url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		//defer remoteConn.Close()
		//defer closeConn(remoteConn)
	}
	// Forwarding edited client request to the end proxy
	err = r.WriteProxy(remoteConn)

	// Hijacking client connection from http library to pass it to the end proxy
	if err != nil {
		log.Printf("Failed to write request to remote proxy: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
	}
	// Getting client buffers
	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Printf("Failed to hijack connection: %s", err)
		http.Error(w, "failed to proxy connection", http.StatusInternalServerError)
	}

	// Proxying connection to the end proxy
	clientTcpConn := clientConn.(*net.TCPConn)
	remoteTcpConn := remoteConn.(*net.TCPConn)
	ProxyConnections(clientTcpConn, remoteTcpConn)

	//defer closeConn(clientConn)
	//defer clientConn.Close()

}

func Run() {
	settings := *MakeSettings()
	proxyManager := *NewProxyManager(&settings)

	mux := &ProxyMux{
		settings,
		proxyManager,
	}

	server := &http.Server{
		Addr: 			settings.GetListenOn(),
		Handler: 		mux,
		ReadTimeout: 	5 * time.Second,
		WriteTimeout:	10 * time.Second,
	}
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("uLimit is %d/%d", rLimit.Cur, rLimit.Max)

	server.SetKeepAlivesEnabled(false)

	errur := server.ListenAndServe()
	if errur != nil {
		log.Fatalf("Failed to start server: %s", errur)
	}
}

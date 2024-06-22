package proxxy

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
)

func GetDefaultEnv (key, defaultValue string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return value
}


type Settings struct {
	Host		string
	Port 		int16
	User		string
	Password	string
	ProxyList	string
}

func (s *Settings) GetListenOn () string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

func (s *Settings) GetAuthHeader () (string, error) {
	if s.Password == "" || s.User == "" {
		return "", fmt.Errorf("password have not been set")
	}

	auth := fmt.Sprintf("%s:%s", s.User, s.Password)
	authString := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	return authString, nil

}


func MakeSettings () *Settings {
	portString := GetDefaultEnv("PROXY_PORT", "8085")
	port64, err := strconv.ParseInt(portString, 10, 16)
	port := int16(port64)
	proxyList := GetDefaultEnv("PROXY_LIST", "")
	host := GetDefaultEnv("PROXY_HOST", "")
	user := GetDefaultEnv("PROXY_USER",  "user")
	password := GetDefaultEnv("PROXY_PASSWORD", "")
	if password != "" {
		log.Printf("Got password %s", password)
	}

	if err != nil {
		log.Fatalf("Port %s is not a digit", port)
	}

	settings := Settings{
		Host: 		host,
		Port: 		port,
		User: 		user,
		Password:	password,
		ProxyList: 	proxyList,
	}

	return &settings
}

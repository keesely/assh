package domain

import (
	"fmt"
	"strings"
)

type Auth struct {
	Password string `json:"password,omitempty"`
	KeyFile  string `json:"keyfile,omitempty"`
}

type Server struct {
	Name    string                 `json:"name"`
	Group   string                 `json:"group"`
	Host    string                 `json:"host"`
	Port    int                    `json:"port"`
	User    string                 `json:"user"`
	Auth    *Auth                  `json:"auth,omitempty"`
	Remark  string                 `json:"remark,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
	Version int                    `json:"version"`
}

func ParseName(name string) (group, serverName string) {
	if name == "" {
		return "", ""
	}

	parts := strings.SplitN(name, ".", 2)
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}

func JoinName(group, name string) string {
	if group == "" {
		return name
	}
	return fmt.Sprintf("%s.%s", group, name)
}

func ValidateServer(s *Server) error {
	if s.Name == "" {
		return ErrInvalidName
	}
	if s.Host == "" {
		return ErrEmptyField
	}
	if s.Port < 1 || s.Port > 65535 {
		return ErrInvalidPort
	}
	return nil
}

func NewServer(name, host string, port int, user string) *Server {
	if port == 0 {
		port = 22
	}
	if user == "" {
		user = "root"
	}

	s := &Server{
		Name:    name,
		Host:    host,
		Port:    port,
		User:    user,
		Options: make(map[string]interface{}),
	}

	group, _ := ParseName(name)
	s.Group = group

	return s
}
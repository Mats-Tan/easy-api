package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

type SessionManager interface {
	AddSession(key string, sess *yamux.Session)
	DialTarget(key string) (net.Conn, error)
}

func NewSessionManager() SessionManager {
	return &sessionManager{
		sessions: map[string][]*yamux.Session{},
	}
}

type sessionManager struct {
	sessions map[string][]*yamux.Session
	mutex    sync.Mutex
}

func (m *sessionManager) AddSession(key string, sess *yamux.Session) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	curr := m.sessions[key]
	if curr == nil {
		curr = []*yamux.Session{}
	}
	curr = append(curr, sess)
	m.sessions[key] = curr
}

func (m *sessionManager) DialTarget(key string) (net.Conn, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	ss := m.sessions[key]
	for {
		if len(ss) == 0 {
			return nil, fmt.Errorf("no session found in '%s'", key)
		}

		idx := rand.Intn(len(ss))
		conn, err := ss[idx].Open()
		if err != nil {
			log.Printf("removing session #%d of '%s' due to dial error: %s", idx, key, err)
			ss[idx].Close()
			ss = append(ss[:idx], ss[idx+1:]...)
			m.sessions[key] = ss
			continue
		}
		log.Printf("find session with key '%s'", key)
		return conn, nil
	}
}

func SetupHandlers(r *gin.RouterGroup) {
	m := NewSessionManager()

	r.GET("/hubs/:id", func(c *gin.Context) {
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("failed to upgrade to WebSocket: %s", err)})
			return
		}

		session, err := yamux.Server(conn.UnderlyingConn(), nil)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": fmt.Sprintf("failed to multiplex channel: %s", err)})
			return
		}

		m.AddSession(c.Param("id"), session)
	})

	r.Any("/proxy/:id/*proxyPath", func(c *gin.Context) {
		u, err := url.Parse("http://127.0.0.1")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
			return
		}

		rp := httputil.NewSingleHostReverseProxy(u)
		key := c.Param("id")
		rp.Transport = &http.Transport{
			DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
				conn, err := m.DialTarget(key)
				return conn, err
			},
		}
		rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			c.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
		}

		hostInQuery, exist := c.GetQuery("x-proxy-host")
		if exist == true {
			c.Request.Header.Add("X-Proxy-Host", hostInQuery)
		}
		c.Request.Header.Add("X-Proxy-Path", c.Param("proxyPath"))
		rp.ServeHTTP(c.Writer, c.Request)
	})
}

func setupRouter() (*gin.Engine, error) {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	SetupHandlers(v1)

	return r, nil
}

func main() {
	r,_ := setupRouter()
	r.Run(":8081")
}

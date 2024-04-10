// majority of code from: https://www.agwa.name/blog/post/writing_an_sni_proxy_in_go
// modifications were made for fault injection

package main

import (
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

var faultInjectionRate float64
var delayInjection int

func init() {
	faultInjectionRateStr := os.Getenv("FAULT_INJECTION_RATE")
	if faultInjectionRateStr == "" {
		faultInjectionRateStr = "0.1" // Default to 10% if not set
	}

	var err error
	faultInjectionRate, err = strconv.ParseFloat(faultInjectionRateStr, 64)
	if err != nil {
		log.Fatal("Invalid fault injection rate:", err)
	}

	// if FAULT_INJECTION_SLEEP is set, the server will sleep for that duration before responding rather than failing the request
	// if set to 0, the server will fail the request immediately
	delayInjectionStr := os.Getenv("FAULT_INJECTION_SLEEP")
	if delayInjectionStr == "" {
		delayInjectionStr = "0" //default to 0
	}

	delayInjection, err = strconv.Atoi(delayInjectionStr)
	if err != nil {
		log.Fatal("Invalid fault injection rate:", err)
	}
}

func main() {
	l, err := net.Listen("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	if err := clientConn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Print(err)
		return
	}

	clientHello, clientReader, err := peekClientHello(clientConn)
	if err != nil {
		log.Print(err)
		return
	}

	log.Printf("Received request for server: %s", clientHello.ServerName)

	if rand.Float64() < faultInjectionRate {
		if delayInjection > 0 {
			log.Printf("Fault injected: Sleeping for %d seconds", delayInjection)
			time.Sleep(time.Duration(delayInjection) * time.Second)
		} else {
			log.Print("Fault injected: Closing connection")
			return
		}
	} else {
		log.Print("No fault injected")
	}

	if err := clientConn.SetReadDeadline(time.Time{}); err != nil {
		log.Print(err)
		return
	}

	backendConn, err := net.DialTimeout("tcp", net.JoinHostPort(clientHello.ServerName, "443"), 5*time.Second)
	if err != nil {
		log.Print(err)
		return
	}
	defer backendConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		io.Copy(clientConn, backendConn)
		clientConn.(*net.TCPConn).CloseWrite()
		wg.Done()
	}()
	go func() {
		io.Copy(backendConn, clientReader)
		backendConn.(*net.TCPConn).CloseWrite()
		wg.Done()
	}()

	wg.Wait()
}

func peekClientHello(reader io.Reader) (*tls.ClientHelloInfo, io.Reader, error) {
	peekedBytes := new(bytes.Buffer)
	hello, err := readClientHello(io.TeeReader(reader, peekedBytes))
	if err != nil {
		return nil, nil, err
	}
	return hello, io.MultiReader(peekedBytes, reader), nil
}

type readOnlyConn struct {
	reader io.Reader
}

func (conn readOnlyConn) Read(p []byte) (int, error)         { return conn.reader.Read(p) }
func (conn readOnlyConn) Write(p []byte) (int, error)        { return 0, io.ErrClosedPipe }
func (conn readOnlyConn) Close() error                       { return nil }
func (conn readOnlyConn) LocalAddr() net.Addr                { return nil }
func (conn readOnlyConn) RemoteAddr() net.Addr               { return nil }
func (conn readOnlyConn) SetDeadline(t time.Time) error      { return nil }
func (conn readOnlyConn) SetReadDeadline(t time.Time) error  { return nil }
func (conn readOnlyConn) SetWriteDeadline(t time.Time) error { return nil }

func readClientHello(reader io.Reader) (*tls.ClientHelloInfo, error) {
	var hello *tls.ClientHelloInfo

	err := tls.Server(readOnlyConn{reader: reader}, &tls.Config{
		GetConfigForClient: func(argHello *tls.ClientHelloInfo) (*tls.Config, error) {
			hello = new(tls.ClientHelloInfo)
			*hello = *argHello
			return nil, nil
		},
	}).Handshake()

	if hello == nil {
		return nil, err
	}

	return hello, nil
}

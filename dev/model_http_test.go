package dev

import (
	//"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestHTTP1(t *testing.T) {

	// launch bogus test server
	addr := ":2001"
	s, listenErr := spawnServerHTTP(t, addr)
	if listenErr != nil {
		t.Errorf("could not spawn bogus HTTP server: %v", listenErr)
	}
	t.Logf("TestHTTP1: server running on %s", addr)

	// run client test
	logger := &testLogger{t}
	app := &bogusApp{
		models:  map[string]*Model{},
		devices: map[string]*Device{},
	}
	RegisterModels(logger, app.models)
	CreateDevice(app, logger, "junos", "lab1", "localhost"+addr, "telnet", "lab", "pass", "en")
	ScanDevices(app, logger, 3, 100*time.Millisecond, 200*time.Millisecond)

	s.close()

	<-s.done // wait termination of accept loop goroutine
}

func spawnServerHTTP(t *testing.T, addr string) (*testServer, error) {

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	s := &testServer{listener: ln, done: make(chan int)}

	go acceptLoopHTTP(t, s, handleConnectionHTTP)

	return s, nil
}

func acceptLoopHTTP(t *testing.T, s *testServer, handler func(*testing.T, net.Conn)) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			t.Logf("acceptLoopHTTP: accept failure, exiting: %v", err)
			break
		}
		go handler(t, conn)
	}

	close(s.done)
}

func handleConnectionHTTP(t *testing.T, c net.Conn) {
	defer c.Close()

	buf := make([]byte, 1000)

	// send username prompt
	if _, err := c.Write([]byte("hostname (ttyp0)\n\nlogin: ")); err != nil {
		t.Logf("handleConnectionHTTP: send username prompt error: %v", err)
		return
	}

	// consume username
	if _, err := c.Read(buf); err != nil {
		t.Logf("handleConnectionHTTP: read username error: %v", err)
		return
	}

	// send password prompt
	if _, err := c.Write([]byte("\nPassword: ")); err != nil {
		t.Logf("handleConnectionHTTP: send password prompt error: %v", err)
		return
	}

	// consume password
	if _, err := c.Read(buf); err != nil {
		t.Logf("handleConnectionHTTP: read password error: %v", err)
		return
	}

	if _, err := c.Write([]byte("\n--- JUNOS 11.2R1.2 built 2011-06-22 02:55:58 UTC")); err != nil {
		t.Logf("handleConnectionHTTP: send username prompt error: %v", err)
		return
	}

LOOP:
	for {
		// send command prompt
		if _, err := c.Write([]byte("\n{master:0}\nlab@host.domain> ")); err != nil {
			t.Logf("handleConnectionHTTP: send command prompt error: %v", err)
			return
		}

		// consume command
		if _, err := c.Read(buf); err != nil {
			if err == io.EOF {
				return // peer closed connection
			}
			t.Logf("handleConnectionHTTP: read command error: %v", err)
			return
		}

		str := string(buf)

		switch {
		case strings.HasPrefix(str, "q"): //quit
			break LOOP
		case strings.HasPrefix(str, "ex"): //exit
			break LOOP
		case strings.HasPrefix(str, "set cli"):
		case strings.HasPrefix(str, "show conf"):
			if _, err := c.Write([]byte("\nshow running-configuration")); err != nil {
				t.Logf("handleConnectionHTTP: send sh run error: %v", err)
				return
			}
		default:
			if _, err := c.Write([]byte("\nIgnoring unknown command")); err != nil {
				t.Logf("handleConnectionHTTP: send unknown command error: %v", err)
				return
			}
		}

	}

	// send bye
	if _, err := c.Write([]byte("\nbye\n")); err != nil {
		t.Logf("handleConnectionHTTP: send bye error: %v", err)
		return
	}

}

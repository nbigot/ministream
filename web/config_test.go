package web

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// A client may wait between two requests sent on a keep-alive connection: the server must not
// close the connection meanwhile (the idle timeout was 60 µs instead of 60 s).
func TestKeepAliveConnectionSurvivesAPause(t *testing.T) {
	app := fiber.New(GetFiberConfig())
	app.Get("/ping", func(c *fiber.Ctx) error { return c.SendString("pong") })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln) }()
	defer func() { _ = app.Shutdown() }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	for i := 1; i <= 2; i++ {
		if i > 1 {
			time.Sleep(200 * time.Millisecond)
		}
		if _, err := conn.Write([]byte("GET /ping HTTP/1.1\r\nHost: localhost\r\n\r\n")); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp, err := http.ReadResponse(reader, nil)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil || string(body) != "pong" {
			t.Fatalf("request %d: body %q, error %v", i, body, err)
		}
	}
}

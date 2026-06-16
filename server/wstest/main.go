//go:build ignore

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	token := os.Getenv("TOKEN")
	url := "ws://localhost:8090/ws?token=" + token
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		fmt.Println("dial error:", err)
		os.Exit(1)
	}
	defer c.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			fmt.Println("RECV:", string(msg))
		}
	}()

	time.Sleep(300 * time.Millisecond)
	send := `{"type":"chat_message","channel_id":1,"content":"hello from ws, hi @admin","temp_id":"t1"}`
	c.WriteMessage(websocket.TextMessage, []byte(send))

	time.Sleep(2 * time.Second)
}

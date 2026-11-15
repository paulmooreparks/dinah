package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestPromptDispatchPrecedesResponseAndSurvivesLoss(t *testing.T) {
	for _, lost := range []bool{false, true} {
		t.Run(map[bool]string{false: "response", true: "lost"}[lost], func(t *testing.T) {
			client, peer := net.Pipe()
			c := Connect(client, client, closePipeHandler{}, Options{})
			defer c.Close()
			defer peer.Close()
			observed := make(chan struct{})
			go func() {
				sc := bufio.NewScanner(peer)
				if !sc.Scan() {
					return
				}
				var request message
				if err := json.Unmarshal(sc.Bytes(), &request); err != nil {
					t.Error(err)
					return
				}
				<-observed
				if lost {
					peer.Close()
					return
				}
				if err := json.NewEncoder(peer).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": PromptResult{StopReason: "end_turn"}}); err != nil {
					t.Error(err)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var out PromptResult
			err := c.PromptWithDispatch(ctx, PromptParams{SessionID: "invented"}, &out, func() { close(observed) })
			if (err != nil) != lost || !lost && out.StopReason != "end_turn" {
				t.Fatalf("lost=%v result=%+v err=%v", lost, out, err)
			}
			select {
			case <-observed:
			default:
				t.Fatal("request was not observed")
			}
		})
	}
}

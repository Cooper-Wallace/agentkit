package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type wakeNotificationPayload struct {
	SessionID string `json:"session_id"`
	State     string `json:"state"`
	Agent     string `json:"agent"`
	Task      string `json:"task"`
	PID       int    `json:"pid"`
}

func defaultWakePort() int {
	for _, envKey := range []string{"AGENTKIT_WAKE_PORT", "WAKE_PORT"} {
		if raw := os.Getenv(envKey); raw != "" {
			if p, err := strconv.Atoi(raw); err == nil && p > 0 {
				return p
			}
		}
	}
	return 18820
}

func postWakeNotification(sessionID, state, agent, task string, port int) {
	body, err := json.Marshal(wakeNotificationPayload{
		SessionID: sessionID,
		State:     state,
		Agent:     agent,
		Task:      task,
		PID:       os.Getpid(),
	})
	if err != nil {
		return
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/v1/notify", port)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout:   500 * time.Millisecond,
		Transport: &http.Transport{Proxy: nil},
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

func isTTYReader(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok || f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// NotifyWake implements the wake-lock hook that pings the local WakeServer with active/idle status.
func NotifyWake(args []string, stdin io.Reader, stdout io.Writer) int {
	for _, a := range args {
		if a == "--dry-run" {
			writeJSONLine(stdout, map[string]string{
				"status":  "dry-run",
				"message": "agentkit hook notify-wake is operational",
			})
			return 0
		}
	}

	defaultAgent := os.Getenv("AGENTKIT_HARNESS")
	if defaultAgent == "" {
		defaultAgent = "agent"
	}

	var stateArg string
	var sessionArg string
	agentArg := defaultAgent
	var taskArg string
	port := defaultWakePort()
	testMode := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--test":
			testMode = true
		case arg == "--session" && i+1 < len(args):
			i++
			sessionArg = args[i]
		case strings.HasPrefix(arg, "--session="):
			sessionArg = strings.TrimPrefix(arg, "--session=")
		case arg == "--agent" && i+1 < len(args):
			i++
			agentArg = args[i]
		case strings.HasPrefix(arg, "--agent="):
			agentArg = strings.TrimPrefix(arg, "--agent=")
		case arg == "--task" && i+1 < len(args):
			i++
			taskArg = args[i]
		case strings.HasPrefix(arg, "--task="):
			taskArg = strings.TrimPrefix(arg, "--task=")
		case arg == "--port" && i+1 < len(args):
			i++
			if p, err := strconv.Atoi(args[i]); err == nil && p > 0 {
				port = p
			}
		case strings.HasPrefix(arg, "--port="):
			if p, err := strconv.Atoi(strings.TrimPrefix(arg, "--port=")); err == nil && p > 0 {
				port = p
			}
		case arg == "active" || arg == "idle":
			if stateArg == "" {
				stateArg = arg
			}
		}
	}

	payload := make(map[string]any)
	if !testMode && stdin != nil && !isTTYReader(stdin) {
		if raw, err := io.ReadAll(stdin); err == nil {
			trimmed := strings.TrimSpace(string(raw))
			if trimmed != "" {
				var loaded map[string]any
				if err := json.Unmarshal([]byte(trimmed), &loaded); err == nil && loaded != nil {
					payload = loaded
				}
			}
		}
	}

	_, hasToolCall := payload["toolCall"]
	_, hasTermination := payload["terminationReason"]

	state := stateArg
	if state == "" {
		if hasTermination {
			state = "idle"
		} else {
			state = "active"
		}
	}

	session := sessionArg
	if session == "" {
		if cid, ok := payload["conversationId"].(string); ok && cid != "" {
			session = cid
		} else if s := os.Getenv("AGY_SESSION_ID"); s != "" {
			session = s
		} else if s := os.Getenv("TERM_SESSION_ID"); s != "" {
			session = s
		} else if s := os.Getenv("ANTIGRAVITY_SESSION_ID"); s != "" {
			session = s
		} else {
			session = fmt.Sprintf("ppid-%d", os.Getppid())
		}
	}

	agent := agentArg
	if agent == "" || agent == "agent" {
		if pa, ok := payload["agent"].(string); ok && pa != "" {
			agent = pa
		} else if envAgent := os.Getenv("AGENTKIT_HARNESS"); envAgent != "" {
			agent = envAgent
		} else {
			agent = "agent"
		}
	}

	task := taskArg
	if task == "" {
		if tc, ok := payload["toolCall"].(map[string]any); ok {
			if name, ok := tc["name"].(string); ok {
				task = name
			}
		} else if t, ok := payload["task"]; ok && t != nil {
			task = fmt.Sprintf("%v", t)
		}
	}

	postWakeNotification(session, state, agent, task, port)

	if hasToolCall {
		writeJSONLine(stdout, map[string]string{"decision": "allow"})
	} else if hasTermination {
		writeJSONLine(stdout, map[string]string{"decision": "stop"})
	} else {
		writeJSONLine(stdout, map[string]any{})
	}
	return 0
}

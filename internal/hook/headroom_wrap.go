package hook

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	logCriticalRe = regexp.MustCompile(`(?i)(error|fail|failed|traceback|exception|assert|panic:|fatal|warning:)`)

	diffHeaderPrefixes = []string{
		"diff --git ",
		"index ",
		"--- ",
		"+++ ",
		"@@ ",
		"new file mode",
		"deleted file mode",
		"rename ",
	}
)

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

func compressionThreshold() int {
	if raw := os.Getenv("AGY_COMPRESSION_THRESHOLD"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			return v
		}
	}
	return 1500
}

func marshalIndentedJSON(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func crushJSON(rawOutput string) (string, bool) {
	stripped := strings.TrimLeft(rawOutput, " \t\r\n")
	if !strings.HasPrefix(stripped, "{") && !strings.HasPrefix(stripped, "[") {
		return "", false
	}

	var data any
	dec := json.NewDecoder(strings.NewReader(stripped))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return "", false
	}

	modified := false
	switch typed := data.(type) {
	case []any:
		if len(typed) > 6 {
			omitted := len(typed) - 5
			crushed := make([]any, 0, 6)
			crushed = append(crushed, typed[:3]...)
			crushed = append(crushed, map[string]any{
				"_headroom_omitted": fmt.Sprintf("... %d items omitted ...", omitted),
			})
			crushed = append(crushed, typed[len(typed)-2:]...)
			data = crushed
			modified = true
		}
	case map[string]any:
		for k, v := range typed {
			if arr, ok := v.([]any); ok && len(arr) > 6 {
				omitted := len(arr) - 5
				crushed := make([]any, 0, 6)
				crushed = append(crushed, arr[:3]...)
				crushed = append(crushed, map[string]any{
					"_headroom_omitted": fmt.Sprintf("... %d items omitted ...", omitted),
				})
				crushed = append(crushed, arr[len(arr)-2:]...)
				typed[k] = crushed
				modified = true
			}
		}
	}

	if !modified {
		return "", false
	}

	compacted, err := marshalIndentedJSON(data)
	if err != nil || len(compacted) >= len(rawOutput) {
		return "", false
	}
	return compacted, true
}

func isDiffHeader(line string) bool {
	for _, prefix := range diffHeaderPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func flushDiffHunk(body []string) []string {
	if len(body) > 16 {
		omitted := len(body) - 16
		out := make([]string, 0, 17)
		out = append(out, body[:12]...)
		out = append(out, fmt.Sprintf("... [%d hunk lines compressed by Headroom CCR] ...", omitted))
		out = append(out, body[len(body)-4:]...)
		return out
	}
	return body
}

func crushDiff(rawOutput string) (string, bool) {
	if !strings.Contains(rawOutput, "diff --git ") {
		return "", false
	}
	diffLines := splitLines(rawOutput)
	if len(diffLines) <= 60 {
		return "", false
	}

	compressed := make([]string, 0, len(diffLines))
	var currentHunkBody []string

	for _, line := range diffLines {
		if isDiffHeader(line) {
			if len(currentHunkBody) > 0 {
				compressed = append(compressed, flushDiffHunk(currentHunkBody)...)
				currentHunkBody = nil
			}
			compressed = append(compressed, line)
		} else {
			currentHunkBody = append(currentHunkBody, line)
		}
	}
	if len(currentHunkBody) > 0 {
		compressed = append(compressed, flushDiffHunk(currentHunkBody)...)
	}

	result := strings.Join(compressed, "\n")
	if len(result) < len(rawOutput) {
		return result, true
	}
	return "", false
}

func compressPayload(rawOutput string, threshold int) string {
	// 1. JSON SmartCrusher
	if compacted, ok := crushJSON(rawOutput); ok {
		return compacted
	}

	// 2. Git Diff Structural Compressor (DiffCrusher)
	if compactedDiff, ok := crushDiff(rawOutput); ok {
		return compactedDiff
	}

	// 3. Error-Preserving Log & Output Crusher
	lines := splitLines(rawOutput)
	if len(lines) > 60 {
		head := lines[:25]
		tail := lines[len(lines)-25:]
		middle := lines[25 : len(lines)-25]

		selectedSet := make(map[int]struct{})
		for idx, line := range middle {
			if logCriticalRe.MatchString(line) {
				start := idx - 2
				if start < 0 {
					start = 0
				}
				end := idx + 3
				if end > len(middle) {
					end = len(middle)
				}
				for ctxIdx := start; ctxIdx < end; ctxIdx++ {
					selectedSet[ctxIdx] = struct{}{}
					if len(selectedSet) >= 40 {
						break
					}
				}
			}
			if len(selectedSet) >= 40 {
				break
			}
		}

		sortedIndices := make([]int, 0, len(selectedSet))
		for idx := range selectedSet {
			sortedIndices = append(sortedIndices, idx)
		}
		sort.Ints(sortedIndices)

		omittedCount := len(middle) - len(sortedIndices)
		divider := fmt.Sprintf("\n... [%d routine lines compressed by Headroom CCR] ...\n", omittedCount)
		if len(sortedIndices) > 0 {
			middleLines := make([]string, 0, len(sortedIndices))
			for _, idx := range sortedIndices {
				middleLines = append(middleLines, middle[idx])
			}
			return strings.Join(head, "\n") + divider + strings.Join(middleLines, "\n") + "\n" + strings.Join(tail, "\n")
		}
		return strings.Join(head, "\n") + divider + strings.Join(tail, "\n")
	}

	if len(rawOutput) >= threshold && len(rawOutput) > 1200 {
		firstPart := rawOutput[:700]
		lastPart := rawOutput[len(rawOutput)-500:]
		omittedChars := len(rawOutput) - 1200
		divider := fmt.Sprintf("\n... [%d chars compressed by Headroom CCR] ...\n", omittedChars)
		return firstPart + divider + lastPart
	}

	return rawOutput
}

func cacheFullOutput(rawOutput string) string {
	sum := sha256.Sum256([]byte(rawOutput))
	contentHash := hex.EncodeToString(sum[:])[:12]

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return ""
	}
	cacheDir := filepath.Join(homeDir, ".cache", "antigravity-headroom")
	cacheFile := filepath.Join(cacheDir, contentHash+".txt")

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return cacheFile
	}
	_ = os.WriteFile(cacheFile, []byte(rawOutput), 0644)

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return cacheFile
	}

	type cachedEntry struct {
		path    string
		modTime time.Time
	}
	var txtFiles []cachedEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		txtFiles = append(txtFiles, cachedEntry{
			path:    filepath.Join(cacheDir, entry.Name()),
			modTime: info.ModTime(),
		})
	}

	if len(txtFiles) > 100 {
		sort.Slice(txtFiles, func(i, j int) bool {
			return txtFiles[i].modTime.Before(txtFiles[j].modTime)
		})
		for _, old := range txtFiles[:len(txtFiles)-100] {
			_ = os.Remove(old.path)
		}
	}

	return cacheFile
}

func runWrappedCommand(cmdArgs []string) (string, int, error) {
	var out bytes.Buffer
	var cmd *exec.Cmd

	if len(cmdArgs) == 1 {
		cmd = exec.Command("sh", "-c", cmdArgs[0])
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		return out.String(), extractExitCode(err), unwrapExecFailure(err)
	}

	cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil && errors.Is(err, exec.ErrNotFound) {
		out.Reset()
		cmd = exec.Command("sh", "-c", strings.Join(cmdArgs, " "))
		cmd.Stdout = &out
		cmd.Stderr = &out
		err = cmd.Run()
	}
	return out.String(), extractExitCode(err), unwrapExecFailure(err)
}

func extractExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func unwrapExecFailure(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil
	}
	return err
}

// HeadroomWrap executes a wrapped command, caches full output for Reversible CCR, and compresses verbose output.
func HeadroomWrap(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--dry-run" {
		writeJSONLine(stdout, map[string]string{
			"status":  "dry-run",
			"message": "agentkit hook headroom-wrap is operational",
		})
		return 0
	}

	cmdArgs := args
	if len(cmdArgs) > 0 && cmdArgs[0] == "--" {
		cmdArgs = cmdArgs[1:]
	}
	if len(cmdArgs) == 0 {
		return 0
	}

	rawOutput, exitCode, execErr := runWrappedCommand(cmdArgs)
	if execErr != nil {
		if stderr != nil {
			_, _ = fmt.Fprintf(stderr, "headroom-wrap execution error: %v\n", execErr)
		}
		return 1
	}

	threshold := compressionThreshold()
	if len(rawOutput) < threshold || os.Getenv("AGY_DISABLE_TOKEN_COMPRESSION") == "1" {
		if stdout != nil {
			_, _ = io.WriteString(stdout, rawOutput)
		}
		return exitCode
	}

	cacheFile := cacheFullOutput(rawOutput)
	compressed := compressPayload(rawOutput, threshold)

	origLen := len(rawOutput)
	compLen := len(compressed)
	savedPct := 0.0
	if origLen > 0 {
		savedPct = math.Round((1.0-(float64(compLen)/float64(origLen)))*1000.0) / 10.0
	}

	breadcrumb := fmt.Sprintf(
		"\n[Headroom CCR: compressed %d -> %d chars (%.1f%% saved). Full output cached at %s]\n",
		origLen, compLen, savedPct, cacheFile,
	)

	if stdout != nil {
		_, _ = io.WriteString(stdout, strings.TrimRight(compressed, "\n")+breadcrumb)
	}
	return exitCode
}

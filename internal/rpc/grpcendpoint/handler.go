// Package grpcendpoint handles gRPC-style endpoints for NotebookLM
package grpcendpoint

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/tmc/nlm/internal/rpc"
)

// Client handles gRPC-style endpoint requests
type Client struct {
	authToken  string
	cookies    string
	httpClient *http.Client
	debug      bool
}

// NewClient creates a new gRPC endpoint client
func NewClient(authToken, cookies string) *Client {
	return &Client{
		authToken:  authToken,
		cookies:    cookies,
		httpClient: &http.Client{},
	}
}

// Request represents a gRPC-style request
type Request struct {
	Endpoint string      // e.g., "/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed"
	Body     interface{} // The request body (will be JSON encoded)
}

// Execute sends a gRPC-style request to NotebookLM
func (c *Client) Execute(req Request) ([]byte, error) {
	baseURL := "https://notebooklm.google.com/_/LabsTailwindUi/data"

	// Build the full URL with the endpoint
	fullURL := baseURL + req.Endpoint

	// Get API parameters dynamically
	apiParams := rpc.GetAPIParams(c.cookies)

	// Add query parameters
	params := url.Values{}
	params.Set("bl", apiParams.BuildVersion)
	params.Set("f.sid", apiParams.SessionID)
	params.Set("hl", "en")
	params.Set("_reqid", fmt.Sprintf("%d", generateRequestID()))
	params.Set("rt", "c")

	fullURL = fullURL + "?" + params.Encode()

	// Encode the request body
	bodyJSON, err := json.Marshal(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request body: %w", err)
	}

	// Create form data
	formData := url.Values{}
	formData.Set("f.req", string(bodyJSON))
	formData.Set("at", c.authToken)

	// Create the HTTP request
	httpReq, err := http.NewRequest("POST", fullURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	httpReq.Header.Set("Cookie", c.cookies)
	httpReq.Header.Set("Origin", "https://notebooklm.google.com")
	httpReq.Header.Set("Referer", "https://notebooklm.google.com/")
	httpReq.Header.Set("X-Same-Domain", "1")
	httpReq.Header.Set("Accept", "*/*")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

	if c.debug {
		fmt.Printf("=== gRPC Endpoint Request ===\n")
		fmt.Printf("URL: %s\n", fullURL)
		fmt.Printf("f.req (raw JSON): %s\n", string(bodyJSON))
		fmt.Printf("Body (URL-encoded): %s\n", formData.Encode())
	}

	// Send the request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	bodyStr := string(body)
	// Strip the )]}' prefix that Google adds to prevent JSON hijacking
	if strings.HasPrefix(bodyStr, ")]}'") {
		bodyStr = strings.TrimPrefix(bodyStr, ")]}'")
		bodyStr = strings.TrimLeft(bodyStr, "\n")
	}

	// Response is in chunked format: <length>\n<json>\n<length>\n<json>...
	// We want to aggregate ALL text content chunks.
	var resultText string
	lines := strings.Split(bodyStr, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || isNumeric(line) {
			continue
		}

		var currentOuter interface{}
		if err := json.Unmarshal([]byte(line), &currentOuter); err != nil {
			continue
		}

		// Use recursive extraction to find all text content in this chunk
		var chunkStrings []string
		extractBodyText(currentOuter, &chunkStrings)

		// Join fragments in this chunk and append to overall fragments
		if len(chunkStrings) > 0 {
			chunkText := strings.Join(chunkStrings, "")
			// Some Google APIs send the full accumulated text in each chunk.
			// Others send incremental diffs.
			// For chat, it often sends the full text so far.
			// We'll keep the longest version we've seen if it seems to contain previous ones.
			if len(chunkText) > len(resultText) {
				resultText = chunkText
			}
		}
	}

	if resultText == "" {
		// FALLBACK: Try to find any string in a nested array (emergency extraction)
		lines := strings.Split(bodyStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, "\"") {
				// Use a regex to find the longest quoted string as a last resort
				re := regexp.MustCompile(`"([^"\\]*(?:\\.[^"\\]*)*)"`)
				matches := re.FindAllStringSubmatch(line, -1)
				for _, m := range matches {
					if len(m[1]) > len(resultText) {
						resultText = m[1]
					}
				}
			}
		}
	}

	if resultText == "" {
		return nil, fmt.Errorf("failed to extract content from chunked response")
	}

	return []byte(resultText), nil
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// StreamResponse handles streaming responses from gRPC endpoints
func (c *Client) Stream(req Request, handler func(chunk []byte) error) error {
	baseURL := "https://notebooklm.google.com/_/LabsTailwindUi/data"
	fullURL := baseURL + req.Endpoint

	// Get API parameters dynamically
	apiParams := rpc.GetAPIParams(c.cookies)

	// Add query parameters
	params := url.Values{}
	params.Set("bl", apiParams.BuildVersion)
	params.Set("f.sid", apiParams.SessionID)
	params.Set("hl", "en")
	params.Set("_reqid", fmt.Sprintf("%d", generateRequestID()))
	params.Set("rt", "c")

	fullURL = fullURL + "?" + params.Encode()

	// Encode the request body
	bodyJSON, err := json.Marshal(req.Body)
	if err != nil {
		return fmt.Errorf("failed to encode request body: %w", err)
	}

	// Create form data
	formData := url.Values{}
	formData.Set("f.req", string(bodyJSON))
	formData.Set("at", c.authToken)

	// Create the HTTP request
	httpReq, err := http.NewRequest("POST", fullURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	httpReq.Header.Set("Cookie", c.cookies)
	httpReq.Header.Set("Origin", "https://notebooklm.google.com")
	httpReq.Header.Set("Referer", "https://notebooklm.google.com/")
	httpReq.Header.Set("X-Same-Domain", "1")

	// Send the request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read the streaming response
	reader := bufio.NewReader(resp.Body)
	for {
		// Read length line
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read length: %w", err)
		}

		lengthStr := strings.TrimSpace(line)
		if lengthStr == "" {
			continue
		}

		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			// If not a number, it might be the prefix or other garbage
			continue
		}

		// Read exactly 'length' bytes
		chunk := make([]byte, length)
		_, err = io.ReadFull(reader, chunk)
		if err != nil {
			return fmt.Errorf("read chunk: %w", err)
		}

		// Process the chunk
		if err := handler(chunk); err != nil {
			return fmt.Errorf("handler error: %w", err)
		}
	}

	return nil
}

// Helper to generate request IDs
var requestCounter int

func generateRequestID() int {
	requestCounter++
	return 1000000 + requestCounter
}

// BuildChatRequest builds a request for the GenerateFreeFormStreamed endpoint
// Browser format: [null,"[[[[\"source_id\"]]],\"prompt\",null,[2,null,[1]]]"]
func BuildChatRequest(sourceIDs []string, prompt string) interface{} {
	// Build the nested source IDs array with 3 levels of wrapping
	// innerArray adds 1 level, so we need 3 wraps to get 4 levels total
	// Format: [[[source_id1, source_id2, ...]]]
	var sourceIDsInner []interface{}
	for _, id := range sourceIDs {
		sourceIDsInner = append(sourceIDsInner, id)
	}
	// 3 wraps: [[[ids]]] -> becomes [[[[ids]]]] in innerArray
	sourceIDsNested := []interface{}{[]interface{}{sourceIDsInner}}

	// Build the inner array: [[[[sources]]], prompt, null, [2,null,[1]]]
	innerArray := []interface{}{
		sourceIDsNested,
		prompt,
		nil,
		[]interface{}{2, nil, []interface{}{1}},
	}

	// Marshal the inner array to JSON string
	innerJSON, _ := json.Marshal(innerArray)

	// Final format: [null, "inner_json_string"]
	return []interface{}{
		nil,
		string(innerJSON),
	}
}

func extractBodyText(v interface{}, results *[]string) {
	switch val := v.(type) {
	case string:
		if isMetadata(val) {
			return
		}
		// Clean up the string (Google sometimes uses \n\n as a separator)
		text := strings.TrimSpace(val)
		if text != "" {
			*results = append(*results, val)
		}
	case []interface{}:
		// Special handling for the gRPC-style content chunks to maintain order and structure
		// If it's a very deep nested array containing just one string, it's likely a content fragment
		if len(val) == 1 {
			if str, ok := val[0].(string); ok {
				if !isMetadata(str) {
					*results = append(*results, str)
					return
				}
			}
		}

		for _, item := range val {
			extractBodyText(item, results)
		}
	case map[string]interface{}:
		for _, item := range val {
			extractBodyText(item, results)
		}
	}
}

var (
	uuidRegex     = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
	metadataTerms = map[string]bool{
		"wrb.fr":                           true,
		"generic":                          true,
		"di":                               true,
		"af.httprm":                        true,
		"LabsTailwindOrchestrationService": true,
		"GenerateFreeFormStreamed":         true,
		"Considering Initial Approach":     true,
		"Summarizing the Stroop Test":      true,
		"Pinpointing a Fact":               true,
		"Outlining the Stroop Test":        true,
	}
)

func isMetadata(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if metadataTerms[s] {
		return true
	}
	if uuidRegex.MatchString(s) {
		return true
	}
	// Skip things that look like URL parts or internal identifiers
	if strings.Contains(s, "source_id") || strings.Contains(s, "project_id") {
		return true
	}
	// Skip short numeric or index strings
	if len(s) < 4 && isNumeric(s) {
		return true
	}
	return false
}

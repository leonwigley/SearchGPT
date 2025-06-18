package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("\033[1;34mWelcome to the SearchGPT application!\033[0m")
	fmt.Println("\033[1;32mThis app allows you to perform intelligent searches powered by xAI.\033[0m")
	fmt.Println("\033[1;33mPlease refer to the README for setup and usage instructions.\033[0m")

	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Get PORT and X_AI
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	apiKey := os.Getenv("X_AI")
	if apiKey == "" {
		log.Fatalf("X_AI not set in .env")
	}

	// Initialize Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Load HTML templates
	r.LoadHTMLGlob("*.html")

	// Serve the main page
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Handle search endpoint for HTMX
	r.POST("/search", func(c *gin.Context) {
		// Get the search query
		query := c.PostForm("query")
		if query == "" {
			c.String(http.StatusOK, "<li>Please enter a search query</li>")
			return
		}

		// Prepare xAI API request
		requestBody, err := json.Marshal(map[string]interface{}{
			"model": "grok-3-mini",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": `Return exactly 10 search results for the query as a JSON array, each with fields "title" (string), "description" (string), and "link" (string). Example: [{"title": "Example", "description": "A sample page", "link": "https://example.com"}, ...]. Do not wrap in markdown or extra text.`,
				},
				{"role": "user", "content": query},
			},
		})
		if err != nil {
			c.String(http.StatusOK, "<li>Error preparing API request</li>")
			return
		}

		// Create and send xAI API request
		req, err := http.NewRequest("POST", "https://api.x.ai/v1/chat/completions", bytes.NewBuffer(requestBody))
		if err != nil {
			c.String(http.StatusOK, "<li>Error creating API request</li>")
			return
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.String(http.StatusOK, "<li>Failed to contact xAI API</li>")
			return
		}
		defer resp.Body.Close()

		// Read response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.String(http.StatusOK, "<li>Error reading API response</li>")
			return
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("xAI API error: %s", string(body))
			c.String(http.StatusOK, "<li>xAI API error</li>")
			return
		}

		// Parse response
		var apiResponse struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			c.String(http.StatusOK, "<li>Error parsing API response</li>")
			return
		}

		if len(apiResponse.Choices) == 0 {
			c.String(http.StatusOK, "<li>No response from xAI API</li>")
			return
		}

		// Clean the content (remove markdown, whitespace)
		content := strings.TrimSpace(apiResponse.Choices[0].Message.Content)
		content = strings.TrimPrefix(content, "```json\n")
		content = strings.TrimSuffix(content, "\n```")
		content = strings.TrimSpace(content)

		// Log for debugging
		log.Printf("Cleaned xAI content: %s", content)

		// Parse the content as JSON array
		var results []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Link        string `json:"link"`
		}
		if err := json.Unmarshal([]byte(content), &results); err != nil {
			log.Printf("Error parsing result: %v, content: %s", err, content)
			c.String(http.StatusOK, "<li>Error parsing result: %s</li>", content)
			return
		}

		if len(results) == 0 {
			c.String(http.StatusOK, "<li>No results found</li>")
			return
		}

		// Build HTML fragment for all results
		var htmlOutput strings.Builder
		for _, result := range results {
			htmlOutput.WriteString(
				`<li><h3>` + html.EscapeString(result.Title) +
					`</h3><p>` + html.EscapeString(result.Description) +
					`</p><a href="` + html.EscapeString(result.Link) + `">` +
					html.EscapeString(result.Link) + `</a></li>`,
			)
		}

		// Return formatted HTML fragment
		c.String(http.StatusOK, htmlOutput.String())
	})

	// Start the server
	log.Printf("Server running on :%s", port)
	r.Run(":" + port)
}

// Serwer MCP dla CoursesList: pozwala agentowi AI (Claude Code, Claude Desktop) obsługiwać
// system przez REST API. Działa lokalnie przez stdio; klient uruchamia go jako proces.
package main

import (
	"context"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	siteURL := os.Getenv("COURSESLIST_API_URL")
	apiKey := os.Getenv("COURSESLIST_API_KEY")
	if siteURL == "" || apiKey == "" {
		log.Fatal("ustaw COURSESLIST_API_URL (np. https://kursy.example.pl) i COURSESLIST_API_KEY (clk_...)")
	}

	// stdout należy do protokołu MCP, więc logi idą na stderr (domyślne wyjście pakietu log).
	if err := newServer(newAPIClient(siteURL, apiKey)).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
